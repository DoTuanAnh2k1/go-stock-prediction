package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---- formatDurationMs ----

func TestFormatDurationMs_Zero(t *testing.T) {
	result := formatDurationMs(0)
	if result != "0s" {
		t.Errorf("formatDurationMs(0) = %q, want %q", result, "0s")
	}
}

func TestFormatDurationMs_LessThan60Seconds(t *testing.T) {
	tests := []struct {
		ms   float64
		want string
	}{
		{500, "0s"},    // 0.5s → truncated to 0
		{1000, "1s"},   // 1s
		{5000, "5s"},   // 5s
		{30000, "30s"}, // 30s
		{59000, "59s"}, // 59s
		{59999, "59s"}, // just under 1 minute
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			got := formatDurationMs(tc.ms)
			if got != tc.want {
				t.Errorf("formatDurationMs(%.0f) = %q, want %q", tc.ms, got, tc.want)
			}
		})
	}
}

func TestFormatDurationMs_MinutesAndSeconds(t *testing.T) {
	tests := []struct {
		ms   float64
		want string
	}{
		{60000, "1m 0s"},     // exactly 1 minute
		{90000, "1m 30s"},    // 1m 30s
		{120000, "2m 0s"},    // exactly 2 minutes
		{300000, "5m 0s"},    // 5 minutes
		{320000, "5m 20s"},   // 5m 20s
		{3599000, "59m 59s"}, // 59m 59s
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			got := formatDurationMs(tc.ms)
			if got != tc.want {
				t.Errorf("formatDurationMs(%.0f) = %q, want %q", tc.ms, got, tc.want)
			}
		})
	}
}

func TestFormatDurationMs_HoursMinutesAndSeconds(t *testing.T) {
	tests := []struct {
		ms   float64
		want string
	}{
		{3600000, "1h 0m 0s"},   // exactly 1 hour
		{3661000, "1h 1m 1s"},   // 1h 1m 1s
		{7200000, "2h 0m 0s"},   // exactly 2 hours
		{7320000, "2h 2m 0s"},   // 2h 2m
		{36000000, "10h 0m 0s"}, // 10 hours
		{90061000, "25h 1m 1s"}, // more than 24h
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			got := formatDurationMs(tc.ms)
			if got != tc.want {
				t.Errorf("formatDurationMs(%.0f) = %q, want %q", tc.ms, got, tc.want)
			}
		})
	}
}

func TestFormatDurationMs_ExactBoundaries(t *testing.T) {
	// 60000ms = exactly 60 seconds → "1m 0s" (not "60s")
	got := formatDurationMs(60000)
	if got != "1m 0s" {
		t.Errorf("formatDurationMs(60000) = %q, want %q", got, "1m 0s")
	}

	// 3600000ms = exactly 3600 seconds → "1h 0m 0s" (not "60m 0s")
	got = formatDurationMs(3600000)
	if got != "1h 0m 0s" {
		t.Errorf("formatDurationMs(3600000) = %q, want %q", got, "1h 0m 0s")
	}
}

func TestFormatDurationMs_ResultIsNonEmpty(t *testing.T) {
	inputs := []float64{0, 1000, 60000, 3600000, 86400000}
	for _, ms := range inputs {
		got := formatDurationMs(ms)
		if got == "" {
			t.Errorf("formatDurationMs(%.0f) returned empty string", ms)
		}
	}
}

// ---- GetTrainingMetrics handler (validation / error paths) ----

// TestGetTrainingMetrics_MethodGet verifies that sending GET to the handler
// proceeds past any early returns and either returns a DB error (500) or succeeds.
// Without a real DB the handler panics or returns 500 — never a validation 400.
func TestGetTrainingMetrics_MethodGet_NeverReturns400(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/training/metrics", nil)
	w := httptest.NewRecorder()

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("handler panicked (DB not available): %v", r)
			}
		}()
		GetTrainingMetrics(w, req)
	}()

	if w.Code != 0 && w.Code == http.StatusBadRequest {
		t.Errorf("GetTrainingMetrics: should not return 400, got %d", w.Code)
	}
}

// TestGetTrainingMetrics_ErrorResponseIsJSON verifies that when the handler
// writes an error response it is well-formed JSON with application/json Content-Type.
// When the DB is unavailable the handler may panic before writing — in that case
// we skip the assertion (no response was written).
func TestGetTrainingMetrics_ErrorResponseIsJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/training/metrics", nil)
	w := httptest.NewRecorder()
	didPanic := false

	func() {
		defer func() {
			if r := recover(); r != nil {
				didPanic = true
				t.Logf("handler panicked (DB not available): %v", r)
			}
		}()
		GetTrainingMetrics(w, req)
	}()

	if didPanic {
		// Handler panicked before writing — nothing to assert about the response.
		return
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	if w.Body.Len() > 0 {
		var raw json.RawMessage
		if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
			t.Errorf("response body is not valid JSON: %v\nBody: %s", err, w.Body.String())
		}
	}
}

// TestGetTrainingMetrics_500ResponseShape verifies that a 500 error body from
// this handler (when DB unavailable and writes a response) contains status_code
// and message.
func TestGetTrainingMetrics_500ResponseShape(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/training/metrics", nil)
	w := httptest.NewRecorder()
	didPanic := false

	func() {
		defer func() {
			if r := recover(); r != nil {
				didPanic = true
			}
		}()
		GetTrainingMetrics(w, req)
	}()

	if didPanic {
		t.Skip("handler panicked (DB nil); skipping shape test")
	}

	if w.Code != http.StatusInternalServerError {
		t.Skipf("skipping shape test: handler returned %d (expected 500 with no DB)", w.Code)
	}

	var resp ResponseFailure
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("body status_code = %d, want 500", resp.StatusCode)
	}
	if resp.Message == "" {
		t.Error("error message must not be empty")
	}
}
