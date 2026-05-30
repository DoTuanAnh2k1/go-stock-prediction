package server

import (
	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"
	"net/http"
)

func TriggerGoldCrawlerHandler(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Info("Trigger gold crawler handler")
	client := requireGRPCClient(w)
	if client == nil {
		return
	}
	_, err := client.TriggerGoldCrawler(r.Context(), &pb.Empty{})
	if err != nil {
		logger.Logger.Errorf("Gold crawler failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
