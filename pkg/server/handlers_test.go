package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGetPredictions_* tests cover validation paths (400 responses)
// that execute entirely before any database access.

func TestGetPredictions_InvalidSymbolChars_Returns400(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/predictions?symbol=INVALID!!!", nil)
	w := httptest.NewRecorder()
	GetPredictions(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestGetPredictions_SymbolTooLong_Returns400(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/predictions?symbol=TOOLONG", nil)
	w := httptest.NewRecorder()
	GetPredictions(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestGetPredictions_SymbolTooShort_Returns400(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/predictions?symbol=V", nil)
	w := httptest.NewRecorder()
	GetPredictions(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestGetPredictions_InvalidLimitNonNumeric_Returns400(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/predictions?limit=abc", nil)
	w := httptest.NewRecorder()
	GetPredictions(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestGetPredictions_LimitExceedsMax_Returns400(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/predictions?limit=999", nil)
	w := httptest.NewRecorder()
	GetPredictions(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestGetPredictions_NegativeLimit_Returns400(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/predictions?limit=-1", nil)
	w := httptest.NewRecorder()
	GetPredictions(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestGetPredictions_UnknownAlgorithm_Returns400(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/predictions?algorithm=random_forest", nil)
	w := httptest.NewRecorder()
	GetPredictions(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestGetPredictions_ValidationError_ResponseIsJSON(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/predictions?symbol=!!!", nil)
	w := httptest.NewRecorder()
	GetPredictions(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %v, want 400", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %v, want application/json", ct)
	}
	var resp ResponseFailure
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("response status_code = %v, want %v", resp.StatusCode, http.StatusBadRequest)
	}
	if resp.Message == "" {
		t.Error("response message should not be empty")
	}
}

func TestGetPredictions_SymbolWithSpecialChars_Returns400(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/predictions?symbol=VI%40C", nil)
	w := httptest.NewRecorder()
	GetPredictions(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestGetPredictions_ZeroLimit_Returns400(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/predictions?limit=0", nil)
	w := httptest.NewRecorder()
	GetPredictions(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}
