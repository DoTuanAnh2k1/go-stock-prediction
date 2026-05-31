package server

import (
	"net/http"

	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"
)

// TriggerNasdaqPredictHandler godoc
//
//	@Summary      Trigger NASDAQ100 prediction
//	@Description  Runs a prediction for all NASDAQ100 instruments in the background via the prediction service. Returns 202 immediately.
//	@Tags         Triggers
//	@Accept       json
//	@Produce      json
//	@Success      202  "Accepted"
//	@Failure      401  {object}  ResponseFailure
//	@Failure      500  {object}  ResponseFailure
//	@Failure      503  {object}  ResponseFailure
//	@Security     BearerAuth
//	@Router       /api/trigger/nasdaq-predict [post]
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
