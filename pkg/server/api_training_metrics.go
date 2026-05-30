package server

import (
	"fmt"
	"go-stock-prediction/pkg/logger"
	modelsapi "go-stock-prediction/pkg/models/models_api"
	"go-stock-prediction/pkg/store/repository"
	"net/http"
	"runtime"
)

// GetTrainingMetrics handles GET /api/training/metrics
// Returns aggregate training performance statistics.
func GetTrainingMetrics(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Info("Getting training metrics...")

	store := repository.GetSingleton()

	agg, err := store.GetTrainingMetricsAggregate()
	if err != nil {
		logger.Logger.Errorf("Failed to get training metrics aggregate: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get training metrics")
		return
	}

	// Format average training duration as human-readable string
	avgMs := agg.AvgDurationMs
	avgTrainingTime := formatDurationMs(avgMs)

	// Optional: read current process memory usage
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	memMB := memStats.Alloc / 1024 / 1024

	dto := &modelsapi.TrainingMetricsDTO{
		AvgTrainingTime:   avgTrainingTime,
		AvgTrainingTimeMs: avgMs,
		DataQuality:       agg.DataQuality,
		MemoryUsageMB:     memMB,
		SuccessRate:       agg.SuccessRate,
		TotalSessions:     agg.TotalSessions,
		TotalPredictions:  agg.TotalPredictions,
	}

	ResponseSuccess(w, http.StatusOK, dto)
}

// formatDurationMs converts milliseconds to a human-readable string like "5m 20s".
func formatDurationMs(ms float64) string {
	totalSec := int64(ms / 1000)
	if totalSec < 60 {
		return fmt.Sprintf("%ds", totalSec)
	}
	minutes := totalSec / 60
	seconds := totalSec % 60
	if minutes < 60 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	hours := minutes / 60
	minutes = minutes % 60
	return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
}
