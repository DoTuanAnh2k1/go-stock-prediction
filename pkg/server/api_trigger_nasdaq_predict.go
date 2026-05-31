package server

import (
	"net/http"

	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"
)

// TriggerNasdaqPredictHandler handles POST /api/trigger/nasdaq-predict
func TriggerNasdaqPredictHandler(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Info("[trigger] NASDAQ predict handler called")
	client := requireGRPCClient(w)
	if client == nil {
		return
	}
	_, err := client.TriggerNasdaqPredict(r.Context(), &pb.Empty{})
	if err != nil {
		logger.Logger.Errorf("[trigger] NASDAQ prediction failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
