package server

import (
	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/service/crawler"
	"net/http"
)

func TriggerCrawlerHandler(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Info("Trigger crawler handler")
	crawler.CronjobCrawler()
	w.WriteHeader(http.StatusAccepted)
}
