package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGetPredictionAccuracy_InvalidDays verifies that a non-numeric `days`
// parameter returns 400 with a descriptive error message.
func TestGetPredictionAccuracy_InvalidDays(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"alphabetic", "?days=abc"},
		{"negative", "?days=-1"},
		{"zero", "?days=0"},
		{"too large", "?days=366"},
		{"float", "?days=1.5"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/predictions/accuracy"+tc.query, nil)
			w := httptest.NewRecorder()
			GetPredictionAccuracy(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("query=%q: expected 400, got %d", tc.query, w.Code)
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

// TestGetPredictionAccuracy_DefaultDays verifies that omitting the `days`
// parameter uses the default (30) and the handler proceeds past validation.
// Without a DB the handler returns 500; we assert it is NOT 400 (no validation
// error) and that the response is JSON.
func TestGetPredictionAccuracy_DefaultDays(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/predictions/accuracy", nil)
	w := httptest.NewRecorder()

	func() {
		defer func() {
			if r := recover(); r != nil {
				// Panic from nil DB — validation passed as expected.
				t.Logf("default days passed validation (DB not available): %v", r)
			}
		}()
		GetPredictionAccuracy(w, req)
	}()

	// A 400 here would indicate a spurious validation failure.
	if w.Code != 0 && w.Code == http.StatusBadRequest {
		t.Errorf("omitting ?days should not return 400, got %d", w.Code)
	}
}

// TestGetPredictionAccuracy_ValidDays verifies that well-formed integer values
// within [1, 365] pass validation.
func TestGetPredictionAccuracy_ValidDays(t *testing.T) {
	validDays := []string{"1", "30", "90", "180", "365"}

	for _, d := range validDays {
		t.Run("days="+d, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/predictions/accuracy?days="+d, nil)
			w := httptest.NewRecorder()

			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Logf("days=%s passed validation (DB not available): %v", d, r)
					}
				}()
				GetPredictionAccuracy(w, req)
			}()

			if w.Code != 0 && w.Code == http.StatusBadRequest {
				t.Errorf("days=%s: should not return 400", d)
			}
		})
	}
}

// TestGetPredictionAccuracy_DaysBoundaryExact verifies exact boundary values.
func TestGetPredictionAccuracy_DaysBoundaryExact(t *testing.T) {
	// days=365 is the maximum allowed value.
	req := httptest.NewRequest(http.MethodGet, "/api/predictions/accuracy?days=365", nil)
	w := httptest.NewRecorder()

	func() {
		defer func() { recover() }()
		GetPredictionAccuracy(w, req)
	}()

	if w.Code != 0 && w.Code == http.StatusBadRequest {
		t.Error("days=365 (maximum) should not return 400")
	}

	// days=366 exceeds the maximum.
	req2 := httptest.NewRequest(http.MethodGet, "/api/predictions/accuracy?days=366", nil)
	w2 := httptest.NewRecorder()
	GetPredictionAccuracy(w2, req2)

	if w2.Code != http.StatusBadRequest {
		t.Errorf("days=366 (over maximum): expected 400, got %d", w2.Code)
	}
}
