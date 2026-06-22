package server

import (
	"net/http"

	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"
)

// TriggerCryptoHistoryHandler godoc
//
//	@Summary      Trigger crypto price history import
//	@Description  Starts a historical crypto price import (BTC/ETH/SOL, 180-day daily from CoinGecko) in the background via the prediction service. Useful to backfill a newly added coin. Returns 202 immediately.
//	@Tags         Triggers
//	@Accept       json
//	@Produce      json
//	@Success      202  "Accepted"
//	@Failure      401  {object}  ResponseFailure
//	@Failure      500  {object}  ResponseFailure
//	@Failure      503  {object}  ResponseFailure
//	@Security     BearerAuth
//	@Router       /api/trigger/crypto-history [post]
func TriggerCryptoHistoryHandler(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Info("[trigger] crypto-history import requested")
	client := requireGRPCClient(w)
	if client == nil {
		return
	}
	_, err := client.TriggerCryptoHistory(r.Context(), &pb.Empty{})
	if err != nil {
		logger.Logger.Errorf("[trigger] crypto-history failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
