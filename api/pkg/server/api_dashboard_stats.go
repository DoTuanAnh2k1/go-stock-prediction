package server

import (
	"fmt"
	"go-stock-prediction/pkg/logger"
	modelsapi "go-stock-prediction/pkg/models/models_api"
	"go-stock-prediction/pkg/store/repository"
	"net/http"
	"time"

	"github.com/shopspring/decimal"
)

// GetDashboardStats godoc
//
//	@Summary      Get dashboard statistics
//	@Description  Returns real-time dashboard statistics: total stock count, total prediction count, per-algorithm accuracy (last 30 days), last crawl timestamp, and last prediction timestamp. Response is cached for 60 seconds.
//	@Tags         Dashboard
//	@Produce      json
//	@Success      200  {object}  modelsapi.DashboardStatsFullDTO
//	@Failure      500  {object}  ResponseFailure
//	@Router       /api/dashboard/stats [get]
func GetDashboardStats(w http.ResponseWriter, r *http.Request) {
	const cacheKey = "dashboard:stats"

	if cached, ok := globalCache.Get(cacheKey); ok {
		ResponseSuccess(w, http.StatusOK, cached)
		return
	}

	store := repository.GetSingleton()

	// --- counts ---
	totalStocks, err := store.CountStocks()
	if err != nil {
		logger.Logger.Errorf("GetDashboardStats: CountStocks failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to count stocks")
		return
	}

	totalPredictions, err := store.CountPredictions()
	if err != nil {
		logger.Logger.Errorf("GetDashboardStats: CountPredictions failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to count predictions")
		return
	}

	// --- per-algorithm accuracy (last 30 days) ---
	fromDate := time.Now().AddDate(0, 0, -30)
	predictions, _, err := store.GetPredictionsFiltered(nil, "", fromDate, time.Time{}, 0, 10000)
	if err != nil {
		logger.Logger.Errorf("GetDashboardStats: GetPredictionsFiltered failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get predictions")
		return
	}

	type algStats struct {
		total    int
		accurate int
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
		if pred.Accuracy.GreaterThanOrEqual(decimal.NewFromFloat(0.9)) {
			s.accurate++
		}
	}

	algorithms := make(map[string]modelsapi.AlgorithmStatsEntry)
	var totalAccRate decimal.Decimal
	algCount := 0

	for algName, s := range stats {
		var accRate decimal.Decimal
		if s.total > 0 {
			accRate = decimal.NewFromInt(int64(s.accurate)).
				Div(decimal.NewFromInt(int64(s.total))).
				Mul(decimal.NewFromInt(100))
		}
		algorithms[algName] = modelsapi.AlgorithmStatsEntry{
			Accuracy:    fmt.Sprintf("%.1f", mustFloat(accRate)),
			Predictions: s.total,
		}
		totalAccRate = totalAccRate.Add(accRate)
		algCount++
	}

	var avgAccuracy string
	if algCount > 0 {
		avg := totalAccRate.Div(decimal.NewFromInt(int64(algCount)))
		avgAccuracy = fmt.Sprintf("%.1f", mustFloat(avg))
	} else {
		avgAccuracy = "0.0"
	}

	// --- last crawl ---
	var lastCrawl *time.Time
	syncLogs, err := store.GetLatestSyncLogs(1)
	if err == nil && len(syncLogs) > 0 {
		t := syncLogs[0].SyncDate
		lastCrawl = &t
	}

	// --- last prediction: take the newest PredictionDate from the recent batch ---
	var lastPrediction *time.Time
	if len(predictions) > 0 {
		// predictions are returned in any order; find the max PredictionDate
		latest := predictions[0].PredictionDate
		for _, p := range predictions[1:] {
			if p.PredictionDate.After(latest) {
				latest = p.PredictionDate
			}
		}
		lastPrediction = &latest
	}

	result := &modelsapi.DashboardStatsFullDTO{
		TotalStocks:      totalStocks,
		TotalPredictions: totalPredictions,
		AvgAccuracy:      avgAccuracy,
		LastCrawl:        lastCrawl,
		LastPrediction:   lastPrediction,
		Algorithms:       algorithms,
	}

	globalCache.Set(cacheKey, result, 60*time.Second)
	ResponseSuccess(w, http.StatusOK, result)
}

// mustFloat converts a decimal.Decimal to float64, returning 0 on overflow/NaN.
func mustFloat(d decimal.Decimal) float64 {
	f, _ := d.Float64()
	return f
}
