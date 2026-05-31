package server

import (
	"net/http"

	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"
)

// TriggerCryptoCrawlerHandler godoc
//
//	@Summary      Trigger crypto price crawler
//	@Description  Starts a cryptocurrency (BTC/ETH) price crawl in the background via the prediction service. Returns 202 immediately.
//	@Tags         Triggers
//	@Accept       json
//	@Produce      json
//	@Success      202  "Accepted"
//	@Failure      401  {object}  ResponseFailure
//	@Failure      500  {object}  ResponseFailure
//	@Failure      503  {object}  ResponseFailure
//	@Security     BearerAuth
//	@Router       /api/trigger/crypto-crawler [post]
func TriggerCryptoCrawlerHandler(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Info("[trigger] Crypto crawler handler called")
	client := requireGRPCClient(w)
	if client == nil {
		return
	}
	_, err := client.TriggerCryptoCrawler(r.Context(), &pb.Empty{})
	if err != nil {
		logger.Logger.Errorf("[trigger] Crypto crawler failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
