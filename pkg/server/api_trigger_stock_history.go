package server

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/service/crawler"
)

const (
	defaultHistoricalDays = 365
	maxHistoricalDays     = 1000
)

// TriggerStockHistoryHandler handles POST /api/trigger/stock-history.
// It accepts an optional query param ?days=N (default 365, max 1000) and starts
// a background goroutine that fetches N days of price history for every VN30 stock,
// upserting the results into the database.
func TriggerStockHistoryHandler(w http.ResponseWriter, r *http.Request) {
	days := defaultHistoricalDays

	if raw := r.URL.Query().Get("days"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			ResponseError(w, http.StatusBadRequest, "invalid days parameter: must be a positive integer")
			return
		}
		if n > maxHistoricalDays {
			n = maxHistoricalDays
		}
		days = n
	}

	logger.Logger.Infof("[trigger] stock-history requested: days=%d", days)

	go func(d int) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		saved, skipped, err := crawler.CrawlHistoricalAll(ctx, d)
		if err != nil {
			logger.Logger.Errorf("[trigger] stock-history failed: %v", err)
			return
		}
		logger.Logger.Infof("[trigger] stock-history completed: saved=%d skipped=%d", saved, skipped)
	}(days)

	ResponseSuccess(w, http.StatusAccepted, map[string]interface{}{
		"status":  "started",
		"days":    days,
		"message": "Historical stock price crawl started in background",
	})
}
