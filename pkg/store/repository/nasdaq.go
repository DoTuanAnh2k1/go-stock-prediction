package repository

import (
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"time"
)

// NasdaqPriceStore handles persistence for NASDAQ 100 daily price records.
type NasdaqPriceStore interface {
	CreateNasdaqPrice(p *modelsdb.NasdaqPrice) error
	UpsertNasdaqPrice(p *modelsdb.NasdaqPrice) error
	GetNasdaqPricesByDateRange(symbol string, from, to time.Time) ([]modelsdb.NasdaqPrice, error)
	GetLatestNasdaqPrice(symbol string) (*modelsdb.NasdaqPrice, error)
	GetNasdaqSymbols() ([]string, error)
	// GetAllNasdaqPricesForSymbol returns all historical prices for a symbol, ordered DESC.
	// Used by walk-forward backtest; caller must reverse to ASC before use.
	GetAllNasdaqPricesForSymbol(symbol string) ([]modelsdb.NasdaqPrice, error)
}

// NasdaqPredictionStore handles persistence for NASDAQ 100 prediction records.
type NasdaqPredictionStore interface {
	CreateNasdaqPrediction(p *modelsdb.NasdaqPrediction) error
	GetNasdaqPredictions(symbol, algorithm string, limit int) ([]modelsdb.NasdaqPrediction, error)
	GetLatestNasdaqPredictions() ([]modelsdb.NasdaqPrediction, error)
	GetNasdaqPredictionsByDateRange(symbol string, from, to time.Time) ([]modelsdb.NasdaqPrediction, error)
	GetNasdaqPredictionsPage(page, limit int, search, algorithm, status, sortBy, sortDir string) ([]modelsdb.NasdaqPrediction, int64, error)
	// BulkCreateNasdaqPredictions inserts multiple NASDAQ predictions in batches.
	BulkCreateNasdaqPredictions(preds []modelsdb.NasdaqPrediction) error
	// DeleteNasdaqPredictionsBeforeDate deletes all NASDAQ predictions whose target_date < cutoff.
	DeleteNasdaqPredictionsBeforeDate(before time.Time) error
	// GetLatestConfirmedNasdaqPredictions returns the most recent confirmed prediction
	// (actual_price IS NOT NULL) per (symbol, algorithm_name).
	GetLatestConfirmedNasdaqPredictions() ([]modelsdb.NasdaqPrediction, error)
}
