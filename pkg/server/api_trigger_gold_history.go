package server

import (
	"net/http"

	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"
)

// TriggerGoldHistoryHandler handles POST /api/trigger/gold-history.
// It delegates to the prediction microservice via gRPC and immediately returns 202 Accepted.
func TriggerGoldHistoryHandler(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Info("[trigger] gold-history import requested")
	client := requireGRPCClient(w)
	if client == nil {
		return
	}
	_, err := client.TriggerGoldHistory(r.Context(), &pb.Empty{})
	if err != nil {
		logger.Logger.Errorf("[trigger] gold-history failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
