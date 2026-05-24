package server

import (
	"go-stock-prediction/pkg/logger"
	modelsapi "go-stock-prediction/pkg/models/models_api"
	"go-stock-prediction/pkg/store/repository"
	"net/http"
	"strings"
)

// GetCurrentPrice - GET /api/stocks/{symbol}/current
func GetCurrentPrice(w http.ResponseWriter, r *http.Request) {
	// Extract symbol from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		ResponseError(w, http.StatusBadRequest, "Invalid URL format")
		return
	}
	symbol := strings.ToUpper(pathParts[3])

	if err := validateSymbol(symbol); err != nil {
		ResponseError(w, http.StatusBadRequest, err.Error())
		return
	}

	logger.Logger.Infof("📈 Getting current price for %s...", symbol)

	store := repository.GetSingleton()

	// Get stock info
	stock, err := store.GetStockBySymbol(symbol)
	if err != nil {
		ResponseError(w, http.StatusNotFound, "Stock not found")
		return
	}

	// Get latest price
	latestPrice, err := store.GetLatestStockPriceByStockID(stock.ID)
	if err != nil {
		ResponseError(w, http.StatusNotFound, "No price data found")
		return
	}

	// Convert to DTO
	currentPriceDTO := &modelsapi.StockCurrentPriceDTO{
		Stock: modelsapi.StockDTO{
			ID:          stock.ID,
			Symbol:      stock.Symbol,
			CompanyName: stock.CompanyName,
			ExchangeID:  stock.ExchangeID,
			IsVN30:      stock.IsVN30,
			IsVN100:     stock.IsVN100,
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

	ResponseSuccess(w, http.StatusOK, currentPriceDTO)
}
