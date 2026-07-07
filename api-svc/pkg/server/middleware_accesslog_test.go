package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// SSE handlers (api_pipeline_stream.go) assert w.(http.Flusher); the access-log
// wrapper must not hide that capability from downstream handlers.
func TestAccessLogMiddleware_PreservesFlusher(t *testing.T) {
	var sawFlusher bool
	h := AccessLogMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, sawFlusher = w.(http.Flusher)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/pipeline/stream", nil)
	h.ServeHTTP(httptest.NewRecorder(), req)

	if !sawFlusher {
		t.Fatal("ResponseWriter wrapped by AccessLogMiddleware does not implement http.Flusher — SSE streaming breaks with 500")
	}
}

func TestAccessLogMiddleware_RecordsStatus(t *testing.T) {
	h := AccessLogMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/anything", nil))

	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTeapot)
	}
}
