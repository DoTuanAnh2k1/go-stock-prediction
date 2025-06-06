package server

import (
	"go-stock-prediction/pkg/logger"
	modelsapi "go-stock-prediction/pkg/models/models_api"
	"go-stock-prediction/pkg/service/predict"
	"net/http"
	"time"
)

func GetTrainingStatus(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Info("📊 Getting training status...")

	isTraining := predict.IsTraining()
	lastTrained := predict.GetLastTrainedTime()

	// Calculate next training (next Sunday 9 AM)
	now := time.Now()
	daysUntilSunday := (7 - int(now.Weekday())) % 7
	if daysUntilSunday == 0 && now.Hour() >= 9 {
		daysUntilSunday = 7
	}
	nextTraining := now.AddDate(0, 0, daysUntilSunday)
	nextTraining = time.Date(nextTraining.Year(), nextTraining.Month(), nextTraining.Day(), 9, 0, 0, 0, nextTraining.Location())

	status := &modelsapi.TrainingStatusDTO{
		IsTraining:   isTraining,
		LastTrained:  lastTrained,
		NextTraining: nextTraining,
		CurrentPhase: "idle",
		Progress:     0,
	}

	if isTraining {
		status.CurrentPhase = "training"
		status.Progress = 45.5 // Could track real progress
		status.EstimatedTime = "~20 minutes"
	}

	ResponseSuccess(w, http.StatusOK, status)
}
