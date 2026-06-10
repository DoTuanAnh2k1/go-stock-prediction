package server

import (
	"net/http"

	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"
)

// TriggerGoldHistoryHandler godoc
//
//	@Summary      Trigger gold price history import
//	@Description  Starts a historical XAU/USD gold price import in the background via the prediction service. Returns 202 immediately.
//	@Tags         Triggers
//	@Accept       json
//	@Produce      json
//	@Success      202  "Accepted"
//	@Failure      401  {object}  ResponseFailure
//	@Failure      500  {object}  ResponseFailure
//	@Failure      503  {object}  ResponseFailure
//	@Security     BearerAuth
//	@Router       /api/trigger/gold-history [post]
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
