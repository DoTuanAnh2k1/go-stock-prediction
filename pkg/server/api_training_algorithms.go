package server

import (
	"go-stock-prediction/pkg/logger"
	modelsapi "go-stock-prediction/pkg/models/models_api"
	"go-stock-prediction/pkg/service/predict/registry"
	"go-stock-prediction/pkg/store/repository"
	"net/http"
	"time"
)

// GetTrainingAlgorithms godoc
//
//	@Summary      List training algorithms
//	@Description  Returns per-algorithm details including status (trained/untrained), configuration, accuracy, training time, total predictions, and success rate. Sourced from the algorithm registry and latest training logs.
//	@Tags         Training
//	@Produce      json
//	@Success      200  {array}   modelsapi.TrainingAlgorithmDTO
//	@Failure      500  {object}  ResponseFailure
//	@Router       /api/training/algorithms [get]
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

	defs := registry.All()
	result := make([]modelsapi.TrainingAlgorithmDTO, 0, len(defs))
	for _, def := range defs {
		key := def.Key
		dto := modelsapi.TrainingAlgorithmDTO{
			Name:             def.DisplayName,
			Key:              key,
			Config:           def.Config,
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
	}

	ResponseSuccess(w, http.StatusOK, result)
}
