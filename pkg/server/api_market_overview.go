package server

import (
	"go-stock-prediction/pkg/logger"
	modelsapi "go-stock-prediction/pkg/models/models_api"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"go-stock-prediction/pkg/store/repository"
	"net/http"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

const marketOverviewCacheKey = "market_overview"

// GetMarketOverview - GET /api/market/overview
// Supports optional query params: ?sector=ngan-hang and ?exchange=HOSE
func GetMarketOverview(w http.ResponseWriter, r *http.Request) {
	sectorFilter := strings.TrimSpace(r.URL.Query().Get("sector"))
	exchangeFilter := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("exchange")))

	// Only use cache when there are no filters
	cacheKey := marketOverviewCacheKey
	if sectorFilter == "" && exchangeFilter == "" {
		if cached, ok := globalCache.Get(cacheKey); ok {
			ResponseSuccess(w, http.StatusOK, cached)
			return
		}
	}

	logger.Logger.Info("Getting market overview...")

	store := repository.GetSingleton()

	// Get all VN30 stocks with latest prices
	vn30Stocks, err := store.GetVN30Stocks()
	if err != nil {
		ResponseError(w, http.StatusInternalServerError, "Failed to get VN30 stocks")
		return
	}

	// Resolve exchange filter to an exchange ID if provided
	var filterExchangeID uint
	if exchangeFilter != "" {
		exchange, err := store.GetExchangeByCode(exchangeFilter)
		if err != nil {
			// Unknown exchange code — return empty list instead of error
			logger.Logger.Warnf("Exchange not found for filter code=%s: %v", exchangeFilter, err)
		} else {
			filterExchangeID = exchange.ID
		}
	}

	// Apply sector / exchange filters
	filteredStocks := make([]modelsdb.Stock, 0, len(vn30Stocks))
	for _, s := range vn30Stocks {
		if sectorFilter != "" && !strings.EqualFold(s.Sector, sectorFilter) {
			continue
		}
		if filterExchangeID != 0 && s.ExchangeID != filterExchangeID {
			continue
		}
		filteredStocks = append(filteredStocks, s)
	}

	var allCurrentPrices []modelsapi.StockCurrentPriceDTO
	var gainers, losers, unchanged int
	var totalVolume int64
	var totalValue decimal.Decimal

	for _, stock := range filteredStocks {
		latestPrice, err := store.GetLatestStockPriceByStockID(stock.ID)
		if err != nil {
			continue
		}

		currentPrice := modelsapi.StockCurrentPriceDTO{
			Stock: modelsapi.StockDTO{
				ID:          stock.ID,
				Symbol:      stock.Symbol,
				CompanyName: stock.CompanyName,
				ExchangeID:  stock.ExchangeID,
				IsVN30:      stock.IsVN30,
				Sector:      stock.Sector,
			},
			CurrentPrice:  latestPrice.ClosePrice,
			Change:        latestPrice.Change,
			ChangePercent: latestPrice.ChangePercent,
			Volume:        latestPrice.Volume,
			Value:         latestPrice.Value,
			High:          latestPrice.HighPrice,
			Low:           latestPrice.LowPrice,
			Open:          latestPrice.OpenPrice,
			TradingDate:   latestPrice.TradingDate,
			LastUpdated:   latestPrice.UpdatedAt,
			MarketStatus:  getMarketStatus(),
		}

		allCurrentPrices = append(allCurrentPrices, currentPrice)

		// Count gainers/losers
		if latestPrice.Change.GreaterThan(decimal.Zero) {
			gainers++
		} else if latestPrice.Change.LessThan(decimal.Zero) {
			losers++
		} else {
			unchanged++
		}

		totalVolume += latestPrice.Volume
		totalValue = totalValue.Add(latestPrice.Value)
	}

	// Sort for top gainers/losers/most active
	topGainers := getTopByChange(allCurrentPrices, true, 5)
	topLosers := getTopByChange(allCurrentPrices, false, 5)
	mostActive := getTopByVolume(allCurrentPrices, 5)

	// Calculate VN30 index from actual stock data
	vn30Index := decimal.Zero
	indexChange := decimal.Zero
	indexPercent := decimal.Zero
	if len(allCurrentPrices) > 0 {
		totalPrice := decimal.Zero
		totalChange := decimal.Zero
		for _, sp := range allCurrentPrices {
			totalPrice = totalPrice.Add(sp.CurrentPrice)
			totalChange = totalChange.Add(sp.Change)
		}
		vn30Index = totalPrice.Div(decimal.NewFromInt(int64(len(allCurrentPrices))))
		indexChange = totalChange.Div(decimal.NewFromInt(int64(len(allCurrentPrices))))
		if vn30Index.GreaterThan(decimal.Zero) {
			indexPercent = indexChange.Div(vn30Index).Mul(decimal.NewFromInt(100))
		}
	}

	var flatStocks []modelsapi.StockFlatDTO
	for _, cp := range allCurrentPrices {
		flatStocks = append(flatStocks, modelsapi.StockFlatDTO{
			Symbol:        cp.Stock.Symbol,
			CompanyName:   cp.Stock.CompanyName,
			Sector:        cp.Stock.Sector,
			Exchange:      "",
			IsVN30:        cp.Stock.IsVN30,
			CurrentPrice:  cp.CurrentPrice,
			Change:        cp.Change,
			ChangePercent: cp.ChangePercent,
			Volume:        cp.Volume,
			Value:         cp.Value,
		})
	}

	overview := &modelsapi.MarketOverviewDTO{
		VN30Index:    vn30Index,
		IndexChange:  indexChange,
		IndexPercent: indexPercent,
		TotalStocks:  len(filteredStocks),
		Gainers:      gainers,
		Losers:       losers,
		Unchanged:    unchanged,
		TotalVolume:  totalVolume,
		TotalValue:   totalValue,
		TopGainers:   topGainers,
		TopLosers:    topLosers,
		MostActive:   mostActive,
		Stocks:       flatStocks,
		LastUpdated:  time.Now(),
	}

	// Only cache unfiltered responses
	if sectorFilter == "" && exchangeFilter == "" {
		globalCache.Set(marketOverviewCacheKey, overview, 60*time.Second)
	}
	ResponseSuccess(w, http.StatusOK, overview)
}
