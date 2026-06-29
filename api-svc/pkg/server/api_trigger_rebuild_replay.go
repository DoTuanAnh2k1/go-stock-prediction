package server

import (
	"encoding/json"
	"net/http"

	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RebuildReplayRequest is the JSON body accepted by TriggerRebuildReplayHandler.
type RebuildReplayRequest struct {
	// CutoffDate is an ISO date string "YYYY-MM-DD". Only predictions with
	// target_date > cutoff_date are kept after the wipe. Empty → server default
	// (2026-06-13).
	CutoffDate string `json:"cutoff_date"`
	// StepSize is the walk-forward step in days. 0 → server default (1).
	StepSize int32 `json:"step_size"`
}

// RebuildReplayResponse is the JSON body returned on 202.
type RebuildReplayResponse struct {
	Message string `json:"message"`
}

// TriggerRebuildReplayHandler godoc
//
//	@Summary      Trigger rebuild & replay
//	@Description  Wipes all predictions and simulation data, then replays a walk-forward backtest from cutoff_date+1 → today (out-of-sample), retrains meta-models, and replays bot simulation. Runs in the background; returns 202 immediately. Returns 409 if a rebuild-replay is already in progress, 400 for invalid arguments, 503 if the prediction service is unavailable.
//	@Tags         Triggers
//	@Accept       json
//	@Produce      json
//	@Param        body  body      RebuildReplayRequest   false  "Rebuild-replay parameters (both fields optional)"
//	@Success      202   {object}  RebuildReplayResponse
//	@Failure      400   {object}  ResponseFailure
//	@Failure      401   {object}  ResponseFailure
//	@Failure      409   {object}  ResponseFailure
//	@Failure      500   {object}  ResponseFailure
//	@Failure      503   {object}  ResponseFailure
//	@Security     BearerAuth
//	@Router       /api/trigger/rebuild-replay [post]
func TriggerRebuildReplayHandler(w http.ResponseWriter, r *http.Request) {
	var req RebuildReplayRequest
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req) // both fields are optional; ignore decode errors
	}

	logger.Logger.Infof("Trigger rebuild-replay request: cutoff_date=%q step_size=%d", req.CutoffDate, req.StepSize)

	client := requireGRPCClient(w)
	if client == nil {
		return
	}

	resp, err := client.TriggerRebuildReplay(r.Context(), &pb.RebuildReplayRequest{
		CutoffDate: req.CutoffDate,
		StepSize:   req.StepSize,
	})
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
		logger.Logger.Errorf("Failed to trigger rebuild-replay: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to trigger rebuild-replay: "+err.Error())
		return
	}

	ResponseSuccess(w, http.StatusAccepted, RebuildReplayResponse{
		Message: resp.GetMessage(),
	})
}
