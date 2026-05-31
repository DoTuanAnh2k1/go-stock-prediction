// Package market defines the unified extensibility point for all tradeable asset markets.
//
// To add a new market (e.g. crypto):
//  1. Create pkg/service/market/<name>/market.go implementing AssetMarket
//  2. Self-register via init(): market.Register(&YourMarket{})
//  3. Add blank import in cmd/prediction/main.go
//  4. Add DB model + repository methods for prices and predictions
//  5. Add crawler in pkg/service/crawler/ + register its cron job in crawler/init.go
//  6. Add gRPC trigger + API routes + frontend page
package market

import (
	"context"
	"time"
)

// Instrument represents a single tradeable asset within a market.
type Instrument struct {
	ID         string // unique within the market: stock symbol, "XAU/spot", "BTC/USDT"
	Name       string // human-readable name
	InternalID uint   // optional DB primary key (e.g. stock.ID); 0 if unused
}

// PredictionData holds one algorithm's output for an instrument.
type PredictionData struct {
	AlgorithmName  string
	PredictedPrice float64
	CurrentPrice   float64
	Confidence     float64
	TargetDate     time.Time
}

// AssetMarket is the single interface every market must implement.
// The prediction orchestrator uses it to drive daily predictions.
type AssetMarket interface {
	// MarketKey returns the unique identifier (e.g. "VN30", "GOLD", "CRYPTO").
	MarketKey() string
	// MarketName returns a human-readable display name.
	MarketName() string
	// GetInstruments returns all assets to predict for in this market.
	GetInstruments(ctx context.Context) ([]Instrument, error)
	// FetchPrices returns historical close/buy prices (oldest→newest) for the given
	// instrument over the last `months` months. Minimum 20 data points required.
	FetchPrices(ctx context.Context, inst Instrument, months int) ([]float64, error)
	// SavePrediction persists one algorithm's prediction for the given instrument.
	SavePrediction(ctx context.Context, inst Instrument, pred PredictionData) error
	// Crawl fetches and stores the latest prices for all instruments.
	// Return nil without doing anything if crawling is managed by a separate cron job.
	Crawl(ctx context.Context) error
}
