package server

import (
	"go-stock-prediction/pkg/logger"
	modelsapi "go-stock-prediction/pkg/models/models_api"
	"go-stock-prediction/pkg/store/repository"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// GetHistoricalData godoc
//
//	@Summary      Get historical price data for a stock
//	@Description  Returns OHLCV price history and statistics for the given stock. Supports ?days=N or ?period=1M query params.
//	@Tags         Stocks
//	@Produce      json
//	@Param        symbol path   string false "Stock symbol (e.g. VCB)"
//	@Param        days   query  int    false "Number of days of history (1-1000, overrides period)"
//	@Param        period query  string false "Time period shorthand (1D, 1W, 1M, 3M, 6M, 1Y)" default(1M)
//	@Param        limit  query  int    false "Maximum number of records to return (1-1000)"     default(500)
//	@Success      200 {object} modelsapi.StockHistoricalDataDTO
//	@Failure      400 {object} ResponseFailure
//	@Failure      404 {object} ResponseFailure
//	@Failure      500 {object} ResponseFailure
//	@Router       /api/stocks/{symbol}/history [get]
func GetHistoricalData(w http.ResponseWriter, r *http.Request) {
	// Extract symbol from URL
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

	// Parse query params — support both ?days=N (frontend sparklines) and ?period=1M
	var fromDate, toDate time.Time
	var period string

	daysStr := r.URL.Query().Get("days")
	if daysStr != "" {
		days, err := strconv.Atoi(daysStr)
		if err != nil || days <= 0 || days > 1000 {
			days = 30
		}
		toDate = time.Now()
		fromDate = toDate.AddDate(0, 0, -days)
		period = strconv.Itoa(days) + "D"
	} else {
		period = r.URL.Query().Get("period")
		if period == "" {
			period = "1M"
		}
		if err := validatePeriod(period); err != nil {
			ResponseError(w, http.StatusBadRequest, err.Error())
			return
		}
		fromDate, toDate = calculateDateRange(period)
	}

	limitStr := r.URL.Query().Get("limit")
	limit, err := validateLimit(limitStr, 500, 1000)
	if err != nil {
		ResponseError(w, http.StatusBadRequest, err.Error())
		return
	}

	logger.Logger.Infof("📊 Getting historical data for %s, period: %s", symbol, period)

	store := repository.GetSingleton()

	// Get stock info
	stock, err := store.GetStockBySymbol(symbol)
	if err != nil {
		ResponseError(w, http.StatusNotFound, "Stock not found")
		return
	}

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
