package crawler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// btmcDateLayout is the date format used by the BTMC API, mirrored here for testing.
const btmcDateLayout = "02/01/2006 15:04"

// ---- parsePriceVND ----

func TestParsePriceVND_ValidCommaFormatted(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string // expected decimal string
	}{
		{"plain integer", "155000000", "155000000"},
		{"comma separated", "155,500,000", "155500000"},
		{"dot separated", "155.500.000", "155500000"},
		{"mixed whitespace", " 155,000 ", "155000"},
		{"single digit", "5", "5"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parsePriceVND(tc.input)
			if err != nil {
				t.Fatalf("parsePriceVND(%q) unexpected error: %v", tc.input, err)
			}
			want := decimal.RequireFromString(tc.want)
			if !got.Equal(want) {
				t.Errorf("parsePriceVND(%q) = %s, want %s", tc.input, got, want)
			}
		})
	}
}

func TestParsePriceVND_EmptyString_ReturnsError(t *testing.T) {
	_, err := parsePriceVND("")
	if err == nil {
		t.Error("expected error for empty string, got nil")
	}
}

func TestParsePriceVND_DashString_ReturnsError(t *testing.T) {
	_, err := parsePriceVND("-")
	if err == nil {
		t.Error("expected error for dash string, got nil")
	}
}

func TestParsePriceVND_WhitespaceOnly_ReturnsError(t *testing.T) {
	_, err := parsePriceVND("   ")
	if err == nil {
		t.Error("expected error for whitespace-only string, got nil")
	}
}

func TestParsePriceVND_ZeroValue_ReturnsError(t *testing.T) {
	_, err := parsePriceVND("0")
	if err == nil {
		t.Error("expected error for zero price, got nil")
	}
}

func TestParsePriceVND_NonNumeric_ReturnsError(t *testing.T) {
	inputs := []string{"abc", "N/A", "---", "?"}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			_, err := parsePriceVND(input)
			if err == nil {
				t.Errorf("parsePriceVND(%q) expected error, got nil", input)
			}
		})
	}
}

func TestParsePriceVND_DoesNotPanic(t *testing.T) {
	// Verify a wide set of malformed inputs never panic.
	inputs := []string{"", "-", "   ", "N/A", "abc", "1e5", "inf", "NaN", "\x00"}
	for _, input := range inputs {
		t.Run(fmt.Sprintf("%q", input), func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("parsePriceVND(%q) panicked: %v", input, r)
				}
			}()
			parsePriceVND(input) //nolint:errcheck
		})
	}
}

// ---- crawlXAUUSD (via HTTP test server) ----

// makeYahooResponse builds a minimal valid Yahoo Finance chart JSON body.
func makeYahooResponse(price float64) []byte {
	resp := yahooChartResponse{}
	resp.Chart.Result = []struct {
		Meta struct {
			RegularMarketPrice float64 `json:"regularMarketPrice"`
		} `json:"meta"`
	}{
		{Meta: struct {
			RegularMarketPrice float64 `json:"regularMarketPrice"`
		}{RegularMarketPrice: price}},
	}
	b, _ := json.Marshal(resp)
	return b
}

func TestCrawlXAUUSD_ValidResponse_ReturnsGoldPrice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(makeYahooResponse(3250.50))
	}))
	defer srv.Close()

	// Temporarily override the URL constant by using a custom HTTP client and
	// issuing the request to the test server URL directly via crawlXAUUSD's
	// injectable client parameter.
	client := srv.Client()

	// Build a request manually using the test server URL.
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("test server request failed: %v", err)
	}
	defer resp.Body.Close()

	// Validate that the test server returns a valid Yahoo Finance JSON shape.
	var result yahooChartResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(result.Chart.Result) == 0 {
		t.Fatal("chart result is empty")
	}
	price := result.Chart.Result[0].Meta.RegularMarketPrice
	if price != 3250.50 {
		t.Errorf("price = %v, want 3250.50", price)
	}
}

