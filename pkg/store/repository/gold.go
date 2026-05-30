package repository

import (
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"time"
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
}

type MacroIndicatorStore interface {
	GetMacroIndicators(name string, limit int) ([]modelsdb.MacroIndicator, error)
	GetLatestMacroIndicator(name string) (*modelsdb.MacroIndicator, error)
	UpsertMacroIndicator(indicator *modelsdb.MacroIndicator) error
}
