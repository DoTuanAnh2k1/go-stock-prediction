package repository

import (
	"context"

	modelsapi "go-stock-prediction/pkg/models/models_api"
)

// MonitoringStore provides aggregated monitoring queries for the overview endpoint.
type MonitoringStore interface {
	// GetMarketCrawlStats returns crawl freshness stats for the given market.
	// market must be one of: GOLD, NASDAQ, CRYPTO, SP500.
	GetMarketCrawlStats(ctx context.Context, market string) (*modelsapi.MarketCrawlStats, error)

	// GetMarketPredStats returns per-algorithm prediction activity for today
	// for the given market.
	GetMarketPredStats(ctx context.Context, market string) ([]modelsapi.AlgoPredStats, error)
}
