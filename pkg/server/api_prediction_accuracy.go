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

func GetPredictionAccuracy(w http.ResponseWriter, r *http.Request) {
	daysStr := r.URL.Query().Get("days")
	days := 30
	if daysStr != "" {
		d, err := strconv.Atoi(daysStr)
		if err != nil || d < 1 || d > 365 {
			ResponseError(w, http.StatusBadRequest, "days must be between 1 and 365")
			return
		}
		days = d
	}

	store := repository.GetSingleton()
	fromDate := time.Now().AddDate(0, 0, -days)

	// Fetch all predictions in the date range (no stock/algo filter, large limit)
	predictions, _, err := store.GetPredictionsFiltered(nil, "", fromDate, time.Time{}, 0, 10000)
	if err != nil {
		logger.Logger.Errorf("Failed to get predictions for accuracy: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get predictions")
		return
	}

	type algStats struct {
		total    int
		accurate int
		totalErr decimal.Decimal
	}
	stats := make(map[string]*algStats)

	for _, pred := range predictions {
		if pred.Accuracy == nil {
			continue
		}
		s, ok := stats[pred.AlgorithmName]
		if !ok {
			s = &algStats{}
			stats[pred.AlgorithmName] = s
		}
		s.total++
		// Treat >= 90% accuracy as "accurate"
		if pred.Accuracy.GreaterThanOrEqual(decimal.NewFromFloat(0.9)) {
			s.accurate++
		}
		// Average error = |predicted - actual| / actual * 100
		if pred.ActualPrice != nil && pred.ActualPrice.GreaterThan(decimal.Zero) {
			errPct := pred.PredictedPrice.Sub(*pred.ActualPrice).Abs().
				Div(*pred.ActualPrice).
				Mul(decimal.NewFromInt(100))
			s.totalErr = s.totalErr.Add(errPct)
		}
	}

	period := fmt.Sprintf("%dd", days)
	result := make([]modelsapi.PredictionAccuracyDTO, 0)

	for algName, s := range stats {
		var accuracyRate, avgErr decimal.Decimal
		if s.total > 0 {
			accuracyRate = decimal.NewFromInt(int64(s.accurate)).
				Div(decimal.NewFromInt(int64(s.total))).
				Mul(decimal.NewFromInt(100))
			avgErr = s.totalErr.Div(decimal.NewFromInt(int64(s.total)))
		}
		result = append(result, modelsapi.PredictionAccuracyDTO{
			AlgorithmName:       algName,
			Period:              period,
			TotalPredictions:    s.total,
			AccuratePredictions: s.accurate,
			AccuracyRate:        accuracyRate,
			AvgError:            avgErr,
		})
	}

	ResponseSuccess(w, http.StatusOK, result)
}
