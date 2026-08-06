package server

import (
	"net/http"
	"time"

	modelsapi "go-stock-prediction/pkg/models/models_api"
	"go-stock-prediction/pkg/store/repository"
)

// GetDashboardStats godoc
//
//	@Summary      Get dashboard statistics
//	@Description  Returns real-time dashboard statistics: last crawl timestamp. Response is cached for 60 seconds.
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
	var lastCrawl *time.Time
	if syncLogs, err := store.GetLatestSyncLogs(r.Context(), 1); err == nil && len(syncLogs) > 0 {
		t := syncLogs[0].SyncDate
		lastCrawl = &t
	}
	result := &modelsapi.DashboardStatsFullDTO{
		TotalStocks:      0,
		TotalPredictions: 0,
		AvgAccuracy:      "0.0",
		LastCrawl:        lastCrawl,
		LastPrediction:   nil,
		Algorithms:       map[string]modelsapi.AlgorithmStatsEntry{},
	}
	globalCache.Set(cacheKey, result, 60*time.Second)
	ResponseSuccess(w, http.StatusOK, result)
}
