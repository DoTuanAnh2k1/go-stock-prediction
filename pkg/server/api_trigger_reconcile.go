package server

import (
	"context"
	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/service/predict"
	"net/http"
	"time"
)

// TriggerReconcileHandler handles POST /api/trigger/reconcile
// It runs prediction reconciliation immediately (fills actual_price, accuracy, status).
func TriggerReconcileHandler(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Info("TriggerReconcileHandler: manual reconcile triggered")

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	if err := predict.ReconcilePredictions(ctx); err != nil {
		logger.Logger.Errorf("TriggerReconcileHandler: reconcile failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}

	ResponseSuccess(w, http.StatusOK, map[string]string{"status": "ok"})
}
