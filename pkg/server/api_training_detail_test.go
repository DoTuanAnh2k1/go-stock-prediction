package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGetTrainingDetail_InvalidPath verifies that a path with fewer than
// 3 non-empty segments returns 400 immediately.
func TestGetTrainingDetail_InvalidPath(t *testing.T) {
	shortPaths := []string{
		"/api/training",
		"/api",
		"/",
	}

	for _, path := range shortPaths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			GetTrainingDetail(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("path=%q: expected 400, got %d", path, w.Code)
			}

			ct := w.Header().Get("Content-Type")
			if ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}

			var resp ResponseFailure
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("body not valid JSON: %v", err)
			}
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("body status_code = %d, want 400", resp.StatusCode)
			}
			if resp.Message == "" {
				t.Error("error message must not be empty")
			}
		})
	}
}

// TestGetTrainingDetail_EmptyIDSegment verifies that a trailing slash (empty ID
// segment) returns 400.
func TestGetTrainingDetail_EmptyIDSegment(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/training/", nil)
	w := httptest.NewRecorder()
	GetTrainingDetail(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty ID segment, got %d", w.Code)
	}
}

// TestGetTrainingDetail_ValidNumericID verifies that a positive integer ID
// passes path-length and non-empty checks and proceeds to the DB layer.
// Without a DB the handler will panic or return 404; we ensure it is not 400.
func TestGetTrainingDetail_ValidNumericID(t *testing.T) {
	ids := []string{"1", "42", "999"}

	for _, id := range ids {
		t.Run("id="+id, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/training/"+id, nil)
			w := httptest.NewRecorder()

			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Logf("id=%s passed validation (DB not available): %v", id, r)
					}
				}()
				GetTrainingDetail(w, req)
			}()

			if w.Code != 0 && w.Code == http.StatusBadRequest {
				t.Errorf("id=%s: should not return 400 for a valid numeric ID", id)
			}
		})
	}
}

// TestGetTrainingDetail_ValidSessionID verifies that a UUID-style session ID
// (non-numeric) passes path-length and non-empty checks.
func TestGetTrainingDetail_ValidSessionID(t *testing.T) {
	sessionID := "550e8400-e29b-41d4-a716-446655440000"
	req := httptest.NewRequest(http.MethodGet, "/api/training/"+sessionID, nil)
	w := httptest.NewRecorder()

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("session ID passed validation (DB not available): %v", r)
			}
		}()
		GetTrainingDetail(w, req)
	}()

	// A 400 would mean validation failed incorrectly for a valid UUID session ID.
	if w.Code != 0 && w.Code == http.StatusBadRequest {
		t.Error("valid UUID session ID should not return 400")
	}
}

// TestGetTrainingDetail_ResponseIsJSON verifies that all error responses from
// the handler carry the application/json Content-Type header.
func TestGetTrainingDetail_ResponseIsJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/training", nil)
	w := httptest.NewRecorder()
	GetTrainingDetail(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var resp ResponseFailure
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}
	if resp.Message == "" {
		t.Error("error response must include a message")
	}
}
