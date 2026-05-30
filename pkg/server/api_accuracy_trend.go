package server

import (
	"fmt"
	"go-stock-prediction/pkg/logger"
	modelsapi "go-stock-prediction/pkg/models/models_api"
	"go-stock-prediction/pkg/store/repository"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/shopspring/decimal"
)

// GetAccuracyTrend - GET /api/predictions/accuracy-trend
// Returns per-day accuracy rates for each algorithm over the last N days (default 30, max 365).
// Predictions with Accuracy >= 0.9 are counted as accurate.
// Result is cached for 120 seconds.
func GetAccuracyTrend(w http.ResponseWriter, r *http.Request) {
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

	cacheKey := fmt.Sprintf("accuracy_trend:%d", days)
	if cached, ok := globalCache.Get(cacheKey); ok {
		ResponseSuccess(w, http.StatusOK, cached)
		return
	}

	store := repository.GetSingleton()
	fromDate := time.Now().AddDate(0, 0, -days)

	predictions, _, err := store.GetPredictionsFiltered(nil, "", fromDate, time.Time{}, 0, 10000)
	if err != nil {
		logger.Logger.Errorf("GetAccuracyTrend: GetPredictionsFiltered failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get predictions")
		return
	}

	// key: "YYYY-MM-DD|algorithm_name"  →  {total, accurate}
	type dayAlgKey struct {
		date string
		alg  string
	}
	type tally struct {
		total    int
		accurate int
	}
	tallies := make(map[dayAlgKey]*tally)
	dateSet := make(map[string]struct{})

	for _, pred := range predictions {
		if pred.Accuracy == nil {
			continue
		}
		dateStr := pred.PredictionDate.Format("2006-01-02")
		key := dayAlgKey{date: dateStr, alg: pred.AlgorithmName}
		t, ok := tallies[key]
		if !ok {
			t = &tally{}
			tallies[key] = t
		}
		t.total++
		if pred.Accuracy.GreaterThanOrEqual(decimal.NewFromFloat(0.9)) {
			t.accurate++
		}
		dateSet[dateStr] = struct{}{}
	}

	// Collect and sort dates
	dates := make([]string, 0, len(dateSet))
	for d := range dateSet {
		dates = append(dates, d)
	}
	sort.Strings(dates) // ascending

	accRate := func(date, alg string) float64 {
		t, ok := tallies[dayAlgKey{date: date, alg: alg}]
		if !ok || t.total == 0 {
			return 0
		}
		rate := decimal.NewFromInt(int64(t.accurate)).
			Div(decimal.NewFromInt(int64(t.total))).
			Mul(decimal.NewFromInt(100))
		f, _ := rate.Float64()
		return f
	}

	result := make([]modelsapi.AccuracyTrendPointDTO, 0, len(dates))
	for _, d := range dates {
		result = append(result, modelsapi.AccuracyTrendPointDTO{
			Date:          d,
			LstmNN:        accRate(d, "lstm_nn"),
			ArimaGarch:    accRate(d, "arima_garch"),
			MovingAverage: accRate(d, "moving_average"),
		})
	}

	globalCache.Set(cacheKey, result, 120*time.Second)
	ResponseSuccess(w, http.StatusOK, result)
}
