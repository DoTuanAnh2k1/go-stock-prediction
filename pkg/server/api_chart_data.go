package server

import (
	"go-stock-prediction/pkg/logger"
	modelsapi "go-stock-prediction/pkg/models/models_api"
	"go-stock-prediction/pkg/store/repository"
	"net/http"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// GetChartData godoc
//
//	@Summary      Get OHLCV chart data for a stock
//	@Description  Returns candlestick (OHLCV) and volume data for the given stock symbol over the specified period and interval. When period=1D and interval=1h, returns hourly intraday data from the intraday table.
//	@Tags         Stocks
//	@Produce      json
//	@Param        symbol   path   string false "Stock symbol (e.g. VCB)"
//	@Param        period   query  string false "Time period (1D, 1W, 1M, 3M, 6M, 1Y)" default(1D)
//	@Param        interval query  string false "Data interval (e.g. 5m, 1h, 1d)"       default(1h)
//	@Success      200 {object} modelsapi.StockChartDataDTO
//	@Failure      400 {object} ResponseFailure
//	@Failure      404 {object} ResponseFailure
//	@Failure      500 {object} ResponseFailure
//	@Router       /api/stocks/{symbol}/chart [get]
func GetChartData(w http.ResponseWriter, r *http.Request) {
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

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "1D"
	}

	if err := validatePeriod(period); err != nil {
		ResponseError(w, http.StatusBadRequest, err.Error())
		return
	}

	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "1h"
	}

	logger.Logger.Infof("Getting chart data for %s, period: %s, interval: %s", symbol, period, interval)

	store := repository.GetSingleton()

	// When period=1D and interval=1h, use intraday table for hourly data
	if period == "1D" && interval == "1h" {
		to := time.Now()
		from := to.Add(-24 * time.Hour)
		intradayPrices, err := store.GetStockIntradayByRange(symbol, from, to)
		if err != nil {
			logger.Logger.Errorf("[api/stocks/%s/chart] Failed to get intraday prices: %v", symbol, err)
			ResponseError(w, http.StatusInternalServerError, "Failed to get intraday chart data")
			return
		}

		var chartData []modelsapi.ChartPointDTO
		var volumeData []modelsapi.VolumePointDTO

		// intraday prices are ordered DESC from DB — reverse for chart (oldest first)
		for i := len(intradayPrices) - 1; i >= 0; i-- {
			p := intradayPrices[i]
			chartData = append(chartData, modelsapi.ChartPointDTO{
				Timestamp: p.Timestamp,
				Open:      p.OpenPrice,
				High:      p.HighPrice,
				Low:       p.LowPrice,
				Close:     p.ClosePrice,
			})
			volumeData = append(volumeData, modelsapi.VolumePointDTO{
				Timestamp: p.Timestamp,
				Volume:    p.Volume,
				Value:     decimal.Zero,
			})
		}

		ResponseSuccess(w, http.StatusOK, &modelsapi.StockChartDataDTO{
			Symbol:   symbol,
			Period:   period,
			Interval: interval,
			Data:     chartData,
			Volume:   volumeData,
			Total:    len(chartData),
		})
		return
	}

	// Get stock (needed for daily price lookup by stock ID)
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
