package server

import (
	"encoding/json"
	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/service/predict"
	"net/http"
)

type TriggerTrainRequest struct {
	Algorithm string `json:"algorithm"`
}

type TriggerTrainResponse struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

func TriggerTrainHandler(w http.ResponseWriter, r *http.Request) {
	var req TriggerTrainRequest
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req) // ignore decode error — algorithm field is optional
	}

	logger.Logger.Infof("Trigger train request: algorithm=%q", req.Algorithm)

	if predict.IsTraining() {
		ResponseError(w, http.StatusConflict, "Training already in progress")
		return
	}

	var sessionID string
	var err error

	if req.Algorithm != "" {
		if err = validateAlgorithm(req.Algorithm); err != nil {
			ResponseError(w, http.StatusBadRequest, err.Error())
			return
		}
		// Train single algorithm synchronously (fast path)
		sessionID, err = predict.TrainSingleAlgorithmByName(req.Algorithm)
	} else {
		// Train all algorithms in background goroutine
		sessionID, err = predict.TrainAllAlgorithms()
	}

	if err != nil {
		logger.Logger.Errorf("Failed to start training: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to start training: "+err.Error())
		return
	}

	ResponseSuccess(w, http.StatusAccepted, TriggerTrainResponse{
		SessionID: sessionID,
		Message:   "Training started",
	})
}
