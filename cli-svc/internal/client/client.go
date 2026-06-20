// Package client provides an HTTP client to the api-svc (via gateway).
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPClient is the interface the shell/handlers depend on. It is small on
// purpose so tests can mock it without any real network.
type HTTPClient interface {
	// Login authenticates against POST /auth/login and returns the auth result.
	Login(ctx context.Context, username, password string) (*LoginResult, error)
	// GetMyCommands fetches GET /me/commands using the supplied JWT.
	GetMyCommands(ctx context.Context, jwt string) ([]AllowedCommand, error)
	// Do performs a generic request against the API, attaching the JWT as a
	// Bearer token. path must start with "/". query may be nil. body may be nil.
	// It returns the decoded JSON (arbitrary shape) and the HTTP status code.
	Do(ctx context.Context, jwt, method, path string, query url.Values, body any) (any, int, error)
	// PostInternal performs an unauthenticated POST used for the handler-catalog
	// upsert on boot (protected by a shared secret in the body).
	PostInternal(ctx context.Context, path string, body any) (int, error)
}

// LoginResult is the subset of the login response cli-svc needs.
type LoginResult struct {
	Token  string `json:"token"`
	Role   string `json:"role"`
	UserID int64  `json:"user_id"`
}

// AllowedCommand mirrors one entry of GET /me/commands.
type AllowedCommand struct {
	ID         int64          `json:"id"`
	Name       string         `json:"name"`
	HandlerKey string         `json:"handler_key"`
	Args       map[string]any `json:"args"`
}

// Client is the concrete HTTPClient backed by net/http.
type Client struct {
	baseURL string
	hc      *http.Client
}

// New returns a Client targeting baseURL (e.g. http://gateway-svc/api).
func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		hc:      &http.Client{Timeout: 30 * time.Second},
	}
}

var _ HTTPClient = (*Client)(nil)

func (c *Client) Login(ctx context.Context, username, password string) (*LoginResult, error) {
	payload := map[string]string{"username": username, "password": password}
	raw, status, err := c.do(ctx, "", http.MethodPost, "/auth/login", nil, payload)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("login failed: status %d", status)
	}
	// Response is wrapped: {success, data:{token, user:{...}}} OR flat {token,...}.
	obj := unwrapObject(raw)
	res := &LoginResult{}
	if v, ok := obj["token"].(string); ok {
		res.Token = v
	}
	if res.Token == "" {
		return nil, fmt.Errorf("login response missing token")
	}
	// role + user_id may be nested under "user" or top-level claims; parse the JWT
	// for the authoritative values.
	if u, ok := obj["user"].(map[string]any); ok {
		if r, ok := u["role"].(string); ok {
			res.Role = r
		}
	}
	role, uid := decodeJWTClaims(res.Token)
	if role != "" {
		res.Role = role
	}
	if uid != 0 {
		res.UserID = uid
	}
	return res, nil
}

func (c *Client) GetMyCommands(ctx context.Context, jwt string) ([]AllowedCommand, error) {
	raw, status, err := c.do(ctx, jwt, http.MethodGet, "/me/commands", nil, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("me/commands: status %d", status)
	}
	// Accept either {data:[...]} or a bare [...] array.
	arr := unwrapArray(raw)
	out := make([]AllowedCommand, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		cmd := AllowedCommand{Args: map[string]any{}}
		if v, ok := m["id"].(float64); ok {
			cmd.ID = int64(v)
		}
		if v, ok := m["name"].(string); ok {
			cmd.Name = v
		}
		if v, ok := m["handler_key"].(string); ok {
			cmd.HandlerKey = v
		}
		if v, ok := m["args"].(map[string]any); ok {
			cmd.Args = v
		}
		out = append(out, cmd)
	}
	return out, nil
}

func (c *Client) Do(ctx context.Context, jwt, method, path string, query url.Values, body any) (any, int, error) {
	return c.do(ctx, jwt, method, path, query, body)
}

func (c *Client) PostInternal(ctx context.Context, path string, body any) (int, error) {
	_, status, err := c.do(ctx, "", http.MethodPost, path, nil, body)
	return status, err
}

func (c *Client) do(ctx context.Context, jwt, method, path string, query url.Values, body any) (any, int, error) {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if jwt != "" {
		req.Header.Set("Authorization", "Bearer "+jwt)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, resp.StatusCode, nil
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		// Non-JSON body — return it as a string payload.
		return string(data), resp.StatusCode, nil
	}
	return out, resp.StatusCode, nil
}

// unwrapObject returns the meaningful object from a raw decoded response,
// unwrapping a {success,data} envelope when present.
func unwrapObject(raw any) map[string]any {
	m, ok := raw.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	if d, ok := m["data"].(map[string]any); ok {
		return d
	}
	return m
}

// unwrapArray returns the array payload, unwrapping {data:[...]} when present.
func unwrapArray(raw any) []any {
	switch v := raw.(type) {
	case []any:
		return v
	case map[string]any:
		if d, ok := v["data"].([]any); ok {
			return d
		}
	}
	return nil
}
