package server

import (
	"net/http"

	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"
)

// TriggerFuelPredictHandler godoc
//
//	@Summary      Trigger fuel price prediction
//	@Description  Runs a prediction for all registered fuel instruments in the background via the prediction service. Returns 202 immediately.
//	@Tags         Triggers
//	@Accept       json
//	@Produce      json
//	@Success      202  "Accepted"
//	@Failure      401  {object}  ResponseFailure
//	@Failure      500  {object}  ResponseFailure
//	@Failure      503  {object}  ResponseFailure
//	@Security     BearerAuth
//	@Router       /api/trigger/fuel-predict [post]
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
