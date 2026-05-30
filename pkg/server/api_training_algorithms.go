package server

import (
	"go-stock-prediction/pkg/logger"
	modelsapi "go-stock-prediction/pkg/models/models_api"
	"go-stock-prediction/pkg/store/repository"
	"net/http"
	"time"
)

// algorithmMeta contains static metadata for each known algorithm.
type algorithmMeta struct {
	displayName string
	config      map[string]interface{}
}

var knownAlgorithms = map[string]algorithmMeta{
	"lstm_nn": {
		displayName: "LSTM Neural Network",
		config: map[string]interface{}{
			"epochs":        100,
			"learning_rate": 0.001,
			"hidden_layers": 2,
			"batch_size":    32,
		},
	},
	"arima_garch": {
		displayName: "ARIMA-GARCH",
		config: map[string]interface{}{
			"p": 5,
			"d": 1,
			"q": 2,
		},
	},
	"moving_average": {
		displayName: "Moving Average (VWMA)",
		config: map[string]interface{}{
			"window": 20,
		},
	},
	"ensemble": {
		displayName: "Ensemble",
		config:      map[string]interface{}{},
	},
}

// GetTrainingAlgorithms handles GET /api/training/algorithms
// Returns per-algorithm status, config, accuracy, and prediction counts.
func GetTrainingAlgorithms(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Info("Getting training algorithms...")

	store := repository.GetSingleton()

	// Latest training log per algorithm
	latestLogs, err := store.GetLatestTrainingLogByAlgorithm()
	if err != nil {
		logger.Logger.Errorf("Failed to get latest training logs: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get training algorithm info")
		return
	}

	// Index latest logs by algorithm name
	logByAlg := make(map[string]interface{}) // just use map for presence check
	type algLogEntry struct {
		lastTrainedStr  string
		accuracyFloat   float64
		trainingTimeSec float64
	}
	logEntries := make(map[string]algLogEntry)
	for _, log := range latestLogs {
		acc, _ := log.Accuracy.Float64()
		logEntries[log.AlgorithmName] = algLogEntry{
			lastTrainedStr:  log.CompletedAt.UTC().Format(time.RFC3339),
			accuracyFloat:   acc / 100.0, // stored as percentage 0-100, convert to 0-1
			trainingTimeSec: float64(log.DurationMs) / 1000.0,
		}
		logByAlg[log.AlgorithmName] = struct{}{}
	}

	// Total prediction count per algorithm
	totalCounts, err := store.GetPredictionCountByAlgorithm()
	if err != nil {
		logger.Logger.Warnf("Failed to get prediction counts by algorithm: %v", err)
		totalCounts = map[string]int64{}
	}

	// Successful predictions (accuracy >= 0.95) per algorithm
	successCounts, err := store.GetSuccessfulPredictionCountByAlgorithm(0.95)
	if err != nil {
		logger.Logger.Warnf("Failed to get success prediction counts: %v", err)
		successCounts = map[string]int64{}
	}

	result := make([]modelsapi.TrainingAlgorithmDTO, 0, len(knownAlgorithms))
	for key, meta := range knownAlgorithms {
		dto := modelsapi.TrainingAlgorithmDTO{
			Name:             meta.displayName,
			Key:              key,
			Config:           meta.config,
			TotalPredictions: totalCounts[key],
		}

		// Calculate success rate
		total := totalCounts[key]
		successes := successCounts[key]
		if total > 0 {
			dto.SuccessRate = float64(successes) / float64(total)
		}

		entry, trained := logEntries[key]
		if trained {
			dto.Status = "trained"
			dto.LastTrained = &entry.lastTrainedStr
			dto.Accuracy = entry.accuracyFloat
			dto.TrainingTimeSeconds = entry.trainingTimeSec
		} else {
			dto.Status = "untrained"
		}

		result = append(result, dto)
		_ = logByAlg // suppress unused warning
	}

	ResponseSuccess(w, http.StatusOK, result)
}
