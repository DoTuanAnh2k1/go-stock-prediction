package repository

import (
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"time"

	"github.com/shopspring/decimal"
)

type GoldPriceStore interface {
	GetGoldPrices(source, productType string, limit int) ([]modelsdb.GoldPrice, error)
	GetGoldPricesByDateRange(source, productType string, from, to time.Time) ([]modelsdb.GoldPrice, error)
	GetLatestGoldPrices() ([]modelsdb.GoldPrice, error)
	UpsertGoldPrice(price *modelsdb.GoldPrice) error
	BulkUpsertGoldPrices(prices []modelsdb.GoldPrice) error
}

type GoldPredictionStore interface {
	CreateGoldPrediction(pred *modelsdb.GoldPrediction) error
	GetGoldPredictions(source, productType, algorithm string, limit int) ([]modelsdb.GoldPrediction, error)
	GetLatestGoldPredictions() ([]modelsdb.GoldPrediction, error)
	GetGoldPredictionsByDateRange(source, productType string, from, to time.Time) ([]modelsdb.GoldPrediction, error)
	GetGoldPredictionsPage(page, limit int, search, algorithm, status, sortBy, sortDir string) ([]modelsdb.GoldPrediction, int64, error)
	// GetPendingGoldPredictions returns gold predictions where actual_price IS NULL and target_date <= cutoff.
	GetPendingGoldPredictions(cutoff time.Time) ([]modelsdb.GoldPrediction, error)
	// UpdateGoldPredictionActual sets actual_price and accuracy for a gold prediction.
	UpdateGoldPredictionActual(id uint, actual, accuracy *decimal.Decimal) error
	// DeleteGoldPredictionsBeforeDate deletes all gold predictions whose target_date < cutoff.
	// Used by gold historical backtest to clear stale data before re-inserting.
	DeleteGoldPredictionsBeforeDate(cutoff time.Time) error
	// BulkCreateGoldPredictions inserts multiple gold predictions in batches.
	BulkCreateGoldPredictions(preds []modelsdb.GoldPrediction) error
}

type MacroIndicatorStore interface {
	GetMacroIndicators(name string, limit int) ([]modelsdb.MacroIndicator, error)
	GetLatestMacroIndicator(name string) (*modelsdb.MacroIndicator, error)
	UpsertMacroIndicator(indicator *modelsdb.MacroIndicator) error
}
