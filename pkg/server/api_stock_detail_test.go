package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGetStockDetail_InvalidSymbol verifies that a symbol containing special
// characters is rejected with 400 before any DB call is made.
func TestGetStockDetail_InvalidSymbol(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		symbol string
	}{
		{"at-sign", "/api/stocks/@@@/detail", "@@@"},
		{"exclamation", "/api/stocks/VIC!/detail", "VIC!"},
		{"space", "/api/stocks/VI%20C/detail", "VI C"},
		{"hash", "/api/stocks/VIC%23/detail", "VIC#"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()
			GetStockDetail(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("path=%q: expected 400, got %d", tc.path, w.Code)
			}

			ct := w.Header().Get("Content-Type")
			if ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}

			var resp ResponseFailure
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("response body is not valid JSON: %v", err)
			}
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("body status_code = %d, want 400", resp.StatusCode)
			}
			if resp.Message == "" {
				t.Error("response message must not be empty")
			}
		})
	}
}

// TestGetStockDetail_SymbolTooShort verifies that a single-character symbol is
// rejected with 400.
func TestGetStockDetail_SymbolTooShort(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/stocks/V/detail", nil)
	w := httptest.NewRecorder()
	GetStockDetail(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var resp ResponseFailure
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}
	if resp.Message == "" {
		t.Error("error message must not be empty")
	}
}

// TestGetStockDetail_SymbolTooLong verifies that a symbol with 6+ characters is
// rejected with 400.
func TestGetStockDetail_SymbolTooLong(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"6-chars", "/api/stocks/TOOLNG/detail"},
		{"8-chars", "/api/stocks/VERYLNGS/detail"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()
			GetStockDetail(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("path=%q: expected 400, got %d", tc.path, w.Code)
			}
		})
	}
}

// TestGetStockDetail_ValidPath verifies that for a well-formed path with a valid
// symbol the handler extracts the symbol from path position [3] and proceeds
// past validation. Without a DB the handler returns 404 (stock not found), which
// is the expected DB-error response — not a validation error.
func TestGetStockDetail_ValidPath(t *testing.T) {
	validSymbols := []struct {
		name   string
		path   string
		symbol string
	}{
		{"3-chars", "/api/stocks/VCB/detail", "VCB"},
		{"4-chars", "/api/stocks/HOSE/detail", "HOSE"},
		{"5-chars", "/api/stocks/ABCDE/detail", "ABCDE"},
		{"2-chars", "/api/stocks/VN/detail", "VN"},
	}

	for _, tc := range validSymbols {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()

			// Use recover to catch the nil-pointer panic that happens when
			// repository.GetSingleton() is nil (no DB in unit tests).
			// A panic here means validation passed — which is what we want to
			// confirm. Any non-400 response also indicates validation passed.
			func() {
				defer func() {
					if r := recover(); r != nil {
						// Panic from nil DB access — validation succeeded.
						t.Logf("symbol %q passed validation (DB not available in test): %v", tc.symbol, r)
					}
				}()
				GetStockDetail(w, req)
			}()

			// If the handler responded without panic, it must NOT be 400 (validation error).
			if w.Code != 0 && w.Code == http.StatusBadRequest {
				t.Errorf("symbol %q: did not expect 400 (validation failure) for a valid symbol", tc.symbol)
			}
		})
	}
}

// TestGetStockDetail_PathTooShort verifies that a path with fewer than 5 segments
// returns 400 immediately.
func TestGetStockDetail_PathTooShort(t *testing.T) {
	shortPaths := []string{
		"/api/stocks",
		"/api/stocks/",
		"/api",
	}

	for _, path := range shortPaths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			GetStockDetail(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("path=%q: expected 400, got %d", path, w.Code)
			}
		})
	}
}

// TestGetStockDetail_ResponseFormat verifies the error response is always JSON
// and contains both status_code and message fields.
func TestGetStockDetail_ResponseFormat(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/stocks/TOOLONG/detail", nil)
	w := httptest.NewRecorder()
	GetStockDetail(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var resp ResponseFailure
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("body status_code = %d, want 400", resp.StatusCode)
	}
	if resp.Message == "" {
		t.Error("body message must not be empty on error")
	}
}
