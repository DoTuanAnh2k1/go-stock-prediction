package server

import (
	"net/http"

	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"
)

// TriggerSP500CrawlerHandler godoc
//
//	@Summary      Trigger S&P 500 price crawler
//	@Description  Starts S&P 500 price crawler in the background via the prediction service.
//	@Tags         Triggers
//	@Produce      json
//	@Success      200  {object}  map[string]string
//	@Failure      401  {object}  ResponseFailure
//	@Failure      500  {object}  ResponseFailure
//	@Failure      503  {object}  ResponseFailure
//	@Security     BearerAuth
//	@Router       /api/trigger/sp500-crawler [post]
func TriggerSP500CrawlerHandler(w http.ResponseWriter, r *http.Request) {
	client := requireGRPCClient(w)
	if client == nil {
		return
	}
	resp, err := client.TriggerSP500Crawler(r.Context(), &pb.Empty{})
	if err != nil {
		logger.Logger.Errorf("TriggerSP500Crawler: %v", err)
		ResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ResponseSuccess(w, http.StatusOK, map[string]string{"message": resp.Message})
}

// TriggerSP500PredictHandler godoc
//
//	@Summary      Trigger S&P 500 prediction
//	@Description  Runs prediction for all S&P 500 symbols in the background via the prediction service.
//	@Tags         Triggers
//	@Produce      json
//	@Success      200  {object}  map[string]string
//	@Failure      401  {object}  ResponseFailure
//	@Failure      500  {object}  ResponseFailure
//	@Failure      503  {object}  ResponseFailure
//	@Security     BearerAuth
//	@Router       /api/trigger/sp500-predict [post]
func TriggerSP500PredictHandler(w http.ResponseWriter, r *http.Request) {
	client := requireGRPCClient(w)
	if client == nil {
		return
	}
	resp, err := client.TriggerSP500Predict(r.Context(), &pb.Empty{})
	if err != nil {
		logger.Logger.Errorf("TriggerSP500Predict: %v", err)
		ResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ResponseSuccess(w, http.StatusOK, map[string]string{"message": resp.Message})
}
