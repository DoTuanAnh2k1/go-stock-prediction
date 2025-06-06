package server

import (
	"go-stock-prediction/pkg/logger"
	modelsapi "go-stock-prediction/pkg/models/models_api"
	"go-stock-prediction/pkg/store/repository"
	"net/http"
	"strconv"
	"time"

	"github.com/shopspring/decimal"
)

func GetTrainingHistory(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Info("📜 Getting training history...")

	// Parse limit
	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	store := repository.GetSingleton()
	syncLogs, err := store.GetSyncLogsBySource("ML Training")
	if err != nil {
		logger.Logger.Errorf("Failed to get training history: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get training history")
		return
	}

	// Convert to DTOs
	var sessions []modelsapi.TrainingSessionDTO
	var totalAccuracy decimal.Decimal
	validSessions := 0
	lastWeekCount := 0
	oneWeekAgo := time.Now().AddDate(0, 0, -7)

	for i, log := range syncLogs {
		if i >= limit {
			break
		}

		session := modelsapi.TrainingSessionDTO{
			ID:           log.ID,
			StartTime:    log.SyncDate,
			Duration:     log.DurationMs,
			TotalStocks:  30, // VN30
			SuccessCount: log.SuccessCount,
			ErrorCount:   log.ErrorCount,
			Status:       "success",
		}

		if log.ErrorCount > 0 {
			if log.SuccessCount > 0 {
				session.Status = "partial"
			} else {
				session.Status = "failed"
			}
		}

		// Mock algorithm results (you'd parse from log.ErrorMessage in real app)
		session.Algorithms = []modelsapi.AlgorithmResultDTO{
			{Name: "moving_average", SuccessCount: 25, ErrorCount: 5, Accuracy: decimal.NewFromFloat(75.5), Duration: 5000},
			{Name: "lstm_nn", SuccessCount: 28, ErrorCount: 2, Accuracy: decimal.NewFromFloat(89.2), Duration: 8000},
			{Name: "arima_garch", SuccessCount: 23, ErrorCount: 7, Accuracy: decimal.NewFromFloat(72.1), Duration: 12000},
		}

		// Calculate overall accuracy
		var algAccuracy decimal.Decimal
		for _, alg := range session.Algorithms {
			algAccuracy = algAccuracy.Add(alg.Accuracy)
		}
		session.OverallAccuracy = algAccuracy.Div(decimal.NewFromInt(3))

		if validSessions == 0 || !session.OverallAccuracy.IsZero() {
			totalAccuracy = totalAccuracy.Add(session.OverallAccuracy)
			validSessions++
		}

		if session.StartTime.After(oneWeekAgo) {
			lastWeekCount++
		}

		sessions = append(sessions, session)
	}

	// Calculate average accuracy
	var avgAccuracy decimal.Decimal
	if validSessions > 0 {
		avgAccuracy = totalAccuracy.Div(decimal.NewFromInt(int64(validSessions)))
	}

	history := &modelsapi.TrainingHistoryDTO{
		Sessions:    sessions,
		Total:       len(sessions),
		AvgAccuracy: avgAccuracy,
		LastWeek:    lastWeekCount,
	}

	ResponseSuccess(w, http.StatusOK, history)
}
