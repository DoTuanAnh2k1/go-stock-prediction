package server

import (
	"net/http"

	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"
)

// TriggerNasdaqCrawlerHandler godoc
//
//	@Summary      Trigger NASDAQ100 crawler
//	@Description  Starts a NASDAQ100 price crawl in the background via the prediction service. Returns 202 immediately.
//	@Tags         Triggers
//	@Accept       json
//	@Produce      json
//	@Success      202  "Accepted"
//	@Failure      401  {object}  ResponseFailure
//	@Failure      500  {object}  ResponseFailure
//	@Failure      503  {object}  ResponseFailure
//	@Security     BearerAuth
//	@Router       /api/trigger/nasdaq-crawler [post]
func TriggerNasdaqCrawlerHandler(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Info("[trigger] NASDAQ crawler handler called")
	client := requireGRPCClient(w)
	if client == nil {
		return
	}
	_, err := client.TriggerNasdaqCrawler(r.Context(), &pb.Empty{})
	if err != nil {
		logger.Logger.Errorf("[trigger] NASDAQ crawler failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
