package server

import (
	"net/http"

	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"
)

// TriggerCryptoCrawlerHandler handles POST /api/trigger/crypto-crawler
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
