package server

import (
	"net/http"

	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"
)

// TriggerCryptoPredictHandler handles POST /api/trigger/crypto-predict
func TriggerCryptoPredictHandler(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Info("[trigger] Crypto predict handler called")
	client := requireGRPCClient(w)
	if client == nil {
		return
	}
	_, err := client.TriggerCryptoPredict(r.Context(), &pb.Empty{})
	if err != nil {
		logger.Logger.Errorf("[trigger] Crypto prediction failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
