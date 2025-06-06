package server

import (
	"go-stock-prediction/pkg/logger"
	modelsapi "go-stock-prediction/pkg/models/models_api"
	"go-stock-prediction/pkg/store/repository"
	"net/http"
	"strconv"
	"strings"
)

// GetHistoricalData - GET /api/stocks/{symbol}/history?period=1M&limit=50
func GetHistoricalData(w http.ResponseWriter, r *http.Request) {
	// Extract symbol from URL
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		ResponseError(w, http.StatusBadRequest, "Invalid URL format")
		return
	}
	symbol := strings.ToUpper(pathParts[3])

	// Parse query params
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "1M"
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	logger.Logger.Infof("📊 Getting historical data for %s, period: %s", symbol, period)

	store := repository.GetSingleton()

	// Get stock info
	stock, err := store.GetStockBySymbol(symbol)
	if err != nil {
		ResponseError(w, http.StatusNotFound, "Stock not found")
		return
	}

	// Calculate date range based on period
	fromDate, toDate := calculateDateRange(period)

	// Get historical prices
	prices, err := store.GetStockPricesByStockIDAndDateRange(stock.ID, fromDate, toDate)
	if err != nil {
		ResponseError(w, http.StatusInternalServerError, "Failed to get historical data")
		return
	}

	// Limit results if needed
	if len(prices) > limit {
		prices = prices[:limit]
	}

	// Convert to DTOs
	var priceData []modelsapi.StockPriceDTO
	for _, price := range prices {
		priceDTO := modelsapi.StockPriceDTO{
			ID:            price.ID,
			StockID:       price.StockID,
			TradingDate:   price.TradingDate,
			OpenPrice:     price.OpenPrice,
			HighPrice:     price.HighPrice,
			LowPrice:      price.LowPrice,
			ClosePrice:    price.ClosePrice,
			Volume:        price.Volume,
			Value:         price.Value,
			Change:        price.Change,
			ChangePercent: price.ChangePercent,
			ForeignBuy:    price.ForeignBuy,
			ForeignSell:   price.ForeignSell,
		}
		priceData = append(priceData, priceDTO)
	}

	// Calculate statistics
	stats := calculateStockStats(prices)

	// Create response
	historicalDTO := &modelsapi.StockHistoricalDataDTO{
		Stock: modelsapi.StockDTO{
			ID:          stock.ID,
			Symbol:      stock.Symbol,
			CompanyName: stock.CompanyName,
			ExchangeID:  stock.ExchangeID,
			IsVN30:      stock.IsVN30,
			IsVN100:     stock.IsVN100,
			Sector:      stock.Sector,
		},
		Period:     period,
		StartDate:  fromDate,
		EndDate:    toDate,
		PriceData:  priceData,
		Statistics: stats,
		Total:      len(priceData),
	}

	ResponseSuccess(w, http.StatusOK, historicalDTO)
}
