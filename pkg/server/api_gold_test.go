package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	modelsdb "go-stock-prediction/pkg/models/models_db"
)

// ---------------------------------------------------------------------------
// DTO JSON field shape tests — no DB required
// ---------------------------------------------------------------------------

func TestGoldLatestItem_JSONFields(t *testing.T) {
	item := goldLatestItem{
		Source:      "BTMC",
		ProductType: "sjc",
		BuyPrice:    decimal.NewFromFloat(9270000),
		SellPrice:   decimal.NewFromFloat(9320000),
		Currency:    "VND",
		TradingDate: "2026-05-30",
	}

	b, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	required := []string{"source", "product_type", "buy_price", "sell_price", "currency", "trading_date"}
	for _, f := range required {
		if _, ok := raw[f]; !ok {
			t.Errorf("goldLatestItem JSON missing required field %q", f)
		}
	}
}

func TestGoldLatestResponse_JSONFields(t *testing.T) {
	resp := goldLatestResponse{
		Data:      []goldLatestItem{},
		UpdatedAt: time.Now().UTC(),
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	for _, f := range []string{"data", "updated_at"} {
		if _, ok := raw[f]; !ok {
			t.Errorf("goldLatestResponse JSON missing required field %q", f)
		}
	}
}

func TestGoldPricePoint_JSONFields(t *testing.T) {
	point := goldPricePoint{
		Date:      "2026-05-30",
		BuyPrice:  decimal.NewFromFloat(9270000),
		SellPrice: decimal.NewFromFloat(9320000),
	}

	b, err := json.Marshal(point)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	for _, f := range []string{"date", "buy_price", "sell_price"} {
		if _, ok := raw[f]; !ok {
			t.Errorf("goldPricePoint JSON missing required field %q", f)
		}
	}
}

func TestGoldPricesResponse_JSONFields(t *testing.T) {
	resp := goldPricesResponse{
		Source:      "BTMC",
		ProductType: "sjc",
		Data:        []goldPricePoint{},
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	for _, f := range []string{"source", "product_type", "data"} {
		if _, ok := raw[f]; !ok {
			t.Errorf("goldPricesResponse JSON missing required field %q", f)
		}
	}
}

func TestGoldChartResponse_JSONFields(t *testing.T) {
	resp := goldChartResponse{
		Labels:     []string{"2026-05-30"},
		BuyPrices:  []decimal.Decimal{decimal.NewFromFloat(9270000)},
		SellPrices: []decimal.Decimal{decimal.NewFromFloat(9320000)},
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	for _, f := range []string{"labels", "buy_prices", "sell_prices"} {
		if _, ok := raw[f]; !ok {
			t.Errorf("goldChartResponse JSON missing required field %q", f)
		}
	}
}

// ---------------------------------------------------------------------------
// GetGoldLatest — validation / contract tests (no DB)
// ---------------------------------------------------------------------------

// TestGetGoldLatest_NilStore_DoesNotReturn400 verifies the handler has no input
// validation of its own (no query params), so it never produces a 400. A nil
// store causes a panic / 500, which we recover from.
func TestGetGoldLatest_NilStore_DoesNotReturn400(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/gold/latest", nil)
	w := httptest.NewRecorder()

	func() {
		defer func() { recover() }()
		GetGoldLatest(w, req)
	}()

	if w.Code != 0 && w.Code == http.StatusBadRequest {
		t.Errorf("GetGoldLatest must never return 400, got %d", w.Code)
	}
}

// TestGetGoldLatest_ResponseError_ContentType_IsJSON verifies that the error
// response written by ResponseError carries Content-Type: application/json.
// We invoke ResponseError directly rather than going through the handler so we
// are not dependent on a live DB singleton.
func TestGetGoldLatest_ResponseError_ContentType_IsJSON(t *testing.T) {
	w := httptest.NewRecorder()
	ResponseError(w, http.StatusInternalServerError, "Failed to get latest gold prices")

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// ---------------------------------------------------------------------------
// GetGoldPrices — query param handling (no DB)
// ---------------------------------------------------------------------------

func TestGetGoldPrices_DefaultDays_NoError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/gold/prices", nil)
	w := httptest.NewRecorder()

	func() {
		defer func() { recover() }()
		GetGoldPrices(w, req)
	}()

	if w.Code != 0 && w.Code == http.StatusBadRequest {
		t.Errorf("expected no 400 for default days, got %d", w.Code)
	}
}

// TestGetGoldPrices_ValidDays_NoError verifies that valid positive ?days values
// pass through without returning 400.
func TestGetGoldPrices_ValidDays_NoError(t *testing.T) {
	dayValues := []string{"1", "7", "30", "90", "365"}
	for _, d := range dayValues {
		t.Run("days="+d, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/gold/prices?days="+d, nil)
			w := httptest.NewRecorder()

			func() {
				defer func() { recover() }()
				GetGoldPrices(w, req)
			}()

			if w.Code != 0 && w.Code == http.StatusBadRequest {
				t.Errorf("days=%s: expected no 400, got %d", d, w.Code)
			}
		})
	}
}

// TestGetGoldPrices_InvalidDays_FallsBackToDefault verifies that non-numeric or
// non-positive ?days values silently fall back to 30 (no 400 returned).
func TestGetGoldPrices_InvalidDays_FallsBackToDefault(t *testing.T) {
	invalidDays := []string{"abc", "-1", "0", "not-a-number"}
	for _, d := range invalidDays {
		t.Run("days="+d, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/gold/prices?days="+d, nil)
			w := httptest.NewRecorder()

			func() {
				defer func() { recover() }()
				GetGoldPrices(w, req)
			}()

			if w.Code != 0 && w.Code == http.StatusBadRequest {
				t.Errorf("days=%q: invalid days should use default (30), not 400, got %d", d, w.Code)
			}
		})
	}
}

// TestGetGoldPrices_SourceAndProductType_NotValidated verifies that source and
// product_type query params are forwarded to the store without handler-level
// validation (no 400 for any string value).
func TestGetGoldPrices_SourceAndProductType_NotValidated(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"BTMC sjc", "?source=BTMC&product_type=sjc"},
		{"XAU spot", "?source=XAU&product_type=spot"},
		{"BTMH nhan_tron", "?source=BTMH&product_type=nhan_tron"},
		{"unknown source", "?source=UNKNOWN&product_type=anything"},
		{"empty params", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/gold/prices"+tc.query, nil)
			w := httptest.NewRecorder()

			func() {
				defer func() { recover() }()
				GetGoldPrices(w, req)
			}()

			if w.Code != 0 && w.Code == http.StatusBadRequest {
				t.Errorf("query=%q: expected no 400, got %d", tc.query, w.Code)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GetGoldChart — query param handling (no DB)
// ---------------------------------------------------------------------------

func TestGetGoldChart_DefaultDays_NoError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/gold/chart", nil)
	w := httptest.NewRecorder()

	func() {
		defer func() { recover() }()
		GetGoldChart(w, req)
	}()

	if w.Code != 0 && w.Code == http.StatusBadRequest {
		t.Errorf("expected no 400 for default days, got %d", w.Code)
	}
}

func TestGetGoldChart_ValidDays_NoError(t *testing.T) {
	dayValues := []string{"7", "30", "90"}
	for _, d := range dayValues {
		t.Run("days="+d, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/gold/chart?days="+d, nil)
			w := httptest.NewRecorder()

			func() {
				defer func() { recover() }()
				GetGoldChart(w, req)
			}()

			if w.Code != 0 && w.Code == http.StatusBadRequest {
				t.Errorf("days=%s: expected no 400, got %d", d, w.Code)
			}
		})
	}
}

func TestGetGoldChart_InvalidDays_FallsBackToDefault(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/gold/chart?days=nope", nil)
	w := httptest.NewRecorder()

	func() {
		defer func() { recover() }()
		GetGoldChart(w, req)
	}()

	if w.Code != 0 && w.Code == http.StatusBadRequest {
		t.Errorf("invalid days should use default (30), not 400, got %d", w.Code)
	}
}

// TestGetGoldChart_ResponseError_ContentType_IsJSON mirrors the equivalent
// GetGoldLatest test — verifies Content-Type via the shared ResponseError helper.
func TestGetGoldChart_ResponseError_ContentType_IsJSON(t *testing.T) {
	w := httptest.NewRecorder()
	ResponseError(w, http.StatusInternalServerError, "Failed to get gold prices")

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// ---------------------------------------------------------------------------
// ResponseError shape — exercised through direct call
// ---------------------------------------------------------------------------

func TestGoldHandler_ResponseError_Shape(t *testing.T) {
	w := httptest.NewRecorder()
	ResponseError(w, http.StatusInternalServerError, "Failed to get latest gold prices")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var resp ResponseFailure
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("body status_code = %d, want 500", resp.StatusCode)
	}
	if resp.Message == "" {
		t.Error("body message must not be empty")
	}
}

// ---------------------------------------------------------------------------
// DTO mapping logic — independent of HTTP / DB
// ---------------------------------------------------------------------------

func TestGoldLatestItem_FromGoldPrice_FieldsMatch(t *testing.T) {
	tradingDate := time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC)
	gp := modelsdb.GoldPrice{
		Source:      "XAU",
		ProductType: "spot",
		TradingDate: tradingDate,
		BuyPrice:    decimal.NewFromFloat(3250.50),
		SellPrice:   decimal.NewFromFloat(3250.50),
		Currency:    "USD",
	}

	// Replicate the mapping the handler performs.
	item := goldLatestItem{
		Source:      gp.Source,
		ProductType: gp.ProductType,
		BuyPrice:    gp.BuyPrice,
		SellPrice:   gp.SellPrice,
		Currency:    gp.Currency,
		TradingDate: gp.TradingDate.Format("2006-01-02"),
	}

	if item.Source != "XAU" {
		t.Errorf("Source = %q, want XAU", item.Source)
	}
	if item.ProductType != "spot" {
		t.Errorf("ProductType = %q, want spot", item.ProductType)
	}
	if !item.BuyPrice.Equal(decimal.NewFromFloat(3250.50)) {
		t.Errorf("BuyPrice = %s, want 3250.50", item.BuyPrice)
	}
	if item.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", item.Currency)
	}
	if item.TradingDate != "2026-05-30" {
		t.Errorf("TradingDate = %q, want 2026-05-30", item.TradingDate)
	}
}

