package server

import (
	"go-stock-prediction/pkg/logger"
	modelsapi "go-stock-prediction/pkg/models/models_api"
	"go-stock-prediction/pkg/store/repository"
	"net/http"
	"time"

	"github.com/shopspring/decimal"
)

const marketOverviewCacheKey = "market_overview"

// GetMarketOverview - GET /api/market/overview
func GetMarketOverview(w http.ResponseWriter, r *http.Request) {
	if cached, ok := globalCache.Get(marketOverviewCacheKey); ok {
		ResponseSuccess(w, http.StatusOK, cached)
		return
	}

	logger.Logger.Info("Getting market overview...")

	store := repository.GetSingleton()

	// Get all VN30 stocks with latest prices
	vn30Stocks, err := store.GetVN30Stocks()
	if err != nil {
		ResponseError(w, http.StatusInternalServerError, "Failed to get VN30 stocks")
		return
	}

	var allCurrentPrices []modelsapi.StockCurrentPriceDTO
	var gainers, losers, unchanged int
	var totalVolume int64
	var totalValue decimal.Decimal

	for _, stock := range vn30Stocks {
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

	// Mock VN30 index calculation (simplified)
	vn30Index := decimal.NewFromFloat(1200.50) // Mock index value
	indexChange := decimal.NewFromFloat(5.25)
	indexPercent := decimal.NewFromFloat(0.44)

	overview := &modelsapi.MarketOverviewDTO{
		VN30Index:    vn30Index,
		IndexChange:  indexChange,
		IndexPercent: indexPercent,
		TotalStocks:  len(vn30Stocks),
		Gainers:      gainers,
		Losers:       losers,
		Unchanged:    unchanged,
		TotalVolume:  totalVolume,
		TotalValue:   totalValue,
		TopGainers:   topGainers,
		TopLosers:    topLosers,
		MostActive:   mostActive,
		LastUpdated:  time.Now(),
	}

	globalCache.Set(marketOverviewCacheKey, overview, 60*time.Second)
	ResponseSuccess(w, http.StatusOK, overview)
}
