package repository

import (
	"context"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"time"

	"github.com/shopspring/decimal"
)

type GoldPriceStore interface {
	GetGoldPrices(ctx context.Context, source, productType string, limit int) ([]modelsdb.GoldPrice, error)
	GetGoldPricesByDateRange(ctx context.Context, source, productType string, from, to time.Time) ([]modelsdb.GoldPrice, error)
	GetLatestGoldPrices(ctx context.Context) ([]modelsdb.GoldPrice, error)
	UpsertGoldPrice(ctx context.Context, price *modelsdb.GoldPrice) error
	BulkUpsertGoldPrices(ctx context.Context, prices []modelsdb.GoldPrice) error
}

type GoldPredictionStore interface {
	CreateGoldPrediction(ctx context.Context, pred *modelsdb.GoldPrediction) error
	GetGoldPredictions(ctx context.Context, source, productType, algorithm string, limit int) ([]modelsdb.GoldPrediction, error)
	GetLatestGoldPredictions(ctx context.Context) ([]modelsdb.GoldPrediction, error)
	GetGoldPredictionsByDateRange(ctx context.Context, source, productType string, from, to time.Time) ([]modelsdb.GoldPrediction, error)
	GetGoldPredictionsPage(ctx context.Context, page, limit int, search, algorithm, status, sortBy, sortDir string) ([]modelsdb.GoldPrediction, int64, error)
	// GetPendingGoldPredictions returns gold predictions where actual_price IS NULL and target_date <= cutoff.
	GetPendingGoldPredictions(ctx context.Context, cutoff time.Time) ([]modelsdb.GoldPrediction, error)
	// UpdateGoldPredictionActual sets actual_price and accuracy for a gold prediction.
	UpdateGoldPredictionActual(ctx context.Context, id uint, actual, accuracy *decimal.Decimal) error
	// DeleteGoldPredictionsBeforeDate deletes all gold predictions whose target_date < cutoff.
	// Used by gold historical backtest to clear stale data before re-inserting.
	DeleteGoldPredictionsBeforeDate(ctx context.Context, cutoff time.Time) error
	// BulkCreateGoldPredictions inserts multiple gold predictions in batches.
	BulkCreateGoldPredictions(ctx context.Context, preds []modelsdb.GoldPrediction) error
	// GetLatestConfirmedGoldPredictions returns the most recent confirmed prediction
	// (actual_price IS NOT NULL) per (source, product_type, algorithm_name).
	GetLatestConfirmedGoldPredictions(ctx context.Context) ([]modelsdb.GoldPrediction, error)
}

// GoldIntradayStore handles persistence for gold hourly intraday price records.
type GoldIntradayStore interface {
	UpsertGoldIntradayPrice(ctx context.Context, p *modelsdb.GoldIntradayPrice) error
	GetGoldIntradayByRange(ctx context.Context, source string, from, to time.Time) ([]modelsdb.GoldIntradayPrice, error)
}

type MacroIndicatorStore interface {
	GetMacroIndicators(ctx context.Context, name string, limit int) ([]modelsdb.MacroIndicator, error)
	GetLatestMacroIndicator(ctx context.Context, name string) (*modelsdb.MacroIndicator, error)
	UpsertMacroIndicator(ctx context.Context, indicator *modelsdb.MacroIndicator) error
}
