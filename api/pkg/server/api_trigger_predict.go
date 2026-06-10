package server

import (
	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"
	"net/http"
)

// TriggerPredictHandler godoc
//
//	@Summary      Trigger prediction run
//	@Description  Runs weekly training and daily prediction synchronously across all registered markets via the prediction service.
//	@Tags         Triggers
//	@Accept       json
//	@Produce      json
//	@Success      200  {object}  map[string]string
//	@Failure      401  {object}  ResponseFailure
//	@Failure      500  {object}  ResponseFailure
//	@Failure      503  {object}  ResponseFailure
//	@Security     BearerAuth
//	@Router       /api/trigger/predict [post]
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
