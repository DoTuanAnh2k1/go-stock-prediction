package server

import (
	"encoding/json"
	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

	if req.Algorithm != "" {
		if err := validateAlgorithm(req.Algorithm); err != nil {
			ResponseError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	client := requireGRPCClient(w)
	if client == nil {
		return
	}
	resp, err := client.TriggerTrain(r.Context(), &pb.TriggerTrainRequest{Algorithm: req.Algorithm})
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			switch st.Code() {
			case codes.Aborted:
				ResponseError(w, http.StatusConflict, st.Message())
				return
			case codes.InvalidArgument:
				ResponseError(w, http.StatusBadRequest, st.Message())
				return
			}
		}
		logger.Logger.Errorf("Failed to start training: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to start training: "+err.Error())
		return
	}

	ResponseSuccess(w, http.StatusAccepted, TriggerTrainResponse{
		SessionID: resp.GetSessionId(),
		Message:   resp.GetMessage(),
	})
}
