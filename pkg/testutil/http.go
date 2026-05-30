package testutil

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-stock-prediction/pkg/server"
)

// NewTestServer creates an httptest.Server using the app's real router.
// The caller should defer ts.Close().
func NewTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := server.SetupAllRoutes()
	return httptest.NewServer(mux)
}

// TestResponse holds a parsed HTTP response for test assertions.
type TestResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// MakeRequest sends an HTTP request to the given URL and returns the parsed response.
func MakeRequest(t *testing.T, method, url string, body io.Reader) *TestResponse {
	t.Helper()

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	return &TestResponse{
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
		Body:       respBody,
	}
}

// ParseJSON unmarshals the response body into the target.
func (r *TestResponse) ParseJSON(t *testing.T, target interface{}) {
	t.Helper()
	if err := json.Unmarshal(r.Body, target); err != nil {
		t.Fatalf("Failed to parse JSON response: %v\nBody: %s", err, string(r.Body))
	}
}

// BodyString returns the response body as a string.
func (r *TestResponse) BodyString() string {
	return string(r.Body)
}
