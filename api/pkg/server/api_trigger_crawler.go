package server

import (
	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"
	"net/http"
)

// TriggerCrawlerHandler godoc
//
//	@Summary      Trigger VN30 stock crawler
//	@Description  Starts a VN30 stock price crawl in the background via the prediction service. Also invalidates the market overview cache.
//	@Tags         Triggers
//	@Accept       json
//	@Produce      json
//	@Success      202  "Accepted"
//	@Failure      401  {object}  ResponseFailure
//	@Failure      500  {object}  ResponseFailure
//	@Failure      503  {object}  ResponseFailure
//	@Security     BearerAuth
//	@Router       /api/trigger/crawler [post]
func TriggerCrawlerHandler(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Info("Trigger crawler handler")
	client := requireGRPCClient(w)
	if client == nil {
		return
	}
	_, err := client.TriggerCrawler(r.Context(), &pb.Empty{})
	if err != nil {
		logger.Logger.Errorf("Trigger crawler failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}
	globalCache.Delete(marketOverviewCacheKey)
	w.WriteHeader(http.StatusAccepted)
}
