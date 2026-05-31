package server

import (
	"net/http"

	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"
)

// TriggerFuelCrawlerHandler handles POST /api/trigger/fuel-crawler
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
