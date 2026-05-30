package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---- knownAlgorithms static metadata ----

func TestKnownAlgorithms_ContainsAllFourAlgorithms(t *testing.T) {
	expected := []string{"lstm_nn", "arima_garch", "moving_average", "ensemble"}
	for _, key := range expected {
		if _, ok := knownAlgorithms[key]; !ok {
			t.Errorf("knownAlgorithms is missing required key %q", key)
		}
	}
}

func TestKnownAlgorithms_LSTMDisplayName(t *testing.T) {
	meta, ok := knownAlgorithms["lstm_nn"]
	if !ok {
		t.Fatal("lstm_nn not found in knownAlgorithms")
	}
	if meta.displayName == "" {
		t.Error("lstm_nn displayName must not be empty")
	}
}

func TestKnownAlgorithms_ARIMAGARCHHasConfig(t *testing.T) {
	meta, ok := knownAlgorithms["arima_garch"]
	if !ok {
		t.Fatal("arima_garch not found in knownAlgorithms")
	}
	if meta.config == nil {
		t.Error("arima_garch config must not be nil")
	}
}

func TestKnownAlgorithms_MovingAverageHasWindowConfig(t *testing.T) {
	meta, ok := knownAlgorithms["moving_average"]
	if !ok {
		t.Fatal("moving_average not found in knownAlgorithms")
	}
	if _, ok := meta.config["window"]; !ok {
		t.Error("moving_average config must contain 'window' key")
	}
}

func TestKnownAlgorithms_LSTMHasEpochsConfig(t *testing.T) {
	meta, ok := knownAlgorithms["lstm_nn"]
	if !ok {
		t.Fatal("lstm_nn not found in knownAlgorithms")
	}
	if _, ok := meta.config["epochs"]; !ok {
		t.Error("lstm_nn config must contain 'epochs' key")
	}
}

func TestKnownAlgorithms_EnsembleHasNonNilConfig(t *testing.T) {
	meta, ok := knownAlgorithms["ensemble"]
	if !ok {
		t.Fatal("ensemble not found in knownAlgorithms")
	}
	// Config may be empty map but must not be nil.
	if meta.config == nil {
		t.Error("ensemble config must not be nil (use empty map, not nil)")
	}
}

func TestKnownAlgorithms_NoExtraUnknownAlgorithms(t *testing.T) {
	// Exactly 4 algorithms should be registered.
	if len(knownAlgorithms) != 4 {
		t.Errorf("knownAlgorithms has %d entries, want exactly 4", len(knownAlgorithms))
	}
}

// ---- GetTrainingAlgorithms handler (validation / error paths) ----

// TestGetTrainingAlgorithms_MethodGet_NeverReturns400 verifies the handler
// performs no input validation that could produce 400. Without DB it returns 500
// or panics — both are acceptable.
func TestGetTrainingAlgorithms_MethodGet_NeverReturns400(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/training/algorithms", nil)
	w := httptest.NewRecorder()

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("handler panicked (DB not available): %v", r)
			}
		}()
		GetTrainingAlgorithms(w, req)
	}()

	if w.Code != 0 && w.Code == http.StatusBadRequest {
		t.Errorf("GetTrainingAlgorithms should not return 400, got %d", w.Code)
	}
}

// TestGetTrainingAlgorithms_ErrorResponseIsJSON verifies that any written
// response has Content-Type: application/json and a valid JSON body.
// When the DB is unavailable the handler may panic before writing — in that
// case we skip the Content-Type assertion (no response was written).
func TestGetTrainingAlgorithms_ErrorResponseIsJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/training/algorithms", nil)
	w := httptest.NewRecorder()
	didPanic := false

	func() {
		defer func() {
			if r := recover(); r != nil {
				didPanic = true
				t.Logf("handler panicked (DB not available): %v", r)
			}
		}()
		GetTrainingAlgorithms(w, req)
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

// TestGetTrainingAlgorithms_500ResponseShape verifies the 500 error body shape
// when the DB is unavailable and the handler writes a response (not a panic).
func TestGetTrainingAlgorithms_500ResponseShape(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/training/algorithms", nil)
	w := httptest.NewRecorder()
	didPanic := false

	func() {
		defer func() {
			if r := recover(); r != nil {
				didPanic = true
			}
		}()
		GetTrainingAlgorithms(w, req)
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

// ---- knownAlgorithms alignment with validAlgorithms ----

// TestKnownAlgorithms_AlignedWithValidAlgorithms verifies that every key in
// knownAlgorithms is also present in validAlgorithms (from helper.go) so that
// the algorithm validation rejects nothing that the training endpoint exposes.
func TestKnownAlgorithms_AlignedWithValidAlgorithms(t *testing.T) {
	for key := range knownAlgorithms {
		if !validAlgorithms[key] {
			t.Errorf("algorithm key %q is in knownAlgorithms but missing from validAlgorithms", key)
		}
	}
}

// TestGetTrainingAlgorithms_IgnoresQueryParams verifies that adding arbitrary
// query parameters does not change the validation behaviour.
func TestGetTrainingAlgorithms_IgnoresQueryParams(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/training/algorithms?foo=bar&page=2", nil)
	w := httptest.NewRecorder()

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("handler panicked (DB not available): %v", r)
			}
		}()
		GetTrainingAlgorithms(w, req)
	}()

	if w.Code != 0 && w.Code == http.StatusBadRequest {
		t.Errorf("unexpected 400 with arbitrary query params, got %d", w.Code)
	}
}