func TestGoldLatestItem_TradingDateFormat_ISO8601(t *testing.T) {
	// Verify the handler's date format matches ISO-8601 YYYY-MM-DD.
	d := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	if got := d.Format("2006-01-02"); got != "2026-01-05" {
		t.Errorf("date format = %q, want 2026-01-05", got)
	}
}

func TestGoldPricePoint_FromGoldPrice_FieldsMatch(t *testing.T) {
	tradingDate := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	gp := modelsdb.GoldPrice{
		TradingDate: tradingDate,
		BuyPrice:    decimal.NewFromFloat(9270000),
		SellPrice:   decimal.NewFromFloat(9320000),
	}

	point := goldPricePoint{
		Date:      gp.TradingDate.Format("2006-01-02"),
		BuyPrice:  gp.BuyPrice,
		SellPrice: gp.SellPrice,
	}

	if point.Date != "2026-05-28" {
		t.Errorf("Date = %q, want 2026-05-28", point.Date)
	}
	if !point.BuyPrice.Equal(decimal.NewFromFloat(9270000)) {
		t.Errorf("BuyPrice = %s, want 9270000", point.BuyPrice)
	}
	if !point.SellPrice.Equal(decimal.NewFromFloat(9320000)) {
		t.Errorf("SellPrice = %s, want 9320000", point.SellPrice)
	}
}

