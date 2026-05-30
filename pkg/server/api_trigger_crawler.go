package server

import (
	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"
	"net/http"
)

func TriggerCrawlerHandler(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Info("Trigger crawler handler")
	client := requireGRPCClient(w)
	if client == nil {
		return
	}
	_, err := client.TriggerCrawler(r.Context(), &pb.Empty{})
	if err != nil {
		logger.Logger.Errorf("Trigger crawler failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}
	globalCache.Delete(marketOverviewCacheKey)
	w.WriteHeader(http.StatusAccepted)
}
