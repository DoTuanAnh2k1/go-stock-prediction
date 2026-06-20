// Package handlers defines the cli-svc handler catalog: the source-of-truth set
// of commands the CLI knows how to execute and render. The catalog is also the
// payload upserted to auth-svc on boot.
package handlers

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"go-stock-prediction/cli-svc/internal/client"
	"go-stock-prediction/cli-svc/internal/render"
)

// ArgSpec describes one argument of a handler.
type ArgSpec struct {
	Name     string   `json:"name"`
	Required bool     `json:"required"`
	Type     string   `json:"type"`              // string | int
	Choices  []string `json:"choices,omitempty"` // allowed values (e.g. markets)
}

// Handler is one catalog entry: metadata + execute + render.
type Handler struct {
	Key         string    `json:"handler_key"`
	DisplayName string    `json:"display_name"`
	Verb        string    `json:"verb"`     // get | set | update | delete
	Resource    string    `json:"resource"` // e.g. market.latest
	ArgSchema   []ArgSpec `json:"arg_schema"`

	// execute calls the API and returns the raw decoded response.
	execute func(ctx context.Context, c client.HTTPClient, jwt string, args map[string]string) (any, int, error)
	// render turns a raw response into a table string.
	render func(raw any) string
}

// Markets are the valid market keys.
var Markets = []string{"gold", "nasdaq", "crypto", "sp500"}

// Execute runs the handler's API call.
func (h *Handler) Execute(ctx context.Context, c client.HTTPClient, jwt string, args map[string]string) (any, int, error) {
	return h.execute(ctx, c, jwt, args)
}

// Render renders a response into a table string.
func (h *Handler) Render(raw any) string {
	return h.render(raw)
}

// Validate checks args against the handler's arg schema, returning a cleaned
// arg map (only known args) or an error.
func (h *Handler) Validate(args map[string]string) (map[string]string, error) {
	clean := map[string]string{}
	known := map[string]ArgSpec{}
	for _, spec := range h.ArgSchema {
		known[spec.Name] = spec
	}
	// Unknown args are an error.
	for name := range args {
		if _, ok := known[name]; !ok {
			return nil, fmt.Errorf("unknown argument %q for %s", name, h.Resource)
		}
	}
	for _, spec := range h.ArgSchema {
		val, present := args[spec.Name]
		if !present || val == "" {
			if spec.Required {
				return nil, fmt.Errorf("missing required argument %q", spec.Name)
			}
			continue
		}
		if spec.Type == "int" {
			for _, r := range val {
				if r < '0' || r > '9' {
					return nil, fmt.Errorf("argument %q must be an integer", spec.Name)
				}
			}
		}
		if len(spec.Choices) > 0 {
			ok := false
			for _, c := range spec.Choices {
				if c == val {
					ok = true
					break
				}
			}
			if !ok {
				return nil, fmt.Errorf("argument %q must be one of %s", spec.Name, strings.Join(spec.Choices, ", "))
			}
		}
		clean[spec.Name] = val
	}
	return clean, nil
}

// Registry holds all handlers keyed by handler_key.
type Registry struct {
	byKey      map[string]*Handler
	byVerbRes  map[string]*Handler // "verb resource" -> handler
}

// NewRegistry builds the default catalog.
func NewRegistry() *Registry {
	r := &Registry{byKey: map[string]*Handler{}, byVerbRes: map[string]*Handler{}}
	for _, h := range defaultHandlers() {
		hh := h
		r.byKey[hh.Key] = &hh
		r.byVerbRes[hh.Verb+" "+hh.Resource] = &hh
	}
	return r
}

// ByKey returns the handler for a handler_key.
func (r *Registry) ByKey(key string) (*Handler, bool) {
	h, ok := r.byKey[key]
	return h, ok
}

// Resolve maps a (verb, resource) pair to a handler.
func (r *Registry) Resolve(verb, resource string) (*Handler, bool) {
	h, ok := r.byVerbRes[verb+" "+resource]
	return h, ok
}

// All returns every handler sorted by key (stable order for upsert + tests).
func (r *Registry) All() []*Handler {
	out := make([]*Handler, 0, len(r.byKey))
	for _, h := range r.byKey {
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// Verbs returns the sorted set of distinct verbs in the catalog.
func (r *Registry) Verbs() []string {
	set := map[string]bool{}
	for _, h := range r.byKey {
		set[h.Verb] = true
	}
	return sortedKeys(set)
}

// ResourcesForVerb returns sorted resources registered under a verb.
func (r *Registry) ResourcesForVerb(verb string) []string {
	set := map[string]bool{}
	for _, h := range r.byKey {
		if h.Verb == verb {
			set[h.Resource] = true
		}
	}
	return sortedKeys(set)
}

// CatalogPayload is the JSON shape upserted to auth-svc on boot.
type CatalogPayload struct {
	Secret   string         `json:"secret"`
	Handlers []HandlerMeta  `json:"handlers"`
}

// HandlerMeta is the proto-agnostic handler description for the upsert payload.
type HandlerMeta struct {
	HandlerKey  string    `json:"handler_key"`
	DisplayName string    `json:"display_name"`
	Verb        string    `json:"verb"`
	Resource    string    `json:"resource"`
	ArgSchema   []ArgSpec `json:"arg_schema"`
	Enabled     bool      `json:"enabled"`
}

// UpsertPayload returns the catalog as the upsert payload with the given secret.
// Every handler in the in-code catalog is enabled by definition.
func (r *Registry) UpsertPayload(secret string) CatalogPayload {
	metas := make([]HandlerMeta, 0, len(r.byKey))
	for _, h := range r.All() {
		metas = append(metas, HandlerMeta{
			HandlerKey:  h.Key,
			DisplayName: h.DisplayName,
			Verb:        h.Verb,
			Resource:    h.Resource,
			ArgSchema:   h.ArgSchema,
			Enabled:     true,
		})
	}
	return CatalogPayload{Secret: secret, Handlers: metas}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// --- helpers shared by handler execute funcs ---

func getJSON(ctx context.Context, c client.HTTPClient, jwt, path string, q url.Values) (any, int, error) {
	return c.Do(ctx, jwt, "GET", path, q, nil)
}

func postJSON(ctx context.Context, c client.HTTPClient, jwt, path string, body any) (any, int, error) {
	return c.Do(ctx, jwt, "POST", path, nil, body)
}

func putJSON(ctx context.Context, c client.HTTPClient, jwt, path string, body any) (any, int, error) {
	return c.Do(ctx, jwt, "PUT", path, nil, body)
}

func deleteJSON(ctx context.Context, c client.HTTPClient, jwt, path string) (any, int, error) {
	return c.Do(ctx, jwt, "DELETE", path, nil, nil)
}

func autoRender(title string) func(any) string {
	return func(raw any) string { return render.Auto(title, raw) }
}
