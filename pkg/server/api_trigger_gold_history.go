package server

import (
	"net/http"

	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/service/crawler"
)

// TriggerGoldHistoryHandler handles POST /api/trigger/gold-history.
// It runs ImportXAUHistory in a background goroutine and immediately returns 202 Accepted.
func TriggerGoldHistoryHandler(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Info("[trigger] gold-history import requested")
	go crawler.ImportXAUHistory()
	w.WriteHeader(http.StatusAccepted)
}
