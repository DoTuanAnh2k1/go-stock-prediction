package server

import (
	"go-stock-prediction/pkg/logger"
	modelsapi "go-stock-prediction/pkg/models/models_api"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"go-stock-prediction/pkg/store/repository"
	"net/http"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
)

func GetTrainingDetail(w http.ResponseWriter, r *http.Request) {
	// Extract id from path /api/training/{id}
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	// pathParts: ["api", "training", "{id}"]
	if len(pathParts) < 3 {
		ResponseError(w, http.StatusBadRequest, "Invalid path")
		return
	}
	idStr := pathParts[2]

	if idStr == "" {
		ResponseError(w, http.StatusBadRequest, "Missing training ID")
		return
	}

	store := repository.GetSingleton()

	// Try as numeric ID first
	if id, err := strconv.ParseUint(idStr, 10, 64); err == nil {
		log, err := store.GetTrainingLogByID(uint(id))
		if err != nil {
			logger.Logger.Errorf("Training log not found id=%d: %v", id, err)
			ResponseError(w, http.StatusNotFound, "Training log not found")
			return
		}

		// Get all logs for this session
		logs, err := store.GetTrainingLogsBySessionID(log.SessionID)
		if err != nil {
			logger.Logger.Errorf("Failed to get session details for session=%s: %v", log.SessionID, err)
			ResponseError(w, http.StatusInternalServerError, "Failed to get session details")
			return
		}

		session := buildTrainingSessionDTOFromLogs(logs)
		ResponseSuccess(w, http.StatusOK, session)
		return
	}

	// Try as session ID (UUID string)
	logs, err := store.GetTrainingLogsBySessionID(idStr)
	if err != nil || len(logs) == 0 {
		logger.Logger.Errorf("Training session not found session_id=%s: %v", idStr, err)
		ResponseError(w, http.StatusNotFound, "Training session not found")
		return
	}

	session := buildTrainingSessionDTOFromLogs(logs)
	ResponseSuccess(w, http.StatusOK, session)
}

// buildTrainingSessionDTOFromLogs aggregates a slice of TrainingLog records (all belonging
// to the same session) into a single TrainingSessionDTO. It is also used by GetTrainingHistory.
func buildTrainingSessionDTOFromLogs(logs []modelsdb.TrainingLog) modelsapi.TrainingSessionDTO {
	if len(logs) == 0 {
		return modelsapi.TrainingSessionDTO{}
	}

	firstLog := logs[0]
	session := modelsapi.TrainingSessionDTO{
		ID:          firstLog.ID,
		StartTime:   firstLog.StartedAt,
		TotalStocks: firstLog.TotalStocks,
		Status:      "success",
	}

	var totalDuration int64
	var totalSuccess, totalErrors int
	var algAccuracy decimal.Decimal
	var algResults []modelsapi.AlgorithmResultDTO

	for _, log := range logs {
		totalDuration += log.DurationMs
		totalSuccess += log.SuccessCount
		totalErrors += log.ErrorCount
		algAccuracy = algAccuracy.Add(log.Accuracy)

		algResults = append(algResults, modelsapi.AlgorithmResultDTO{
			Name:         log.AlgorithmName,
			SuccessCount: log.SuccessCount,
			ErrorCount:   log.ErrorCount,
			Accuracy:     log.Accuracy,
			Duration:     log.DurationMs,
		})

		// Use the earliest StartedAt across all algorithm logs as session start time
		if log.StartedAt.Before(session.StartTime) {
			session.StartTime = log.StartedAt
		}
	}

	session.Duration = totalDuration
	session.SuccessCount = totalSuccess
	session.ErrorCount = totalErrors
	session.Algorithms = algResults

	session.OverallAccuracy = algAccuracy.Div(decimal.NewFromInt(int64(len(logs))))

	if totalErrors > 0 {
		if totalSuccess > 0 {
			session.Status = "partial"
		} else {
			session.Status = "failed"
		}
	}

	return session
}
