package modelsapi

import "time"

// MarketCrawlStats holds crawl freshness data for a single market.
type MarketCrawlStats struct {
	LastDailyAt    *time.Time
	LastIntradayAt *time.Time
	DailyToday     int64
	IntradayToday  int64
}

// AlgoPredStats holds today's prediction count for a single (market, algorithm) pair.
type AlgoPredStats struct {
	AlgorithmName string
	TodayCount    int64
	LastPredictAt *time.Time
}
