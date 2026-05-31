package server

import (
	"context"
	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"
	"net/http"
	"time"
)

// TriggerReconcileHandler godoc
//
//	@Summary      Trigger prediction reconciliation
//	@Description  Runs prediction reconciliation immediately, filling in actual_price, accuracy, and status for past predictions. Runs synchronously with a 10-minute timeout.
//	@Tags         Triggers
//	@Accept       json
//	@Produce      json
//	@Success      200  {object}  map[string]string
//	@Failure      401  {object}  ResponseFailure
//	@Failure      500  {object}  ResponseFailure
//	@Failure      503  {object}  ResponseFailure
//	@Security     BearerAuth
//	@Router       /api/trigger/reconcile [post]
func TriggerReconcileHandler(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Info("TriggerReconcileHandler: manual reconcile triggered")

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	client := requireGRPCClient(w)
	if client == nil {
		return
	}
	if _, err := client.TriggerReconcile(ctx, &pb.Empty{}); err != nil {
		logger.Logger.Errorf("TriggerReconcileHandler: reconcile failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}

	ResponseSuccess(w, http.StatusOK, map[string]string{"status": "ok"})
}
