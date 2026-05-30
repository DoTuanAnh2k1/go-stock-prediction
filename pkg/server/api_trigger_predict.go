package server

import (
	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"
	"net/http"
)

func TriggerPredictHandler(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Info("Trigger predict")
	client := requireGRPCClient(w)
	if client == nil {
		return
	}
	resp, err := client.TriggerPredict(r.Context(), &pb.Empty{})
	if err != nil {
		logger.Logger.Errorf("Predict error: %v", err)
		ResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ResponseSuccess(w, http.StatusOK, map[string]string{"message": resp.GetMessage()})
}
