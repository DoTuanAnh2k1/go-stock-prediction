package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// ---- validateSymbol ----

func TestValidateSymbol_Empty_ReturnsError(t *testing.T) {
	err := validateSymbol("")
	if err == nil {
		t.Error("expected error for empty symbol, got nil")
	}
}

func TestValidateSymbol_Valid_NoError(t *testing.T) {
	validSymbols := []string{"VIC", "VCB", "FPT", "VN", "A1"}
	for _, sym := range validSymbols {
		err := validateSymbol(sym)
		if err != nil {
			t.Errorf("validateSymbol(%q) returned unexpected error: %v", sym, err)
		}
	}
}

func TestValidateSymbol_TooShort_ReturnsError(t *testing.T) {
	err := validateSymbol("V")
	if err == nil {
		t.Error("expected error for 1-char symbol, got nil")
	}
}

func TestValidateSymbol_TooLong_ReturnsError(t *testing.T) {
	// len > 5
	err := validateSymbol("TOOLNG")
	if err == nil {
		t.Error("expected error for symbol > 5 chars, got nil")
	}
}

func TestValidateSymbol_InvalidChars_ReturnsError(t *testing.T) {
	invalidSymbols := []string{"VI!", "V C", "V@C", "VIC#"}
	for _, sym := range invalidSymbols {
		err := validateSymbol(sym)
		if err == nil {
			t.Errorf("validateSymbol(%q) should return error for invalid chars", sym)
		}
	}
}

func TestValidateSymbol_MaxLength5_Valid(t *testing.T) {
	err := validateSymbol("ABCDE")
	if err != nil {
		t.Errorf("validateSymbol(5 chars) returned unexpected error: %v", err)
	}
}

// ---- validateLimit ----

