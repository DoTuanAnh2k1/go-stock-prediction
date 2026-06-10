package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---- GetPredictionCompare: symbol validation ----

func TestGetPredictionCompare_MissingSymbol_Returns400(t *testing.T) {
	// PathValue("symbol") returns "" when the path variable is not set.
	req := httptest.NewRequest(http.MethodGet, "/api/predictions/compare/", nil)
	w := httptest.NewRecorder()
	GetPredictionCompare(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing symbol, got %d", w.Code)
	}
}

func TestGetPredictionCompare_SymbolTooShort_Returns400(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/predictions/compare/V", nil)
	// Set the path value explicitly since httptest does not parse path variables.
	req.SetPathValue("symbol", "V")
	w := httptest.NewRecorder()
	GetPredictionCompare(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for single-char symbol, got %d", w.Code)
	}

	var resp ResponseFailure
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if resp.Message == "" {
		t.Error("error message must not be empty")
	}
}

func TestGetPredictionCompare_SymbolTooLong_Returns400(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/predictions/compare/TOOLONG", nil)
	req.SetPathValue("symbol", "TOOLONG")
	w := httptest.NewRecorder()
	GetPredictionCompare(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for symbol > 5 chars, got %d", w.Code)
	}
}

func TestGetPredictionCompare_InvalidCharsInSymbol_Returns400(t *testing.T) {
	tests := []struct {
		name   string
		symbol string
	}{
		{"at-sign", "VI@C"},
		{"exclamation", "VIC!"},
		{"hash", "VIC#"},
		{"dot", "V.CB"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Use a safe path in the URL; set the actual symbol via SetPathValue
			// (as the router would do) so the handler sees the raw symbol string.
			req := httptest.NewRequest(http.MethodGet, "/api/predictions/compare/SAFE", nil)
			req.SetPathValue("symbol", tc.symbol)
			w := httptest.NewRecorder()
			GetPredictionCompare(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("symbol=%q: expected 400, got %d", tc.symbol, w.Code)
			}

			ct := w.Header().Get("Content-Type")
			if ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
		})
	}
}

func TestGetPredictionCompare_SymbolWithSpace_Returns400(t *testing.T) {
	// A space is an invalid character in a symbol. We set the path value directly
	// without encoding it in the URL to avoid httptest.NewRequest panicking.
	req := httptest.NewRequest(http.MethodGet, "/api/predictions/compare/SAFE", nil)
	req.SetPathValue("symbol", "VI C")
	w := httptest.NewRecorder()
	GetPredictionCompare(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("symbol with space: expected 400, got %d", w.Code)
	}
}

// ---- GetPredictionCompare: algorithm query param validation ----

func TestGetPredictionCompare_InvalidAlgorithm_Returns400(t *testing.T) {
	tests := []struct {
		name      string
		algorithm string
	}{
		{"random_forest", "random_forest"},
		{"xgboost", "xgboost"},
		{"linear_regression", "linear_regression"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet,
				"/api/predictions/compare/VCB?algorithm="+tc.algorithm, nil)
			req.SetPathValue("symbol", "VCB")
			w := httptest.NewRecorder()
			GetPredictionCompare(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("algorithm=%q: expected 400, got %d", tc.algorithm, w.Code)
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

func TestGetPredictionCompare_ValidAlgorithmFilters_PassValidation(t *testing.T) {
	validAlgos := []string{"lstm_nn", "arima_garch", "moving_average", "ensemble"}

	for _, algo := range validAlgos {
		t.Run(algo, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet,
				"/api/predictions/compare/VCB?algorithm="+algo, nil)
			req.SetPathValue("symbol", "VCB")
			w := httptest.NewRecorder()

			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Logf("algorithm=%s passed validation (DB not available): %v", algo, r)
					}
				}()
				GetPredictionCompare(w, req)
			}()

			if w.Code != 0 && w.Code == http.StatusBadRequest {
				t.Errorf("algorithm=%q: should not return 400 (valid algorithm)", algo)
			}
		})
	}
}

func TestGetPredictionCompare_EmptyAlgorithm_PassesValidation(t *testing.T) {
	// No algorithm filter → validateAlgorithm("") is valid.
	req := httptest.NewRequest(http.MethodGet, "/api/predictions/compare/VCB", nil)
	req.SetPathValue("symbol", "VCB")
	w := httptest.NewRecorder()

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("empty algorithm passed validation (DB not available): %v", r)
			}
		}()
		GetPredictionCompare(w, req)
	}()

	if w.Code != 0 && w.Code == http.StatusBadRequest {
		t.Errorf("empty algorithm should not return 400, got %d", w.Code)
	}
}

// ---- GetPredictionCompare: days query param ----

func TestGetPredictionCompare_ValidDaysParam_PassesValidation(t *testing.T) {
	// Valid ?days param → should proceed past validation (not return 400).
	dayValues := []string{"1", "14", "30", "90", "365"}

	for _, d := range dayValues {
		t.Run("days="+d, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet,
				"/api/predictions/compare/VCB?days="+d, nil)
			req.SetPathValue("symbol", "VCB")
			w := httptest.NewRecorder()

			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Logf("days=%s passed validation (DB not available): %v", d, r)
					}
				}()
				GetPredictionCompare(w, req)
			}()

			if w.Code != 0 && w.Code == http.StatusBadRequest {
				t.Errorf("days=%s: should not return 400 for valid days param", d)
			}
		})
	}
}