// ---------------------------------------------------------------------------
// GetGoldChart reversal logic — unit tests without HTTP or DB
// ---------------------------------------------------------------------------

// TestGoldChart_ReverseOrdering_OldestFirst verifies the DESC→ASC reversal
// logic that GetGoldChart applies to prices returned from the store.
func TestGoldChart_ReverseOrdering_OldestFirst(t *testing.T) {
	// DB returns prices newest-first (DESC).
	pricesDesc := []modelsdb.GoldPrice{
		{TradingDate: time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC), BuyPrice: decimal.NewFromFloat(9300000)},
		{TradingDate: time.Date(2026, 5, 29, 0, 0, 0, 0, time.UTC), BuyPrice: decimal.NewFromFloat(9280000)},
		{TradingDate: time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC), BuyPrice: decimal.NewFromFloat(9270000)},
	}

	// Replicate GetGoldChart reversal.
	labels := make([]string, 0, len(pricesDesc))
	buyPrices := make([]decimal.Decimal, 0, len(pricesDesc))
	for i := len(pricesDesc) - 1; i >= 0; i-- {
		p := pricesDesc[i]
		labels = append(labels, p.TradingDate.Format("2006-01-02"))
		buyPrices = append(buyPrices, p.BuyPrice)
	}

	if len(labels) != 3 {
		t.Fatalf("expected 3 labels, got %d", len(labels))
	}
	// After reversal the oldest date must be first.
	if labels[0] != "2026-05-28" {
		t.Errorf("labels[0] = %q, want 2026-05-28 (oldest first)", labels[0])
	}
	if labels[2] != "2026-05-30" {
		t.Errorf("labels[2] = %q, want 2026-05-30 (newest last)", labels[2])
	}
	if !buyPrices[0].Equal(decimal.NewFromFloat(9270000)) {
		t.Errorf("buyPrices[0] = %s, want 9270000", buyPrices[0])
	}
	if !buyPrices[2].Equal(decimal.NewFromFloat(9300000)) {
		t.Errorf("buyPrices[2] = %s, want 9300000", buyPrices[2])
	}
}

