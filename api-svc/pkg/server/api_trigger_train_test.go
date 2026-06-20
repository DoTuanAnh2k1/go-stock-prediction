package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestTriggerTrain_InvalidAlgorithm verifies that posting a JSON body with an
// unknown algorithm name returns 400 before any training is started.
func TestTriggerTrain_InvalidAlgorithm(t *testing.T) {
	unknownAlgos := []string{
		"invalid",
		"linear_regression",
		"gradient_boost",
	}

	for _, algo := range unknownAlgos {
		t.Run(algo, func(t *testing.T) {
			body, _ := json.Marshal(TriggerTrainRequest{Algorithm: algo})
			req := httptest.NewRequest(http.MethodPost, "/api/trigger/train", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			TriggerTrainHandler(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("algorithm=%q: expected 400, got %d", algo, w.Code)
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

// TestTriggerTrain_ValidAlgorithms verifies that recognised algorithm names
// pass validation. Without a predictor initialised the handler returns 500
// (service not ready) — never 400 (validation error).
func TestTriggerTrain_ValidAlgorithms(t *testing.T) {
	validAlgos := []string{"moving_average", "lstm_nn", "arima_garch"}

	for _, algo := range validAlgos {
		t.Run(algo, func(t *testing.T) {
			body, _ := json.Marshal(TriggerTrainRequest{Algorithm: algo})
			req := httptest.NewRequest(http.MethodPost, "/api/trigger/train", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			TriggerTrainHandler(w, req)

			// Validation passed → should not be 400.
			if w.Code == http.StatusBadRequest {
				t.Errorf("algorithm=%q: should not return 400 (validation should pass)", algo)
			}

			// Without a DB / predictor we expect 500, 202, or 409 — all acceptable.
			t.Logf("algorithm=%q returned status %d", algo, w.Code)
		})
	}
}

// TestTriggerTrain_NoBody verifies that sending a request without a body
// (algorithm field defaults to empty string) does not trigger a 400. An empty
// algorithm trains all algorithms, which will fail with 500 when the predictor
// is not initialised — but validation itself must pass.
func TestTriggerTrain_NoBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/trigger/train", nil)
	w := httptest.NewRecorder()
	TriggerTrainHandler(w, req)

	if w.Code == http.StatusBadRequest {
		t.Errorf("no body (train all) should not return 400, got %d", w.Code)
	}
}

// TestTriggerTrain_EmptyAlgorithmField verifies that an empty `algorithm` field
// in the JSON body is treated as "train all" and does not return 400.
func TestTriggerTrain_EmptyAlgorithmField(t *testing.T) {
	body := []byte(`{"algorithm":""}`)
	req := httptest.NewRequest(http.MethodPost, "/api/trigger/train", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	TriggerTrainHandler(w, req)

	if w.Code == http.StatusBadRequest {
		t.Errorf("empty algorithm field should not return 400, got %d", w.Code)
	}
}

// TestTriggerTrain_InvalidJSON verifies that malformed JSON is tolerated
// gracefully (the handler ignores decode errors, treating it as an empty
// algorithm) and does not return 400.
func TestTriggerTrain_InvalidJSON(t *testing.T) {
	body := []byte(`{not valid json}`)
	req := httptest.NewRequest(http.MethodPost, "/api/trigger/train", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	TriggerTrainHandler(w, req)

	// Bad JSON → decoder fails silently → algorithm="" → should proceed as "train all"
	// and result in 500 (predictor nil) or 409 (already training). Never 400.
	if w.Code == http.StatusBadRequest {
		t.Errorf("malformed JSON body should not return 400, got %d", w.Code)
	}
}

// TestTriggerTrain_ResponseBodyIsJSON verifies that the error response from an
// invalid algorithm is well-formed JSON with status_code and message fields.
func TestTriggerTrain_ResponseBodyIsJSON(t *testing.T) {
	body, _ := json.Marshal(TriggerTrainRequest{Algorithm: "invalid_algo"})
	req := httptest.NewRequest(http.MethodPost, "/api/trigger/train", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	TriggerTrainHandler(w, req)

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
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("body status_code = %d, want 400", resp.StatusCode)
	}
	if resp.Message == "" {
		t.Error("error message must not be empty")
	}
}

// TestTriggerTrain_MethodNotAllowed_ViaRouter confirms the router does NOT
// register TriggerTrainHandler for GET. We test this by calling the handler
// directly with a GET — the handler itself does not check the method (method
// filtering is the router's responsibility), so this test documents that
// calling the function with GET still processes the body, i.e., no built-in
// method guard exists in the handler itself.
//
// Note: actual method enforcement lives in router.go (ServeMux). This test
// simply documents the handler's behaviour under a GET request.
func TestTriggerTrain_HandlerDoesNotEnforceGET(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/trigger/train", nil)
	w := httptest.NewRecorder()
	TriggerTrainHandler(w, req)

	// Handler has no method check → it reads body (nil) and proceeds.
	// With predictor nil: expects 500 or 202. Must not be 405.
	if w.Code == http.StatusMethodNotAllowed {
		t.Error("handler itself does not return 405; method enforcement is the router's job")
	}
}
