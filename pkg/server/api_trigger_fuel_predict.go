package server

import (
	"net/http"

	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"
)

// TriggerFuelPredictHandler handles POST /api/trigger/fuel-predict
func TriggerFuelPredictHandler(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Info("[trigger] Fuel predict handler called")
	client := requireGRPCClient(w)
	if client == nil {
		return
	}
	_, err := client.TriggerFuelPredict(r.Context(), &pb.Empty{})
	if err != nil {
		logger.Logger.Errorf("[trigger] Fuel prediction failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
