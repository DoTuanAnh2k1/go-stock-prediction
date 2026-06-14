package repository

import modelsapi "go-stock-prediction/pkg/models/models_api"

// MonitoringStore provides aggregated monitoring queries for the overview endpoint.
type MonitoringStore interface {
	// GetMarketCrawlStats returns crawl freshness stats for the given market.
	// market must be one of: GOLD, NASDAQ, CRYPTO, SP500.
	GetMarketCrawlStats(market string) (*modelsapi.MarketCrawlStats, error)

	// GetMarketPredStats returns per-algorithm prediction activity for today
	// for the given market.
	GetMarketPredStats(market string) ([]modelsapi.AlgoPredStats, error)
}
