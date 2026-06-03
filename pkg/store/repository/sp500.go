package repository

import (
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"time"
)

// SP500PriceStore handles persistence for S&P 500 daily price records.
type SP500PriceStore interface {
	CreateSP500Price(p *modelsdb.SP500Price) error
	UpsertSP500Price(p *modelsdb.SP500Price) error
	GetSP500PricesByDateRange(symbol string, from, to time.Time) ([]modelsdb.SP500Price, error)
	GetLatestSP500Price(symbol string) (*modelsdb.SP500Price, error)
	GetSP500Symbols() ([]string, error)
	// GetAllSP500PricesForSymbol returns all historical prices for a symbol, ordered DESC.
	// Used by walk-forward backtest; caller must reverse to ASC before use.
	GetAllSP500PricesForSymbol(symbol string) ([]modelsdb.SP500Price, error)
}

// SP500PredictionStore handles persistence for S&P 500 prediction records.
type SP500PredictionStore interface {
	CreateSP500Prediction(p *modelsdb.SP500Prediction) error
	GetSP500Predictions(symbol, algorithm string, limit int) ([]modelsdb.SP500Prediction, error)
	GetLatestSP500Predictions() ([]modelsdb.SP500Prediction, error)
	GetSP500PredictionsByDateRange(symbol string, from, to time.Time) ([]modelsdb.SP500Prediction, error)
	GetSP500PredictionsPage(page, limit int, search, algorithm, status, sortBy, sortDir string) ([]modelsdb.SP500Prediction, int64, error)
	// BulkCreateSP500Predictions inserts multiple S&P 500 predictions in batches.
	BulkCreateSP500Predictions(preds []modelsdb.SP500Prediction) error
	// DeleteSP500PredictionsBeforeDate deletes all S&P 500 predictions whose target_date < cutoff.
	DeleteSP500PredictionsBeforeDate(before time.Time) error
	// GetLatestConfirmedSP500Predictions returns the most recent confirmed prediction
	// (actual_price IS NOT NULL) per (symbol, algorithm_name).
	GetLatestConfirmedSP500Predictions() ([]modelsdb.SP500Prediction, error)
}
