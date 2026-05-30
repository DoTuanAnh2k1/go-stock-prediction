package server

import (
	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/service/crawler"
	"net/http"
)

func TriggerGoldCrawlerHandler(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Info("Trigger gold crawler handler")
	if err := crawler.CronjobGoldCrawler(); err != nil {
		logger.Logger.Errorf("Gold crawler failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
