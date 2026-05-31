package server

import (
	"net/http"

	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"
)

// TriggerFuelCrawlerHandler godoc
//
//	@Summary      Trigger fuel price crawler
//	@Description  Starts a fuel price crawl in the background via the prediction service. Returns 202 immediately.
//	@Tags         Triggers
//	@Accept       json
//	@Produce      json
//	@Success      202  "Accepted"
//	@Failure      401  {object}  ResponseFailure
//	@Failure      500  {object}  ResponseFailure
//	@Failure      503  {object}  ResponseFailure
//	@Security     BearerAuth
//	@Router       /api/trigger/fuel-crawler [post]
func TriggerFuelCrawlerHandler(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Info("[trigger] Fuel crawler handler called")
	client := requireGRPCClient(w)
	if client == nil {
		return
	}
	_, err := client.TriggerFuelCrawler(r.Context(), &pb.Empty{})
	if err != nil {
		logger.Logger.Errorf("[trigger] Fuel crawler failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