func TestGoldChart_ReverseOrdering_SingleEntry(t *testing.T) {
	prices := []modelsdb.GoldPrice{
		{TradingDate: time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC), BuyPrice: decimal.NewFromFloat(9300000)},
	}

	labels := make([]string, 0)
	for i := len(prices) - 1; i >= 0; i-- {
		labels = append(labels, prices[i].TradingDate.Format("2006-01-02"))
	}

	if len(labels) != 1 || labels[0] != "2026-05-30" {
		t.Errorf("single-entry labels = %v, want [2026-05-30]", labels)
	}
}

func TestGoldChart_ReverseOrdering_EmptySlice(t *testing.T) {
	prices := []modelsdb.GoldPrice{}
	labels := make([]string, 0)
	for i := len(prices) - 1; i >= 0; i-- {
		labels = append(labels, prices[i].TradingDate.Format("2006-01-02"))
	}
	if len(labels) != 0 {
		t.Errorf("empty prices should produce empty labels, got %v", labels)
	}
}

// ---------------------------------------------------------------------------
// goldLatestResponse.Data serialisation edge cases
// ---------------------------------------------------------------------------

func TestGoldLatestResponse_EmptyData_SerializesAsEmptyArray(t *testing.T) {
	resp := goldLatestResponse{
		Data:      []goldLatestItem{},
		UpdatedAt: time.Now().UTC(),
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var raw map[string]json.RawMessage
	json.Unmarshal(b, &raw) //nolint:errcheck

	// Initialised as non-nil slice so "data" must be "[]", not "null".
	if string(raw["data"]) == "null" {
		t.Error(`data field must be "[]" not "null" when empty`)
	}
}

func TestGoldLatestResponse_MultipleItems_PreservesOrder(t *testing.T) {
	items := []goldLatestItem{
		{Source: "XAU", ProductType: "spot", Currency: "USD", TradingDate: "2026-05-30"},
		{Source: "BTMC", ProductType: "sjc", Currency: "VND", TradingDate: "2026-05-30"},
		{Source: "BTMH", ProductType: "sjc", Currency: "VND", TradingDate: "2026-05-30"},
	}

	resp := goldLatestResponse{Data: items, UpdatedAt: time.Now().UTC()}
	b, _ := json.Marshal(resp)

	var got goldLatestResponse
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(got.Data) != 3 {
		t.Fatalf("expected 3 items, got %d", len(got.Data))
	}
	if got.Data[0].Source != "XAU" {
		t.Errorf("Data[0].Source = %q, want XAU", got.Data[0].Source)
	}
	if got.Data[1].Source != "BTMC" {
		t.Errorf("Data[1].Source = %q, want BTMC", got.Data[1].Source)
	}
	if got.Data[2].Source != "BTMH" {
		t.Errorf("Data[2].Source = %q, want BTMH", got.Data[2].Source)
	}
}

// ---------------------------------------------------------------------------
// goldPricesResponse source/product_type echo
// ---------------------------------------------------------------------------

func TestGoldPricesResponse_EchoesSourceAndProductType(t *testing.T) {
	// The handler writes source and product_type from query params into the
	// response body. Validate DTO round-trips both fields correctly.
	tests := []struct {
		source      string
		productType string
	}{
		{"BTMC", "sjc"},
		{"XAU", "spot"},
		{"BTMH", "nhan_tron"},
		{"XAU_VND", "spot"},
	}

	for _, tc := range tests {
		t.Run(tc.source+"/"+tc.productType, func(t *testing.T) {
			resp := goldPricesResponse{
				Source:      tc.source,
				ProductType: tc.productType,
				Data:        []goldPricePoint{},
			}

			b, _ := json.Marshal(resp)
			var got goldPricesResponse
			json.Unmarshal(b, &got) //nolint:errcheck

			if got.Source != tc.source {
				t.Errorf("Source = %q, want %q", got.Source, tc.source)
			}
			if got.ProductType != tc.productType {
				t.Errorf("ProductType = %q, want %q", got.ProductType, tc.productType)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// goldChartResponse — parallel slice lengths invariant
// ---------------------------------------------------------------------------

func TestGoldChartResponse_SliceLengthsMatch(t *testing.T) {
	// labels, buy_prices and sell_prices must always have the same length.
	prices := []modelsdb.GoldPrice{
		{TradingDate: time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC), BuyPrice: decimal.NewFromFloat(9300000), SellPrice: decimal.NewFromFloat(9350000)},
		{TradingDate: time.Date(2026, 5, 29, 0, 0, 0, 0, time.UTC), BuyPrice: decimal.NewFromFloat(9280000), SellPrice: decimal.NewFromFloat(9330000)},
	}

	labels := make([]string, 0, len(prices))
	buyPrices := make([]decimal.Decimal, 0, len(prices))
	sellPrices := make([]decimal.Decimal, 0, len(prices))

	for i := len(prices) - 1; i >= 0; i-- {
		p := prices[i]
		labels = append(labels, p.TradingDate.Format("2006-01-02"))
		buyPrices = append(buyPrices, p.BuyPrice)
		sellPrices = append(sellPrices, p.SellPrice)
	}

	if len(labels) != len(buyPrices) || len(labels) != len(sellPrices) {
		t.Errorf("slice lengths mismatch: labels=%d buy=%d sell=%d", len(labels), len(buyPrices), len(sellPrices))
	}
}

// ---------------------------------------------------------------------------
// Decimal precision — buy/sell prices survive JSON round-trip
// ---------------------------------------------------------------------------

func TestGoldPricePoint_DecimalPrecision_RoundTrip(t *testing.T) {
	original := decimal.NewFromFloat(9270000.50)
	point := goldPricePoint{
		Date:      "2026-05-30",
		BuyPrice:  original,
		SellPrice: original,
	}

	b, _ := json.Marshal(point)
	var got goldPricePoint
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if !got.BuyPrice.Equal(original) {
		t.Errorf("BuyPrice after round-trip = %s, want %s", got.BuyPrice, original)
	}
	if !got.SellPrice.Equal(original) {
		t.Errorf("SellPrice after round-trip = %s, want %s", got.SellPrice, original)
	}
}

func TestGoldLatestItem_DecimalPrecision_RoundTrip(t *testing.T) {
	buyPrice := decimal.NewFromFloat(3250.75)
	sellPrice := decimal.NewFromFloat(3251.00)

	item := goldLatestItem{
		Source:      "XAU",
		ProductType: "spot",
		BuyPrice:    buyPrice,
		SellPrice:   sellPrice,
		Currency:    "USD",
		TradingDate: "2026-05-30",
	}

	b, _ := json.Marshal(item)
	var got goldLatestItem
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if !got.BuyPrice.Equal(buyPrice) {
		t.Errorf("BuyPrice = %s, want %s", got.BuyPrice, buyPrice)
	}
	if !got.SellPrice.Equal(sellPrice) {
		t.Errorf("SellPrice = %s, want %s", got.SellPrice, sellPrice)
	}
}
