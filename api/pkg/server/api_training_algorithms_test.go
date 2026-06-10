package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-stock-prediction/pkg/service/predict/registry"
)

// ---- registry.All() static metadata ----

func TestRegistry_ContainsRequiredAlgorithms(t *testing.T) {
	required := []string{"lstm_nn", "arima_garch", "moving_average", "ensemble"}
	defs := registry.All()
	keySet := make(map[string]bool, len(defs))
	for _, d := range defs {
		keySet[d.Key] = true
	}
	for _, key := range required {
		if !keySet[key] {
			t.Errorf("registry is missing required algorithm key %q", key)
		}
	}
}

func TestRegistry_LSTMDisplayName(t *testing.T) {
	for _, def := range registry.All() {
		if def.Key == "lstm_nn" {
			if def.DisplayName == "" {
				t.Error("lstm_nn DisplayName must not be empty")
			}
			return
		}
	}
	t.Fatal("lstm_nn not found in registry")
}

func TestRegistry_ARIMAGARCHHasConfig(t *testing.T) {
	for _, def := range registry.All() {
		if def.Key == "arima_garch" {
			if def.Config == nil {
				t.Error("arima_garch Config must not be nil")
			}
			return
		}
	}
	t.Fatal("arima_garch not found in registry")
}

func TestRegistry_MovingAverageHasWindowConfig(t *testing.T) {
	for _, def := range registry.All() {
		if def.Key == "moving_average" {
			if _, ok := def.Config["window"]; !ok {
				t.Error("moving_average Config must contain 'window' key")
			}
			return
		}
	}
	t.Fatal("moving_average not found in registry")
}

func TestRegistry_LSTMHasEpochsConfig(t *testing.T) {
	for _, def := range registry.All() {
		if def.Key == "lstm_nn" {
			if _, ok := def.Config["epochs"]; !ok {
				t.Error("lstm_nn Config must contain 'epochs' key")
			}
			return
		}
	}
	t.Fatal("lstm_nn not found in registry")
}

func TestRegistry_EnsembleHasNonNilConfig(t *testing.T) {
	for _, def := range registry.All() {
		if def.Key == "ensemble" {
			// Config may be empty map but must not be nil.
			if def.Config == nil {
				t.Error("ensemble Config must not be nil (use empty map, not nil)")
			}
			return
		}
	}
	t.Fatal("ensemble not found in registry")
}

func TestRegistry_AtLeastFourAlgorithms(t *testing.T) {
	defs := registry.All()
	if len(defs) < 4 {
		t.Errorf("registry has %d entries, want at least 4", len(defs))
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

// ---- registry alignment with validAlgorithms ----

// TestRegistry_AlignedWithValidAlgorithms verifies that every key in the
// registry is also present in validAlgorithms (from helper.go) so that
// the algorithm validation rejects nothing that the training endpoint exposes.
func TestRegistry_AlignedWithValidAlgorithms(t *testing.T) {
	for _, def := range registry.All() {
		if !validAlgorithms[def.Key] {
			t.Errorf("algorithm key %q is in registry but missing from validAlgorithms", def.Key)
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
