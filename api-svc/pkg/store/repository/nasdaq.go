package repository

import (
	"context"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"time"
)

// NasdaqPriceStore handles persistence for NASDAQ 100 daily price records.
type NasdaqPriceStore interface {
	CreateNasdaqPrice(ctx context.Context, p *modelsdb.NasdaqPrice) error
	UpsertNasdaqPrice(ctx context.Context, p *modelsdb.NasdaqPrice) error
	GetNasdaqPricesByDateRange(ctx context.Context, symbol string, from, to time.Time) ([]modelsdb.NasdaqPrice, error)
	GetLatestNasdaqPrice(ctx context.Context, symbol string) (*modelsdb.NasdaqPrice, error)
	GetNasdaqSymbols(ctx context.Context) ([]string, error)
	// GetAllNasdaqPricesForSymbol returns all historical prices for a symbol, ordered DESC.
	// Used by walk-forward backtest; caller must reverse to ASC before use.
	GetAllNasdaqPricesForSymbol(ctx context.Context, symbol string) ([]modelsdb.NasdaqPrice, error)
}

// NasdaqIntradayStore handles persistence for NASDAQ 100 hourly intraday price records.
type NasdaqIntradayStore interface {
	UpsertNasdaqIntradayPrice(ctx context.Context, p *modelsdb.NasdaqIntradayPrice) error
	GetNasdaqIntradayByRange(ctx context.Context, symbol string, from, to time.Time) ([]modelsdb.NasdaqIntradayPrice, error)
}

// NasdaqPredictionStore handles persistence for NASDAQ 100 prediction records.
type NasdaqPredictionStore interface {
	CreateNasdaqPrediction(ctx context.Context, p *modelsdb.NasdaqPrediction) error
	GetNasdaqPredictions(ctx context.Context, symbol, algorithm string, limit int) ([]modelsdb.NasdaqPrediction, error)
	GetLatestNasdaqPredictions(ctx context.Context) ([]modelsdb.NasdaqPrediction, error)
	GetNasdaqPredictionsByDateRange(ctx context.Context, symbol string, from, to time.Time) ([]modelsdb.NasdaqPrediction, error)
	GetNasdaqPredictionsPage(ctx context.Context, page, limit int, search, algorithm, status, sortBy, sortDir string) ([]modelsdb.NasdaqPrediction, int64, error)
	// BulkCreateNasdaqPredictions inserts multiple NASDAQ predictions in batches.
	BulkCreateNasdaqPredictions(ctx context.Context, preds []modelsdb.NasdaqPrediction) error
	// DeleteNasdaqPredictionsBeforeDate deletes all NASDAQ predictions whose target_date < cutoff.
	DeleteNasdaqPredictionsBeforeDate(ctx context.Context, before time.Time) error
	// GetLatestConfirmedNasdaqPredictions returns the most recent confirmed prediction
	// (actual_price IS NOT NULL) per (symbol, algorithm_name).
	GetLatestConfirmedNasdaqPredictions(ctx context.Context) ([]modelsdb.NasdaqPrediction, error)
}