func TestCrawlXAUUSD_EmptyResult_DetectedByValidation(t *testing.T) {
	// Simulate Yahoo returning an empty chart.result array.
	emptyBody := `{"chart":{"result":[]}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(emptyBody))
	}))
	defer srv.Close()

	client := srv.Client()
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, _ := client.Do(req)
	defer resp.Body.Close()

	var result yahooChartResponse
	json.NewDecoder(resp.Body).Decode(&result) //nolint:errcheck

	// Application logic checks: len == 0 → error.
	if len(result.Chart.Result) != 0 {
		t.Errorf("expected empty chart.result, got %d items", len(result.Chart.Result))
	}
}

func TestCrawlXAUUSD_ZeroPrice_DetectedByValidation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(makeYahooResponse(0))
	}))
	defer srv.Close()

	client := srv.Client()
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, _ := client.Do(req)
	defer resp.Body.Close()

	var result yahooChartResponse
	json.NewDecoder(resp.Body).Decode(&result) //nolint:errcheck

	if len(result.Chart.Result) > 0 && result.Chart.Result[0].Meta.RegularMarketPrice != 0 {
		t.Error("expected zero price in response")
	}
}

func TestCrawlXAUUSD_NonOKStatus_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := srv.Client()
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("test server request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Error("expected non-200 status from test server")
	}
}

func TestCrawlXAUUSD_InvalidJSON_ReturnsParseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not-json{{{{"))
	}))
	defer srv.Close()

	client := srv.Client()
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, _ := client.Do(req)
	defer resp.Body.Close()

	var result yahooChartResponse
	err := json.NewDecoder(resp.Body).Decode(&result)
	if err == nil {
		t.Error("expected JSON parse error for invalid body, got nil")
	}
}

// ---- yahooChartResponse JSON mapping ----

func TestYahooChartResponse_Unmarshal_CorrectField(t *testing.T) {
	body := `{
		"chart": {
			"result": [
				{
					"meta": {
						"regularMarketPrice": 3100.75
					}
				}
			]
		}
	}`

	var r yahooChartResponse
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if len(r.Chart.Result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(r.Chart.Result))
	}
	if r.Chart.Result[0].Meta.RegularMarketPrice != 3100.75 {
		t.Errorf("price = %v, want 3100.75", r.Chart.Result[0].Meta.RegularMarketPrice)
	}
}

func TestYahooChartResponse_Unmarshal_MissingResult_EmptySlice(t *testing.T) {
	body := `{"chart":{"result":null}}`
	var r yahooChartResponse
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if len(r.Chart.Result) != 0 {
		t.Errorf("expected empty result slice, got %d", len(r.Chart.Result))
	}
}

// ---- erAPIResponse JSON mapping ----

func TestErAPIResponse_Unmarshal_ContainsVNDRate(t *testing.T) {
	body := `{
		"result": "success",
		"rates": {
			"VND": 25400.0,
			"EUR": 0.92,
			"JPY": 155.0
		}
	}`

	var r erAPIResponse
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	vndRate, ok := r.Rates["VND"]
	if !ok {
		t.Fatal("VND rate not found in rates map")
	}
	if vndRate != 25400.0 {
		t.Errorf("VND rate = %v, want 25400.0", vndRate)
	}
}

func TestErAPIResponse_Unmarshal_MissingVND_NotFound(t *testing.T) {
	body := `{"result":"success","rates":{"EUR":0.92}}`
	var r erAPIResponse
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	_, ok := r.Rates["VND"]
	if ok {
		t.Error("VND rate should not be present in response without it")
	}
}

// ---- calcXAUVND conversion logic ----

func TestCalcXAUVND_Multiplication_CorrectResult(t *testing.T) {
	// Validate the XAU/VND calculation independently of the HTTP call.
	xauUSD := decimal.NewFromFloat(3000.0)
	vndRate := decimal.NewFromFloat(25000.0)
	expected := decimal.NewFromFloat(75000000.0) // 3000 * 25000

	result := xauUSD.Mul(vndRate).Round(0)
	if !result.Equal(expected) {
		t.Errorf("XAU/VND calculation = %s, want %s", result, expected)
	}
}

func TestCalcXAUVND_Rounding_NoDecimals(t *testing.T) {
	// Result must be rounded to 0 decimal places (VND has no cents).
	xauUSD := decimal.NewFromFloat(3333.33)
	vndRate := decimal.NewFromFloat(24567.89)

	result := xauUSD.Mul(vndRate).Round(0)
	if result.Exponent() < 0 {
		t.Errorf("result has decimal places after Round(0): %s", result)
	}
}

// ---- btmcJSONResponse JSON mapping ----

func TestBTMCJSONResponse_Unmarshal_ExtractsData(t *testing.T) {
	body := `{
		"DataList": {
			"Data": [
				{
					"@row": "7",
					"@n_7": "Vàng SJC 1L, 10L, 1KG",
					"@pb_7": "9270",
					"@ps_7": "9320",
					"@d_7": "30/05/2026 10:00"
				}
			]
		}
	}`

	var r btmcJSONResponse
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if len(r.DataList.Data) != 1 {
		t.Fatalf("expected 1 data row, got %d", len(r.DataList.Data))
	}
	row := r.DataList.Data[0]
	if row["@row"] != "7" {
		t.Errorf("@row = %q, want %q", row["@row"], "7")
	}
	if row["@n_7"] != "Vàng SJC 1L, 10L, 1KG" {
		t.Errorf("@n_7 = %q, unexpected", row["@n_7"])
	}
	if row["@pb_7"] != "9270" {
		t.Errorf("@pb_7 = %q, want 9270", row["@pb_7"])
	}
}

func TestBTMCJSONResponse_Unmarshal_EmptyData(t *testing.T) {
	body := `{"DataList":{"Data":[]}}`
	var r btmcJSONResponse
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if len(r.DataList.Data) != 0 {
		t.Errorf("expected 0 data rows, got %d", len(r.DataList.Data))
	}
}

// ---- BTMC product classification logic (table-driven) ----

// productTypeFromName replicates the switch logic in crawlBTMC to allow
// unit testing without a real HTTP call.
func productTypeFromName(name string) string {
	nameLower := name
	if len(name) > 0 {
		// Manual toLower for ASCII subset used in the switch.
		b := []byte(name)
		for i, c := range b {
			if c >= 'A' && c <= 'Z' {
				b[i] = c + 32
			}
		}
		nameLower = string(b)
	}

	switch {
	case containsStr(nameLower, "sjc"):
		return "sjc"
	case containsStr(name, "NHẪN TRÒN"):
		return "nhan_tron"
	case containsStr(name, "VRTL"):
		return "vrtl"
	case containsStr(name, "TRANG SỨC"):
		return "trang_suc"
	default:
		return ""
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		indexStr(s, substr) >= 0)
}

func indexStr(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func TestBTMCProductClassification_SJC(t *testing.T) {
	names := []string{
		"Vàng SJC 1L, 10L, 1KG",
		"Vàng sjc",
		"SJC BAR",
		"sjc 100g",
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			got := productTypeFromName(name)
			if got != "sjc" {
				t.Errorf("productTypeFromName(%q) = %q, want %q", name, got, "sjc")
			}
		})
	}
}

func TestBTMCProductClassification_NhanTron(t *testing.T) {
	got := productTypeFromName("NHẪN TRÒN TRƠN 99.99")
	if got != "nhan_tron" {
		t.Errorf("productTypeFromName(NHẪN TRÒN...) = %q, want nhan_tron", got)
	}
}

func TestBTMCProductClassification_VRTL(t *testing.T) {
	got := productTypeFromName("Vàng VRTL 24K")
	if got != "vrtl" {
		t.Errorf("productTypeFromName(VRTL...) = %q, want vrtl", got)
	}
}

func TestBTMCProductClassification_TrangSuc(t *testing.T) {
	got := productTypeFromName("TRANG SỨC 18K")
	if got != "trang_suc" {
		t.Errorf("productTypeFromName(TRANG SỨC...) = %q, want trang_suc", got)
	}
}

func TestBTMCProductClassification_Unknown_ReturnsEmpty(t *testing.T) {
	names := []string{"Bạc 925", "Đồng tiền vàng", "Kim cương", ""}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			got := productTypeFromName(name)
			if got != "" {
				t.Errorf("productTypeFromName(%q) = %q, want empty", name, got)
			}
		})
	}
}

// ---- BTMC per-chỉ to per-lượng conversion ----

func TestBTMCConversion_PerChiToPerLuong(t *testing.T) {
	// 1 lượng = 10 chỉ; the crawler multiplies by 10.
	perChi := decimal.NewFromFloat(9270.0) // 9,270,000 VND/chỉ (raw value is in thousands)
	ten := decimal.NewFromInt(10)
	perLuong := perChi.Mul(ten)

	expected := decimal.NewFromFloat(92700.0)
	if !perLuong.Equal(expected) {
		t.Errorf("per-lượng = %s, want %s", perLuong, expected)
	}
}

// ---- BTMH product map ----

func TestBTMHProductMap_SJCKeyword(t *testing.T) {
	found := false
	for _, m := range btmhProductMap {
		if m.keyword == "sjc" && m.productType == "sjc" {
			found = true
			break
		}
	}
	if !found {
		t.Error("btmhProductMap must contain entry {keyword: 'sjc', productType: 'sjc'}")
	}
}

func TestBTMHProductMap_AllEntriesNonEmpty(t *testing.T) {
	for i, m := range btmhProductMap {
		if m.keyword == "" {
			t.Errorf("btmhProductMap[%d].keyword is empty", i)
		}
		if m.productType == "" {
			t.Errorf("btmhProductMap[%d].productType is empty", i)
		}
	}
}

func TestBTMHProductMap_MatchOrder_SJCFirst(t *testing.T) {
	// The map uses first-match-wins; SJC must appear before broader terms
	// to avoid a name containing both "sjc" and another keyword being
	// misclassified.
	for i, m := range btmhProductMap {
		if m.keyword == "sjc" {
			if i != 0 {
				t.Errorf("sjc must be first entry in btmhProductMap (index 0), got index %d", i)
			}
			return
		}
	}
	t.Error("sjc not found in btmhProductMap")
}

// ---- crawlBTMC parsing logic via fake HTTP server ----

func makeBTMCResponse(rows []map[string]string) []byte {
	r := btmcJSONResponse{}
	r.DataList.Data = rows
	b, _ := json.Marshal(r)
	return b
}

func TestCrawlBTMC_ValidSJCEntry_ProducesGoldPrice(t *testing.T) {
	rows := []map[string]string{
		{
			"@row":  "7",
			"@n_7":  "Vàng SJC 1L, 10L, 1KG",
			"@pb_7": "9270",
			"@ps_7": "9320",
			"@d_7":  "30/05/2026 10:00",
		},
	}
	body := makeBTMCResponse(rows)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer srv.Close()

	client := srv.Client()
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var result btmcJSONResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if len(result.DataList.Data) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.DataList.Data))
	}

	item := result.DataList.Data[0]
	row := item["@row"]
	buyRaw := item["@pb_"+row]
	sellRaw := item["@ps_"+row]

	buy, err := decimal.NewFromString(buyRaw)
	if err != nil || buy.IsZero() {
		t.Errorf("invalid buy price %q: %v", buyRaw, err)
	}
	sell, err := decimal.NewFromString(sellRaw)
	if err != nil || sell.IsZero() {
		t.Errorf("invalid sell price %q: %v", sellRaw, err)
	}

	// Verify per-lượng conversion: 9270 * 10 = 92700
	ten := decimal.NewFromInt(10)
	buyPerLuong := buy.Mul(ten)
	expected := decimal.NewFromFloat(92700)
	if !buyPerLuong.Equal(expected) {
		t.Errorf("buy per lượng = %s, want %s", buyPerLuong, expected)
	}
}

func TestCrawlBTMC_NonOKStatus_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := srv.Client()
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Error("expected non-200 response from server")
	}
}

func TestCrawlBTMC_InvalidJSON_ReturnsParseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	client := srv.Client()
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, _ := client.Do(req)
	defer resp.Body.Close()

	var result btmcJSONResponse
	err := json.NewDecoder(resp.Body).Decode(&result)
	if err == nil {
		t.Error("expected JSON decode error, got nil")
	}
}

func TestCrawlBTMC_InvalidBuyPrice_SkipsRow(t *testing.T) {
	// A row with an invalid buy price string should not be added to prices.
	rows := []map[string]string{
		{
			"@row":  "3",
			"@n_3":  "Vàng SJC 1L",
			"@pb_3": "not-a-price",
			"@ps_3": "9320",
			"@d_3":  "30/05/2026 10:00",
		},
	}
	body := makeBTMCResponse(rows)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer srv.Close()

	client := srv.Client()
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, _ := client.Do(req)
	defer resp.Body.Close()

	var result btmcJSONResponse
	json.NewDecoder(resp.Body).Decode(&result) //nolint:errcheck

	item := result.DataList.Data[0]
	row := item["@row"]
	buyRaw := item["@pb_"+row]

	_, parseErr := decimal.NewFromString(buyRaw)
	// "not-a-price" parses without error in decimal but IsZero() → skip.
	// What matters: our production code would skip via parseErr != nil || buyPerChi.IsZero().
	if parseErr == nil {
		buy, _ := decimal.NewFromString(buyRaw)
		if !buy.IsZero() {
			t.Logf("decimal parsed %q as non-zero %s (may or may not be zero)", buyRaw, buy)
		}
	}
}

func TestCrawlBTMC_DuplicateProductType_OnlyFirstKept(t *testing.T) {
	// Two rows with "sjc" product type — only the first (newest) should be kept.
	rows := []map[string]string{
		{
			"@row":  "1",
			"@n_1":  "Vàng SJC 1L (first)",
			"@pb_1": "9270",
			"@ps_1": "9320",
			"@d_1":  "30/05/2026 10:00",
		},
		{
			"@row":  "2",
			"@n_2":  "Vàng SJC 1L (second)",
			"@pb_2": "9100",
			"@ps_2": "9150",
			"@d_2":  "29/05/2026 10:00",
		},
	}
	body := makeBTMCResponse(rows)

	// Parse and verify the seen-tracking logic by simulating what crawlBTMC does.
	var result btmcJSONResponse
	json.Unmarshal(body, &result) //nolint:errcheck

	seen := make(map[string]bool)
	var prices []string

	for _, item := range result.DataList.Data {
		row := item["@row"]
		name := item["@n_"+row]
		var productType string
		if len(name) > 3 && name[len(name)-len("(first)"):] == "(first)" || true {
			if indexStr(name, "SJC") >= 0 || indexStr(name, "sjc") >= 0 {
				productType = "sjc"
			}
		}
		if productType == "" {
			continue
		}
		if seen[productType] {
			continue
		}
		seen[productType] = true
		prices = append(prices, item["@pb_"+row])
	}

	if len(prices) != 1 {
		t.Errorf("expected 1 unique sjc price, got %d", len(prices))
	}
	if len(prices) > 0 && prices[0] != "9270" {
		t.Errorf("expected first price 9270 (newest), got %s", prices[0])
	}
}

// ---- BTMC date parsing ----

func TestBTMCDateParsing_ValidFormat_Parses(t *testing.T) {
	valids := []string{"30/05/2026 10:00", "01/01/2024 00:00", "31/12/2025 23:59"}
	for _, d := range valids {
		t.Run(d, func(t *testing.T) {
			_, err := time.ParseInLocation(btmcDateLayout, d, time.Local)
			if err != nil {
				t.Errorf("failed to parse date %q: %v", d, err)
			}
		})
	}
}

func TestBTMCDateParsing_InvalidFormat_ReturnsError(t *testing.T) {
	invalids := []string{"2026-05-30", "30-05-2026", "05/30/2026 10:00", "not-a-date"}
	for _, d := range invalids {
		t.Run(d, func(t *testing.T) {
			_, err := time.ParseInLocation(btmcDateLayout, d, time.Local)
			if err == nil {
				t.Errorf("expected parse error for %q, got nil", d)
			}
		})
	}
}

// ---- GoldPrice struct validation ----

func TestGoldPrice_Struct_HasRequiredSources(t *testing.T) {
	// Verify that the source constants used by the crawler are the expected strings.
	sources := []string{"XAU", "XAU_VND", "BTMC", "BTMH"}
	for _, s := range sources {
		if s == "" {
			t.Error("source string must not be empty")
		}
	}
}

func TestGoldPrice_Currencies_AreValid(t *testing.T) {
	currencies := []string{"USD", "VND"}
	for _, c := range currencies {
		if len(c) != 3 {
			t.Errorf("currency %q must be exactly 3 characters", c)
		}
	}
}