func TestValidateLimit_EmptyString_ReturnsDefault(t *testing.T) {
	limit, err := validateLimit("", 20, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limit != 20 {
		t.Errorf("limit = %v, want default 20", limit)
	}
}

func TestValidateLimit_ValidValue(t *testing.T) {
	limit, err := validateLimit("50", 20, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limit != 50 {
		t.Errorf("limit = %v, want 50", limit)
	}
}

func TestValidateLimit_NegativeValue_ReturnsError(t *testing.T) {
	_, err := validateLimit("-1", 20, 100)
	if err == nil {
		t.Error("expected error for negative limit, got nil")
	}
}

func TestValidateLimit_ExceedsMax_ReturnsError(t *testing.T) {
	_, err := validateLimit("999", 20, 100)
	if err == nil {
		t.Error("expected error for limit exceeding max, got nil")
	}
}

func TestValidateLimit_Zero_ReturnsError(t *testing.T) {
	_, err := validateLimit("0", 20, 100)
	if err == nil {
		t.Error("expected error for zero limit, got nil")
	}
}

func TestValidateLimit_NonNumeric_ReturnsError(t *testing.T) {
	_, err := validateLimit("abc", 20, 100)
	if err == nil {
		t.Error("expected error for non-numeric limit, got nil")
	}
}

func TestValidateLimit_ExactMax_Valid(t *testing.T) {
	limit, err := validateLimit("100", 20, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limit != 100 {
		t.Errorf("limit = %v, want 100", limit)
	}
}

func TestValidateLimit_ExactMin_Valid(t *testing.T) {
	limit, err := validateLimit("1", 20, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limit != 1 {
		t.Errorf("limit = %v, want 1", limit)
	}
}

// ---- validateAlgorithm ----

func TestValidateAlgorithm_Empty_NoError(t *testing.T) {
	err := validateAlgorithm("")
	if err != nil {
		t.Errorf("validateAlgorithm('') should be valid, got: %v", err)
	}
}

func TestValidateAlgorithm_ValidAlgorithms_NoError(t *testing.T) {
	validAlgos := []string{"moving_average", "lstm_nn", "arima_garch"}
	for _, algo := range validAlgos {
		err := validateAlgorithm(algo)
		if err != nil {
			t.Errorf("validateAlgorithm(%q) returned unexpected error: %v", algo, err)
		}
	}
}

func TestValidateAlgorithm_UnknownAlgorithm_ReturnsError(t *testing.T) {
	err := validateAlgorithm("random_forest")
	if err == nil {
		t.Error("expected error for unknown algorithm, got nil")
	}
}

// ---- validatePeriod ----

func TestValidatePeriod_Empty_NoError(t *testing.T) {
	err := validatePeriod("")
	if err != nil {
		t.Errorf("validatePeriod('') should be valid, got: %v", err)
	}
}

func TestValidatePeriod_ValidPeriods_NoError(t *testing.T) {
	validPeriods := []string{"1D", "1W", "1M", "3M", "6M", "1Y"}
	for _, p := range validPeriods {
		err := validatePeriod(p)
		if err != nil {
			t.Errorf("validatePeriod(%q) returned unexpected error: %v", p, err)
		}
	}
}

func TestValidatePeriod_InvalidPeriod_ReturnsError(t *testing.T) {
	err := validatePeriod("2Y")
	if err == nil {
		t.Error("expected error for invalid period, got nil")
	}
}

// ---- getPredictionStatus ----

func TestGetPredictionStatus_WithActualPrice_Confirmed(t *testing.T) {
	price := decimal.NewFromFloat(100.5)
	status := getPredictionStatus(time.Now().Add(-24*time.Hour), &price)
	if status != "confirmed" {
		t.Errorf("status = %v, want 'confirmed'", status)
	}
}

func TestGetPredictionStatus_FutureDate_NoActualPrice_Pending(t *testing.T) {
	futureDate := time.Now().Add(24 * time.Hour)
	status := getPredictionStatus(futureDate, nil)
	if status != "pending" {
		t.Errorf("status = %v, want 'pending'", status)
	}
}

func TestGetPredictionStatus_PastDate_NoActualPrice_PendingConfirmation(t *testing.T) {
	pastDate := time.Now().Add(-24 * time.Hour)
	status := getPredictionStatus(pastDate, nil)
	if status != "pending_confirmation" {
		t.Errorf("status = %v, want 'pending_confirmation'", status)
	}
}

// ---- ResponseError ----

func TestResponseError_SetsStatusCode(t *testing.T) {
	w := httptest.NewRecorder()
	ResponseError(w, http.StatusBadRequest, "bad request")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status code = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestResponseError_SetsContentType(t *testing.T) {
	w := httptest.NewRecorder()
	ResponseError(w, http.StatusBadRequest, "error msg")
	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %v, want application/json", ct)
	}
}

func TestResponseError_BodyContainsMessage(t *testing.T) {
	w := httptest.NewRecorder()
	ResponseError(w, http.StatusBadRequest, "test error message")
	var resp ResponseFailure
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Message != "test error message" {
		t.Errorf("message = %v, want 'test error message'", resp.Message)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status_code in body = %v, want %v", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestResponseError_InternalServerError(t *testing.T) {
	w := httptest.NewRecorder()
	ResponseError(w, http.StatusInternalServerError, "internal error")
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status code = %v, want %v", w.Code, http.StatusInternalServerError)
	}
}

// ---- ResponseSuccess ----

func TestResponseSuccess_SetsStatusCode(t *testing.T) {
	w := httptest.NewRecorder()
	ResponseSuccess(w, http.StatusOK, map[string]string{"key": "value"})
	if w.Code != http.StatusOK {
		t.Errorf("status code = %v, want %v", w.Code, http.StatusOK)
	}
}

func TestResponseSuccess_SetsContentType(t *testing.T) {
	w := httptest.NewRecorder()
	ResponseSuccess(w, http.StatusOK, map[string]string{"key": "value"})
	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %v, want application/json", ct)
	}
}

func TestResponseSuccess_NilData_NoBody(t *testing.T) {
	w := httptest.NewRecorder()
	ResponseSuccess(w, http.StatusNoContent, nil)
	if w.Code != http.StatusNoContent {
		t.Errorf("status code = %v, want %v", w.Code, http.StatusNoContent)
	}
	if w.Body.Len() != 0 {
		t.Errorf("expected empty body for nil data, got: %v", w.Body.String())
	}
}

func TestResponseSuccess_BodyContainsData(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]interface{}{"total": 5, "items": []string{"a", "b"}}
	ResponseSuccess(w, http.StatusOK, data)
	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if result["total"] != float64(5) {
		t.Errorf("total = %v, want 5", result["total"])
	}
}

// ---- calculateVolatility ----

func TestCalculateVolatility_EmptySlice_ReturnsZero(t *testing.T) {
	v := calculateVolatility([]float64{})
	if v != 0 {
		t.Errorf("volatility of empty slice = %v, want 0", v)
	}
}

func TestCalculateVolatility_SingleElement_ReturnsZero(t *testing.T) {
	v := calculateVolatility([]float64{5.0})
	if v != 0 {
		t.Errorf("volatility of single element = %v, want 0", v)
	}
}

func TestCalculateVolatility_IdenticalValues_ReturnsZero(t *testing.T) {
	changes := []float64{1.0, 1.0, 1.0, 1.0, 1.0}
	v := calculateVolatility(changes)
	if v != 0 {
		t.Errorf("volatility of identical values = %v, want 0", v)
	}
}

func TestCalculateVolatility_NonNegative(t *testing.T) {
	changes := []float64{1.0, -2.0, 3.0, -1.5, 0.5}
	v := calculateVolatility(changes)
	if v < 0 {
		t.Errorf("volatility = %v, should be >= 0", v)
	}
}

// ---- getMarketStatus ----

func TestGetMarketStatus_ReturnsValidStatus(t *testing.T) {
	status := getMarketStatus()
	validStatuses := map[string]bool{
		"open":        true,
		"pre_market":  true,
		"after_hours": true,
		"closed":      true,
	}
	if !validStatuses[status] {
		t.Errorf("getMarketStatus returned invalid status: %v", status)
	}
}

// ---- calculateDateRange ----

func TestCalculateDateRange_ValidPeriods(t *testing.T) {
	periods := []string{"1D", "1W", "1M", "3M", "6M", "1Y", "unknown"}
	for _, p := range periods {
		from, to := calculateDateRange(p)
		if from.IsZero() || to.IsZero() {
			t.Errorf("calculateDateRange(%q) returned zero time", p)
		}
		if from.After(to) {
			t.Errorf("calculateDateRange(%q): from (%v) is after to (%v)", p, from, to)
		}
	}
}
