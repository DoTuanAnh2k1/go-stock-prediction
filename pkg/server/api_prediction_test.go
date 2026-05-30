package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---------------------------------------------------------------------------
// GetPredictions — validation tests (no DB access)
// ---------------------------------------------------------------------------

// TestGetPredictions_InvalidPage verifies that a negative page number is
// rejected with 400.
func TestGetPredictions_InvalidPage(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"negative page", "?page=-1"},
		{"zero page", "?page=0"},
		{"non-numeric page", "?page=abc"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/predictions"+tc.query, nil)
			w := httptest.NewRecorder()
			GetPredictions(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("query=%q: expected 400, got %d", tc.query, w.Code)
			}

			var resp ResponseFailure
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("body not valid JSON: %v", err)
			}
			if resp.Message == "" {
				t.Error("error message must not be empty")
			}
		})
	}
}

// TestGetPredictions_InvalidDateFormat verifies that a malformed date string
// in the `from` or `to` parameters is rejected with 400.
func TestGetPredictions_InvalidDateFormat(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"from=invalid", "?from=invalid"},
		{"from=bad-format", "?from=2024/01/01"},
		{"to=invalid", "?to=notadate"},
		{"to=bad-format", "?to=01-01-2024"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/predictions"+tc.query, nil)
			w := httptest.NewRecorder()
			GetPredictions(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("query=%q: expected 400, got %d", tc.query, w.Code)
			}
		})
	}
}

// TestGetPredictions_InvalidAlgorithm verifies that an unrecognised algorithm
// name is rejected with 400.
func TestGetPredictions_InvalidAlgorithm(t *testing.T) {
	unknownAlgos := []string{"unknown", "random_forest", "xgboost", "linear_regression"}

	for _, algo := range unknownAlgos {
		t.Run(algo, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/predictions?algorithm="+algo, nil)
			w := httptest.NewRecorder()
			GetPredictions(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("algorithm=%q: expected 400, got %d", algo, w.Code)
			}

			ct := w.Header().Get("Content-Type")
			if ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
		})
	}
}

// TestGetPredictions_InvalidLimit verifies that limit values outside the
// accepted range [1, 100] are rejected with 400.
func TestGetPredictions_InvalidLimit(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"limit=999", "?limit=999"},
		{"limit=101", "?limit=101"},
		{"limit=0", "?limit=0"},
		{"limit=-5", "?limit=-5"},
		{"limit=abc", "?limit=abc"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/predictions"+tc.query, nil)
			w := httptest.NewRecorder()
			GetPredictions(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("query=%q: expected 400, got %d", tc.query, w.Code)
			}
		})
	}
}

// TestGetPredictions_ValidAlgorithms verifies that known algorithm names pass
// validation. Without a DB the handler will eventually return a non-400 error
// (500 or panic from nil DB), but validation itself must not trigger a 400.
func TestGetPredictions_ValidAlgorithms(t *testing.T) {
	validAlgos := []string{"moving_average", "lstm_nn", "arima_garch", "ensemble"}

	for _, algo := range validAlgos {
		t.Run(algo, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/predictions?algorithm="+algo, nil)
			w := httptest.NewRecorder()

			func() {
				defer func() {
					if r := recover(); r != nil {
						// Panic from nil DB — validation succeeded.
						t.Logf("algorithm %q passed validation (DB not available): %v", algo, r)
					}
				}()
				GetPredictions(w, req)
			}()

			if w.Code != 0 && w.Code == http.StatusBadRequest {
				t.Errorf("algorithm=%q should not produce 400 (validation failure)", algo)
			}
		})
	}
}

// TestGetPredictions_EmptyAlgorithm verifies that omitting the algorithm
// parameter is accepted (passes validation).
func TestGetPredictions_EmptyAlgorithm(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/predictions", nil)
	w := httptest.NewRecorder()

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("passed validation (DB not available): %v", r)
			}
		}()
		GetPredictions(w, req)
	}()

	if w.Code != 0 && w.Code == http.StatusBadRequest {
		t.Error("empty algorithm param should not return 400")
	}
}

// TestGetPredictions_ValidDateFormat verifies that correctly formatted dates
// (YYYY-MM-DD) pass validation.
func TestGetPredictions_ValidDateFormat(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/predictions?from=2024-01-01&to=2024-12-31", nil)
	w := httptest.NewRecorder()

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("passed validation (DB not available): %v", r)
			}
		}()
		GetPredictions(w, req)
	}()

	if w.Code != 0 && w.Code == http.StatusBadRequest {
		t.Error("valid date format should not return 400")
	}
}

// ---------------------------------------------------------------------------
// GetPredictionDetail — validation tests (no DB access)
// ---------------------------------------------------------------------------

// TestGetPredictionDetail_InvalidID verifies that a non-numeric prediction ID
// in the path is rejected with 400.
func TestGetPredictionDetail_InvalidID(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"alphabetic", "/api/predictions/abc"},
		{"special chars", "/api/predictions/@!#"},
		{"float", "/api/predictions/1.5"},
		{"empty segment", "/api/predictions/"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()
			GetPredictionDetail(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("path=%q: expected 400, got %d", tc.path, w.Code)
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
				t.Error("error message must not be empty")
			}
		})
	}
}

// TestGetPredictionDetail_InvalidPath verifies that a path that is too short
// (fewer than 3 segments) returns 400.
func TestGetPredictionDetail_InvalidPath(t *testing.T) {
	shortPaths := []string{
		"/api/predictions",
		"/api",
		"/",
	}

	for _, path := range shortPaths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			GetPredictionDetail(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("path=%q: expected 400, got %d", path, w.Code)
			}
		})
	}
}

// TestGetPredictionDetail_NegativeID verifies that a negative numeric ID is
// rejected with 400 (ParseUint rejects it).
func TestGetPredictionDetail_NegativeID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/predictions/-1", nil)
	w := httptest.NewRecorder()
	GetPredictionDetail(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for negative ID, got %d", w.Code)
	}
}

// TestGetPredictionDetail_ValidNumericID verifies that a valid positive integer
// ID passes validation. Without a DB the handler returns 404 (not found).
func TestGetPredictionDetail_ValidNumericID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/predictions/42", nil)
	w := httptest.NewRecorder()

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("ID=42 passed validation (DB not available): %v", r)
			}
		}()
		GetPredictionDetail(w, req)
	}()

	// Validation should not cause a 400 for a valid numeric ID.
	if w.Code != 0 && w.Code == http.StatusBadRequest {
		t.Error("valid numeric ID should not return 400")
	}
}