func TestGetPredictionCompare_InvalidDaysParam_FallsBackToDefault(t *testing.T) {
	// Non-numeric or out-of-range days → handler silently falls back to 30.
	// This means the request still proceeds past validation (no 400).
	invalidDays := []string{"abc", "-1", "0", "366", "999"}

	for _, d := range invalidDays {
		t.Run("days="+d, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet,
				"/api/predictions/compare/VCB?days="+d, nil)
			req.SetPathValue("symbol", "VCB")
			w := httptest.NewRecorder()

			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Logf("days=%s silently ignored, proceeds to DB (not available): %v", d, r)
					}
				}()
				GetPredictionCompare(w, req)
			}()

			// Invalid days does NOT return 400 — the handler uses the default (30).
			if w.Code != 0 && w.Code == http.StatusBadRequest {
				t.Errorf("days=%s: invalid days should fall back to default, not return 400", d)
			}
		})
	}
}

// ---- GetPredictionCompare: valid symbol formats ----

func TestGetPredictionCompare_ValidSymbols_PassValidation(t *testing.T) {
	validSymbols := []string{"VN", "VCB", "HOSE", "ABCDE"}

	for _, sym := range validSymbols {
		t.Run(sym, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet,
				"/api/predictions/compare/"+sym, nil)
			req.SetPathValue("symbol", sym)
			w := httptest.NewRecorder()

			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Logf("symbol=%s passed validation (DB not available): %v", sym, r)
					}
				}()
				GetPredictionCompare(w, req)
			}()

			if w.Code != 0 && w.Code == http.StatusBadRequest {
				t.Errorf("symbol=%q: should not return 400 for valid symbol", sym)
			}
		})
	}
}

// ---- GetPredictionCompare: response format ----

func TestGetPredictionCompare_ValidationError_ResponseIsJSON(t *testing.T) {
	// Trigger a guaranteed validation error (bad symbol chars).
	req := httptest.NewRequest(http.MethodGet, "/api/predictions/compare/!!!!", nil)
	req.SetPathValue("symbol", "!!!!")
	w := httptest.NewRecorder()
	GetPredictionCompare(w, req)

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

// ---- GetErrorDistribution: algorithm query param validation ----

func TestGetErrorDistribution_InvalidAlgorithm_Returns400(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet,
		"/api/predictions/error-distribution?algorithm=invalid_algo", nil)
	w := httptest.NewRecorder()
	GetErrorDistribution(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid algorithm, got %d", w.Code)
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
}

func TestGetErrorDistribution_EmptyAlgorithm_PassesValidation(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/predictions/error-distribution", nil)
	w := httptest.NewRecorder()

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("empty algorithm passed validation (DB not available): %v", r)
			}
		}()
		GetErrorDistribution(w, req)
	}()

	if w.Code != 0 && w.Code == http.StatusBadRequest {
		t.Errorf("empty algorithm should not return 400, got %d", w.Code)
	}
}

func TestGetErrorDistribution_ValidAlgorithms_PassValidation(t *testing.T) {
	validAlgos := []string{"lstm_nn", "arima_garch", "moving_average", "ensemble"}

	for _, algo := range validAlgos {
		t.Run(algo, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet,
				"/api/predictions/error-distribution?algorithm="+algo, nil)
			w := httptest.NewRecorder()

			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Logf("algorithm=%s passed validation (DB not available): %v", algo, r)
					}
				}()
				GetErrorDistribution(w, req)
			}()

			if w.Code != 0 && w.Code == http.StatusBadRequest {
				t.Errorf("algorithm=%q: should not return 400", algo)
			}
		})
	}
}

// ---- PredictionCompareDTO structure ----

func TestPredictionCompareDTO_JSONFieldNames(t *testing.T) {
	dto := PredictionCompareDTO{
		Symbol: "VCB",
		Data:   []PredictionComparePoint{},
	}
	b, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("failed to marshal PredictionCompareDTO: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	requiredFields := []string{"symbol", "data"}
	for _, field := range requiredFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("PredictionCompareDTO JSON is missing required field %q", field)
		}
	}
}

func TestPredictionComparePoint_JSONFieldNames(t *testing.T) {
	// Use a zero-value decimal — json marshal should still produce valid JSON.
	point := PredictionComparePoint{
		Date:      "2025-01-15",
		Algorithm: "lstm_nn",
	}
	b, err := json.Marshal(point)
	if err != nil {
		t.Fatalf("failed to marshal PredictionComparePoint: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	requiredFields := []string{"date", "predicted", "actual", "algorithm"}
	for _, field := range requiredFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("PredictionComparePoint JSON is missing required field %q", field)
		}
	}
}

func TestErrorDistributionDTO_JSONFieldNames(t *testing.T) {
	dto := ErrorDistributionDTO{
		Data: []ErrorDistributionPoint{},
	}
	b, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("failed to marshal ErrorDistributionDTO: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if _, ok := raw["data"]; !ok {
		t.Error("ErrorDistributionDTO JSON is missing required field 'data'")
	}
}

func TestErrorDistributionPoint_JSONFieldNames(t *testing.T) {
	point := ErrorDistributionPoint{
		PredictedChangePct: 1.5,
		ActualChangePct:    2.0,
		Algorithm:          "arima_garch",
		Symbol:             "VCB",
	}
	b, err := json.Marshal(point)
	if err != nil {
		t.Fatalf("failed to marshal ErrorDistributionPoint: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	requiredFields := []string{"predicted_change_pct", "actual_change_pct", "algorithm", "symbol"}
	for _, field := range requiredFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("ErrorDistributionPoint JSON is missing required field %q", field)
		}
	}
}
