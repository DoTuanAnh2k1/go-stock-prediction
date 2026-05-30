package server

import (
	"context"
	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/service/crawler"
	"go-stock-prediction/pkg/service/predict"
	"go-stock-prediction/pkg/store/repository"
	"net/http"
	"strings"
	"time"
)

// TriggerStockCrawl crawls latest data for a single stock
// POST /api/stocks/{symbol}/crawl
func TriggerStockCrawl(w http.ResponseWriter, r *http.Request) {
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

	// Verify stock exists in DB
	store := repository.GetSingleton()
	_, err := store.GetStockBySymbol(symbol)
	if err != nil {
		ResponseError(w, http.StatusNotFound, "Không tìm thấy cổ phiếu: "+symbol)
		return
	}

	logger.Logger.Infof("Triggering crawl for stock: %s", symbol)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	stockData, err := crawler.CrawlAndSaveSingleStock(ctx, symbol)
	if err != nil {
		logger.Logger.Errorf("Failed to crawl %s: %v", symbol, err)
		ResponseError(w, http.StatusInternalServerError, "Thu thập dữ liệu thất bại: "+err.Error())
		return
	}

	// Invalidate cache for this stock
	globalCache.Delete("stock_detail:" + symbol)

	ResponseSuccess(w, http.StatusOK, map[string]interface{}{
		"symbol":  symbol,
		"message": "Thu thập dữ liệu thành công",
		"data":    stockData,
	})
}

// TriggerStockPredict runs all prediction algorithms for a single stock
// POST /api/stocks/{symbol}/predict
func TriggerStockPredict(w http.ResponseWriter, r *http.Request) {
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

	logger.Logger.Infof("Triggering prediction for stock: %s", symbol)

	count, err := predict.PredictSingleStock(symbol)
	if err != nil {
		logger.Logger.Errorf("Failed to predict %s: %v", symbol, err)
		ResponseError(w, http.StatusInternalServerError, "Dự đoán thất bại: "+err.Error())
		return
	}

	// Invalidate cache for this stock
	globalCache.Delete("stock_detail:" + symbol)

	ResponseSuccess(w, http.StatusOK, map[string]interface{}{
		"symbol":            symbol,
		"message":           "Dự đoán thành công",
		"predictions_count": count,
	})
}
