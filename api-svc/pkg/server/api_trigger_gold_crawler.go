package server

import (
	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"
	"net/http"
)

// TriggerGoldCrawlerHandler godoc
//
//	@Summary      Trigger gold price crawler
//	@Description  Crawls current gold prices (SJC BTMC and XAU/USD) synchronously via the prediction service.
//	@Tags         Triggers
//	@Accept       json
//	@Produce      json
//	@Success      202  "Accepted"
//	@Failure      401  {object}  ResponseFailure
//	@Failure      500  {object}  ResponseFailure
//	@Failure      503  {object}  ResponseFailure
//	@Security     BearerAuth
//	@Router       /api/trigger/gold-crawler [post]
func TriggerGoldCrawlerHandler(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Info("Trigger gold crawler handler")
	client := requireGRPCClient(w)
	if client == nil {
		return
	}
	_, err := client.TriggerGoldCrawler(r.Context(), &pb.Empty{})
	if err != nil {
		logger.Logger.Errorf("Gold crawler failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
