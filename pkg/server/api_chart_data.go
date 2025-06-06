package server

import (
	"go-stock-prediction/pkg/logger"
	modelsapi "go-stock-prediction/pkg/models/models_api"
	"go-stock-prediction/pkg/store/repository"
	"net/http"
	"strings"
)

// GetChartData - GET /api/stocks/{symbol}/chart?period=1D&interval=5m
func GetChartData(w http.ResponseWriter, r *http.Request) {
	// Extract symbol from URL
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		ResponseError(w, http.StatusBadRequest, "Invalid URL format")
		return
	}
	symbol := strings.ToUpper(pathParts[3])

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "1D"
	}

	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "1h"
	}

	logger.Logger.Infof("📈 Getting chart data for %s, period: %s, interval: %s", symbol, period, interval)

	store := repository.GetSingleton()

	// Get stock
	stock, err := store.GetStockBySymbol(symbol)
	if err != nil {
		ResponseError(w, http.StatusNotFound, "Stock not found")
		return
	}

	// Get historical data
	fromDate, toDate := calculateDateRange(period)
	prices, err := store.GetStockPricesByStockIDAndDateRange(stock.ID, fromDate, toDate)
	if err != nil {
		ResponseError(w, http.StatusInternalServerError, "Failed to get chart data")
		return
	}

	// Convert to chart format
	var chartData []modelsapi.ChartPointDTO
	var volumeData []modelsapi.VolumePointDTO

	for _, price := range prices {
		chartPoint := modelsapi.ChartPointDTO{
			Timestamp: price.TradingDate,
			Open:      price.OpenPrice,
			High:      price.HighPrice,
			Low:       price.LowPrice,
			Close:     price.ClosePrice,
		}
		chartData = append(chartData, chartPoint)

		volumePoint := modelsapi.VolumePointDTO{
			Timestamp: price.TradingDate,
			Volume:    price.Volume,
			Value:     price.Value,
		}
		volumeData = append(volumeData, volumePoint)
	}

	chartDTO := &modelsapi.StockChartDataDTO{
		Symbol:   symbol,
		Period:   period,
		Interval: interval,
		Data:     chartData,
		Volume:   volumeData,
		Total:    len(chartData),
	}

	ResponseSuccess(w, http.StatusOK, chartDTO)
}
