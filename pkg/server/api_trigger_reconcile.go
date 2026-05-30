package server

import (
	"context"
	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"
	"net/http"
	"time"
)

// TriggerReconcileHandler handles POST /api/trigger/reconcile
// It runs prediction reconciliation immediately (fills actual_price, accuracy, status).
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
