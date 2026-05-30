package server

import (
	"go-stock-prediction/pkg/logger"
	modelsapi "go-stock-prediction/pkg/models/models_api"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"go-stock-prediction/pkg/store/repository"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

func GetTrainingHistory(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Info("Getting training history...")

	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 50 {
			limit = l
		}
	}

	// Optional ?level=error|info filter.
	// Since TrainingLog has no explicit level field, we infer:
	//   "error" — session has at least one log with non-empty ErrorDetails
	//   "info"  — all logs in session have empty ErrorDetails
	levelFilter := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("level")))

	store := repository.GetSingleton()
	logs, totalCount, err := store.GetTrainingSessions(limit, 0)
	if err != nil {
		logger.Logger.Errorf("Failed to get training history: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get training history")
		return
	}

	// Group logs by session_id, preserving order
	sessionMap := make(map[string][]modelsdb.TrainingLog)
	sessionOrder := []string{}
	for _, log := range logs {
		if _, exists := sessionMap[log.SessionID]; !exists {
			sessionOrder = append(sessionOrder, log.SessionID)
		}
		sessionMap[log.SessionID] = append(sessionMap[log.SessionID], log)
	}

	var sessions []modelsapi.TrainingSessionDTO
	var totalAccuracy decimal.Decimal
	validSessions := 0
	oneWeekAgo := time.Now().AddDate(0, 0, -7)
	lastWeekCount := 0

	for _, sessionID := range sessionOrder {
		algLogs := sessionMap[sessionID]
		if len(algLogs) == 0 {
			continue
		}

		// Apply level filter based on inferred log level
		if levelFilter != "" {
			hasErrors := sessionHasErrors(algLogs)
			if levelFilter == "error" && !hasErrors {
				continue
			}
			if levelFilter == "info" && hasErrors {
				continue
			}
		}

		session := buildTrainingSessionDTOFromLogs(algLogs)

		totalAccuracy = totalAccuracy.Add(session.OverallAccuracy)
		validSessions++

		if session.StartTime.After(oneWeekAgo) {
			lastWeekCount++
		}

		sessions = append(sessions, session)
	}

	var avgAccuracy decimal.Decimal
	if validSessions > 0 {
		avgAccuracy = totalAccuracy.Div(decimal.NewFromInt(int64(validSessions)))
	}

	history := &modelsapi.TrainingHistoryDTO{
		Sessions:    sessions,
		Total:       int(totalCount),
		AvgAccuracy: avgAccuracy,
		LastWeek:    lastWeekCount,
	}

	ResponseSuccess(w, http.StatusOK, history)
}

// sessionHasErrors returns true when at least one log entry in the session
// has non-empty ErrorDetails, indicating an error-level event.
func sessionHasErrors(logs []modelsdb.TrainingLog) bool {
	for _, l := range logs {
		if strings.TrimSpace(l.ErrorDetails) != "" {
			return true
		}
	}
	return false
}
