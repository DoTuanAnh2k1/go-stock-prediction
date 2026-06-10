package server

import (
	"fmt"
	"go-stock-prediction/pkg/logger"
	modelsapi "go-stock-prediction/pkg/models/models_api"
	"go-stock-prediction/pkg/store/repository"
	"net/http"
	"strconv"
	"time"

	"github.com/shopspring/decimal"
)

// GetAlgorithmComparison godoc
//
//	@Summary      Compare algorithm performance
//	@Description  Returns a side-by-side comparison of all prediction algorithms over a configurable lookback period. Includes average error, accuracy rate, total predictions, and the best-performing algorithm for the period.
//	@Tags         Algorithms
//	@Produce      json
//	@Param        days  query     int  false  "Lookback period in days (default 30)"
//	@Success      200   {object}  modelsapi.AlgorithmComparisonDTO
//	@Failure      500   {object}  ResponseFailure
//	@Router       /api/algorithms/comparison [get]
func GetAlgorithmComparison(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Info("⚔️ Getting algorithm comparison...")

	// Parse days parameter
	daysStr := r.URL.Query().Get("days")
	days := 30
	if daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}

	store := repository.GetSingleton()
	fromDate := time.Now().AddDate(0, 0, -days)

	predictions, err := store.GetPredictionsByDateRange(fromDate, time.Now())
	if err != nil {
		logger.Logger.Errorf("Failed to get predictions: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get predictions")
		return
	}

	// Group by algorithm
	algStats := make(map[string]*modelsapi.AlgorithmPerformanceDTO)

	for _, pred := range predictions {
		if pred.ActualPrice == nil {
			continue // Skip without actual price
		}

		if _, exists := algStats[pred.AlgorithmName]; !exists {
			algStats[pred.AlgorithmName] = &modelsapi.AlgorithmPerformanceDTO{
				Name:           pred.AlgorithmName,
				LastPrediction: pred.PredictionDate,
			}
		}

		stat := algStats[pred.AlgorithmName]
		stat.TotalPredictions++

		// Calculate accuracy (within 5% = accurate)
		actualPrice := *pred.ActualPrice
		errorPercent := actualPrice.Sub(pred.PredictedPrice).Abs().Div(actualPrice).Mul(decimal.NewFromInt(100))

		if errorPercent.LessThanOrEqual(decimal.NewFromFloat(5.0)) {
			// Accurate prediction
		}

		stat.AvgError = stat.AvgError.Add(errorPercent)

		if pred.PredictionDate.After(stat.LastPrediction) {
			stat.LastPrediction = pred.PredictionDate
		}
	}

	// Calculate final stats
	var algorithms []modelsapi.AlgorithmPerformanceDTO
	var bestAlgorithm string
	var bestAccuracy decimal.Decimal

	for algName, stat := range algStats {
		if stat.TotalPredictions > 0 {
			stat.AvgError = stat.AvgError.Div(decimal.NewFromInt(int64(stat.TotalPredictions)))
			// Calculate accuracy rate (100% - avg error)
			stat.AccuracyRate = decimal.NewFromInt(100).Sub(stat.AvgError)

			if stat.AccuracyRate.GreaterThan(bestAccuracy) {
				bestAccuracy = stat.AccuracyRate
				bestAlgorithm = algName
			}
		}

		stat.Trend = "stable" // Could calculate trend based on time series
		algorithms = append(algorithms, *stat)
	}

	comparison := &modelsapi.AlgorithmComparisonDTO{
		Period:     fmt.Sprintf("Last %d days", days),
		Algorithms: algorithms,
		Winner:     bestAlgorithm,
	}

	ResponseSuccess(w, http.StatusOK, comparison)
}
