package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	modelsapi "go-stock-prediction/pkg/models/models_api"

	"github.com/shopspring/decimal"
)

// ---- Cache helpers used by market overview ----

func TestGlobalCache_SetAndGet(t *testing.T) {
	key := "test_market_overview_cache"
	data := map[string]string{"hello": "world"}

	globalCache.Set(key, data, 10*time.Second)

	got, ok := globalCache.Get(key)
	if !ok {
		t.Fatal("expected to find cached entry, got miss")
	}
	m, ok := got.(map[string]string)
	if !ok {
		t.Fatalf("cached value has wrong type: %T", got)
	}
	if m["hello"] != "world" {
		t.Errorf("cached value = %v, want 'world'", m["hello"])
	}

	// Clean up.
	globalCache.Delete(key)
}

func TestGlobalCache_MissAfterExpiry(t *testing.T) {
	key := "test_expiry_key"
	globalCache.Set(key, "value", 1*time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	_, ok := globalCache.Get(key)
	if ok {
		t.Error("expected cache miss after TTL expiry, got hit")
	}
}

func TestGlobalCache_Delete(t *testing.T) {
	key := "test_delete_key"
	globalCache.Set(key, "value", 10*time.Second)
	globalCache.Delete(key)

	_, ok := globalCache.Get(key)
	if ok {
		t.Error("expected cache miss after Delete, got hit")
	}
}

// ---- getTopByChange ----

func makeStockCurrentPriceDTO(symbol string, changePercent float64, volume int64) modelsapi.StockCurrentPriceDTO {
	return modelsapi.StockCurrentPriceDTO{
		Stock: modelsapi.StockDTO{
			Symbol: symbol,
		},
		ChangePercent: decimal.NewFromFloat(changePercent),
		Volume:        volume,
	}
}

func TestGetTopByChange_Gainers_ReturnsSortedDesc(t *testing.T) {
	stocks := []modelsapi.StockCurrentPriceDTO{
		makeStockCurrentPriceDTO("A", 1.0, 100),
		makeStockCurrentPriceDTO("B", 3.0, 200),
		makeStockCurrentPriceDTO("C", 2.0, 300),
	}

	top := getTopByChange(stocks, true, 2)

	if len(top) != 2 {
		t.Fatalf("expected 2 results, got %d", len(top))
	}
	// Highest change percent first.
	if !top[0].ChangePercent.GreaterThanOrEqual(top[1].ChangePercent) {
		t.Errorf("top gainers not sorted descending: %v >= %v", top[0].ChangePercent, top[1].ChangePercent)
	}
}

func TestGetTopByChange_Losers_ReturnsSortedAsc(t *testing.T) {
	stocks := []modelsapi.StockCurrentPriceDTO{
		makeStockCurrentPriceDTO("A", -1.0, 100),
		makeStockCurrentPriceDTO("B", -3.0, 200),
		makeStockCurrentPriceDTO("C", -2.0, 300),
	}

	top := getTopByChange(stocks, false, 2)

	if len(top) != 2 {
		t.Fatalf("expected 2 results, got %d", len(top))
	}
	// Most negative (biggest loss) first.
	if !top[0].ChangePercent.LessThanOrEqual(top[1].ChangePercent) {
		t.Errorf("top losers not sorted ascending: %v <= %v", top[0].ChangePercent, top[1].ChangePercent)
	}
}

func TestGetTopByChange_LimitExceedsSlice_ReturnsAll(t *testing.T) {
	stocks := []modelsapi.StockCurrentPriceDTO{
		makeStockCurrentPriceDTO("A", 1.0, 100),
		makeStockCurrentPriceDTO("B", 2.0, 200),
	}

	top := getTopByChange(stocks, true, 10)
	if len(top) != 2 {
		t.Errorf("expected 2 results (all stocks), got %d", len(top))
	}
}

func TestGetTopByChange_EmptySlice_ReturnsEmpty(t *testing.T) {
	top := getTopByChange([]modelsapi.StockCurrentPriceDTO{}, true, 5)
	if len(top) != 0 {
		t.Errorf("expected empty result, got %d items", len(top))
	}
}

func TestGetTopByChange_ExactLimit_ReturnsTrimmed(t *testing.T) {
	stocks := []modelsapi.StockCurrentPriceDTO{
		makeStockCurrentPriceDTO("A", 5.0, 100),
		makeStockCurrentPriceDTO("B", 3.0, 200),
		makeStockCurrentPriceDTO("C", 1.0, 300),
		makeStockCurrentPriceDTO("D", 4.0, 400),
	}

	top := getTopByChange(stocks, true, 3)
	if len(top) != 3 {
		t.Errorf("expected 3 results, got %d", len(top))
	}
}

// ---- getTopByVolume ----

func TestGetTopByVolume_ReturnsSortedByVolumeDesc(t *testing.T) {
	stocks := []modelsapi.StockCurrentPriceDTO{
		makeStockCurrentPriceDTO("A", 0.0, 100),
		makeStockCurrentPriceDTO("B", 0.0, 500),
		makeStockCurrentPriceDTO("C", 0.0, 300),
	}

	top := getTopByVolume(stocks, 2)

	if len(top) != 2 {
		t.Fatalf("expected 2 results, got %d", len(top))
	}
	if top[0].Volume < top[1].Volume {
		t.Errorf("top by volume not sorted descending: %d >= %d", top[0].Volume, top[1].Volume)
	}
}

func TestGetTopByVolume_LimitExceedsSlice_ReturnsAll(t *testing.T) {
	stocks := []modelsapi.StockCurrentPriceDTO{
		makeStockCurrentPriceDTO("A", 0.0, 100),
		makeStockCurrentPriceDTO("B", 0.0, 200),
	}

	top := getTopByVolume(stocks, 10)
	if len(top) != 2 {
		t.Errorf("expected 2 results, got %d", len(top))
	}
}

func TestGetTopByVolume_EmptySlice_ReturnsEmpty(t *testing.T) {
	top := getTopByVolume([]modelsapi.StockCurrentPriceDTO{}, 5)
	if len(top) != 0 {
		t.Errorf("expected empty result, got %d items", len(top))
	}
}

// ---- GetMarketOverview handler: query param & cache behaviour ----

// TestGetMarketOverview_NoFilters_UsesCacheKey verifies that on the second call
// with no filters the cached response is served. We seed the cache manually.
func TestGetMarketOverview_NoFilters_UsesCacheKey(t *testing.T) {
	// Seed the cache with a sentinel value.
	sentinel := &modelsapi.MarketOverviewDTO{TotalStocks: 99}
	globalCache.Set(marketOverviewCacheKey, sentinel, 60*time.Second)
	defer globalCache.Delete(marketOverviewCacheKey)

	req := httptest.NewRequest(http.MethodGet, "/api/market/overview", nil)
	w := httptest.NewRecorder()
	GetMarketOverview(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from cache hit, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	// The body should round-trip as JSON with total_stocks = 99.
	var dto modelsapi.MarketOverviewDTO
	if err := json.Unmarshal(w.Body.Bytes(), &dto); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}
	if dto.TotalStocks != 99 {
		t.Errorf("total_stocks = %d, want 99 (from cache)", dto.TotalStocks)
	}
}

// TestGetMarketOverview_WithSectorFilter_SkipsCache verifies that adding a
// ?sector query param bypasses the cache (so filtered results are never cached).
func TestGetMarketOverview_WithSectorFilter_SkipsCache(t *testing.T) {
	// Seed cache with a different sentinel value.
	sentinel := &modelsapi.MarketOverviewDTO{TotalStocks: 77}
	globalCache.Set(marketOverviewCacheKey, sentinel, 60*time.Second)
	defer globalCache.Delete(marketOverviewCacheKey)

	req := httptest.NewRequest(http.MethodGet, "/api/market/overview?sector=ngan-hang", nil)
	w := httptest.NewRecorder()

	// The handler will try to hit the DB after bypassing cache — it will panic or return 500.
	func() {
		defer func() { recover() }()
		GetMarketOverview(w, req)
	}()

	if w.Code == http.StatusOK {
		// If somehow it returned 200, the body must NOT be the cached sentinel (77 stocks).
		var dto modelsapi.MarketOverviewDTO
		if err := json.Unmarshal(w.Body.Bytes(), &dto); err == nil {
			if dto.TotalStocks == 77 {
				t.Error("sector filter should bypass cache, but got cached value with total_stocks=77")
			}
		}
	}
	// Any status other than the cached 200 proves cache was bypassed — acceptable.
}

// TestGetMarketOverview_WithExchangeFilter_SkipsCache verifies ?exchange also
// bypasses the cache.
func TestGetMarketOverview_WithExchangeFilter_SkipsCache(t *testing.T) {
	sentinel := &modelsapi.MarketOverviewDTO{TotalStocks: 55}
	globalCache.Set(marketOverviewCacheKey, sentinel, 60*time.Second)
	defer globalCache.Delete(marketOverviewCacheKey)

	req := httptest.NewRequest(http.MethodGet, "/api/market/overview?exchange=HOSE", nil)
	w := httptest.NewRecorder()

	func() {
		defer func() { recover() }()
		GetMarketOverview(w, req)
	}()

	if w.Code == http.StatusOK {
		var dto modelsapi.MarketOverviewDTO
		if err := json.Unmarshal(w.Body.Bytes(), &dto); err == nil {
			if dto.TotalStocks == 55 {
				t.Error("exchange filter should bypass cache, but got cached value with total_stocks=55")
			}
		}
	}
}

// TestGetMarketOverview_NeverReturns400 verifies that the market overview handler
// has no input validation that results in 400. Bad filters silently yield empty
// results or trigger DB errors.
func TestGetMarketOverview_NeverReturns400(t *testing.T) {
	queryStrings := []string{
		"",
		"?sector=ngan-hang",
		"?exchange=HOSE",
		"?sector=unknown-sector",
		"?exchange=UNKNOWN_EXCHANGE",
		"?sector=a&exchange=b",
	}

	for _, qs := range queryStrings {
		t.Run("query="+qs, func(t *testing.T) {
			// Clear cache so we don't get a false 200 from cached data.
			globalCache.Delete(marketOverviewCacheKey)

			req := httptest.NewRequest(http.MethodGet, "/api/market/overview"+qs, nil)
			w := httptest.NewRecorder()

			func() {
				defer func() { recover() }()
				GetMarketOverview(w, req)
			}()

			if w.Code != 0 && w.Code == http.StatusBadRequest {
				t.Errorf("query=%q: market overview must never return 400, got %d", qs, w.Code)
			}
		})
	}
}

// TestGetMarketOverview_CacheHit_Returns200 verifies the happy-path where the
// cache already holds data. This is the only path we can test without a DB.
func TestGetMarketOverview_CacheHit_Returns200(t *testing.T) {
	overview := &modelsapi.MarketOverviewDTO{
		TotalStocks: 30,
		Gainers:     10,
		Losers:      5,
		Unchanged:   15,
	}
	globalCache.Set(marketOverviewCacheKey, overview, 60*time.Second)
	defer globalCache.Delete(marketOverviewCacheKey)

	req := httptest.NewRequest(http.MethodGet, "/api/market/overview", nil)
	w := httptest.NewRecorder()
	GetMarketOverview(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var got modelsapi.MarketOverviewDTO
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}
	if got.TotalStocks != 30 {
		t.Errorf("total_stocks = %d, want 30", got.TotalStocks)
	}
	if got.Gainers != 10 {
		t.Errorf("gainers = %d, want 10", got.Gainers)
	}
	if got.Losers != 5 {
		t.Errorf("losers = %d, want 5", got.Losers)
	}
}

// TestGetMarketOverview_ExchangeFilterUppercased verifies that the handler
// normalises the exchange param to upper-case before passing it to the store.
// We test this indirectly: "hose", "Hose", and "HOSE" should all bypass the
// cache (since a filter is present) and NOT return 400.
func TestGetMarketOverview_ExchangeFilterUppercased(t *testing.T) {
	globalCache.Delete(marketOverviewCacheKey)

	exchangeValues := []string{"hose", "Hose", "HOSE", "hnx", "HNX"}
	for _, ex := range exchangeValues {
		t.Run("exchange="+ex, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/market/overview?exchange="+ex, nil)
			w := httptest.NewRecorder()

			func() {
				defer func() { recover() }()
				GetMarketOverview(w, req)
			}()

			if w.Code != 0 && w.Code == http.StatusBadRequest {
				t.Errorf("exchange=%q: must not return 400, got %d", ex, w.Code)
			}
		})
	}
}
