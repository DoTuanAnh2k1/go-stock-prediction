package server

import (
	"net/http"

	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"
)

// TriggerCryptoPredictHandler godoc
//
//	@Summary      Trigger crypto prediction
//	@Description  Runs a prediction for all registered cryptocurrency instruments (BTC, ETH) in the background via the prediction service. Returns 202 immediately.
//	@Tags         Triggers
//	@Accept       json
//	@Produce      json
//	@Success      202  "Accepted"
//	@Failure      401  {object}  ResponseFailure
//	@Failure      500  {object}  ResponseFailure
//	@Failure      503  {object}  ResponseFailure
//	@Security     BearerAuth
//	@Router       /api/trigger/crypto-predict [post]
func TriggerCryptoPredictHandler(w http.ResponseWriter, r *http.Request) {
	logger.Ctx(r.Context()).Info("[trigger] Crypto predict handler called")
	client := requireGRPCClient(w)
	if client == nil {
		return
	}
	_, err := client.TriggerCryptoPredict(r.Context(), &pb.Empty{})
	if err != nil {
		logger.Ctx(r.Context()).Errorf("[trigger] Crypto prediction failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
