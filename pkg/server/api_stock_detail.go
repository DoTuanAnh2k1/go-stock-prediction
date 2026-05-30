package server

import (
	"go-stock-prediction/pkg/logger"
	modelsapi "go-stock-prediction/pkg/models/models_api"
	"go-stock-prediction/pkg/store/repository"
	"net/http"
	"strings"
	"time"
)

// GetStockDetail - GET /api/stocks/{symbol}/detail
// Returns comprehensive stock info: current price, 30-day history, latest predictions, and stats.
func GetStockDetail(w http.ResponseWriter, r *http.Request) {
	// Extract symbol from path /api/stocks/{symbol}/detail
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		ResponseError(w, http.StatusBadRequest, "Invalid path")
		return
	}
	symbol := strings.ToUpper(pathParts[3])

	if err := validateSymbol(symbol); err != nil {
		ResponseError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Check cache
	cacheKey := "stock_detail:" + symbol
	if cached, ok := globalCache.Get(cacheKey); ok {
		ResponseSuccess(w, http.StatusOK, cached)
		return
	}

	logger.Logger.Infof("Getting stock detail for %s", symbol)

	store := repository.GetSingleton()

	// 1. Get stock
	stock, err := store.GetStockBySymbol(symbol)
	if err != nil {
		ResponseError(w, http.StatusNotFound, "Stock not found: "+symbol)
		return
	}

	stockDTO := modelsapi.StockDTO{
		ID:          stock.ID,
		Symbol:      stock.Symbol,
		CompanyName: stock.CompanyName,
		ExchangeID:  stock.ExchangeID,
		IsVN30:      stock.IsVN30,
		IsVN100:     stock.IsVN100,
		Sector:      stock.Sector,
		ListingDate: stock.ListingDate,
	}

	// 2. Get latest price
	var currentPriceDTO modelsapi.StockCurrentPriceDTO
	latestPrice, err := store.GetLatestStockPriceByStockID(stock.ID)
	if err == nil && latestPrice != nil {
		currentPriceDTO = modelsapi.StockCurrentPriceDTO{
			Stock:         stockDTO,
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
	}

	// 3. Get historical prices (last 30 days)
	fromDate := time.Now().AddDate(0, -1, 0)
	prices, _ := store.GetStockPricesByStockIDAndDateRange(stock.ID, fromDate, time.Now())

	var historyDTOs []modelsapi.StockPriceDTO
	for _, p := range prices {
		historyDTOs = append(historyDTOs, modelsapi.StockPriceDTO{
			ID:            p.ID,
			StockID:       p.StockID,
			TradingDate:   p.TradingDate,
			OpenPrice:     p.OpenPrice,
			HighPrice:     p.HighPrice,
			LowPrice:      p.LowPrice,
			ClosePrice:    p.ClosePrice,
			Volume:        p.Volume,
			Value:         p.Value,
			Change:        p.Change,
			ChangePercent: p.ChangePercent,
			ForeignBuy:    p.ForeignBuy,
			ForeignSell:   p.ForeignSell,
		})
	}

	// 4. Calculate stats
	stats := calculateStockStats(prices)

	// 5. Get latest predictions (limit 12 = ~4 per algorithm)
	dbPredictions, _ := store.GetLatestPredictionsByStockID(stock.ID, 12)
	var predDTOs []modelsapi.PredictionDetailDTO
	for _, pred := range dbPredictions {
		predDTOs = append(predDTOs, modelsapi.PredictionDetailDTO{
			ID:             pred.ID,
			Stock:          stockDTO,
			PredictedPrice: pred.PredictedPrice,
			CurrentPrice:   pred.CurrentPrice,
			ActualPrice:    pred.ActualPrice,
			Confidence:     pred.Confidence,
			AlgorithmName:  pred.AlgorithmName,
			PredictionDate: pred.PredictionDate,
			TargetDate:     pred.TargetDate,
			Accuracy:       pred.Accuracy,
			Status:         getPredictionStatus(pred.TargetDate, pred.ActualPrice),
		})
	}

	result := &modelsapi.StockDetailDTO{
		Stock:        stockDTO,
		CurrentPrice: currentPriceDTO,
		History:      historyDTOs,
		Predictions:  predDTOs,
		Stats:        stats,
	}

	// Cache for 60 seconds
	globalCache.Set(cacheKey, result, 60*time.Second)
	ResponseSuccess(w, http.StatusOK, result)
}
