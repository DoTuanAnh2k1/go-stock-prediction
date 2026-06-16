package modelsapi

import "time"

// MarketCrawlStats holds crawl freshness data for a single market.
type MarketCrawlStats struct {
	LastDailyAt    *time.Time `json:"last_daily_at"`
	LastIntradayAt *time.Time `json:"last_intraday_at"`
	DailyToday     int64      `json:"daily_today"`
	IntradayToday  int64      `json:"intraday_today"`
}

// AlgoPredStats holds today's prediction count for a single (market, algorithm) pair.
type AlgoPredStats struct {
	AlgorithmName string     `json:"algorithm_name"`
	TodayCount    int64      `json:"today_count"`
	LastPredictAt *time.Time `json:"last_predict_at"`
}
