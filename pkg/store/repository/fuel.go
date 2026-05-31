package repository

import (
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"time"
)

// FuelPriceStore handles persistence for Vietnamese retail fuel price records.
type FuelPriceStore interface {
	CreateFuelPrice(p *modelsdb.FuelPrice) error
	UpsertFuelPrice(p *modelsdb.FuelPrice) error
	BulkUpsertFuelPrices(prices []modelsdb.FuelPrice) error
	GetFuelPricesByDateRange(productType string, from, to time.Time) ([]modelsdb.FuelPrice, error)
	GetLatestFuelPrice(productType string) (*modelsdb.FuelPrice, error)
	GetFuelProducts() ([]string, error)
}

// FuelPredictionStore handles persistence for Vietnamese fuel prediction records.
type FuelPredictionStore interface {
	CreateFuelPrediction(p *modelsdb.FuelPrediction) error
	GetFuelPredictions(productType, algorithm string, limit int) ([]modelsdb.FuelPrediction, error)
	GetLatestFuelPredictions() ([]modelsdb.FuelPrediction, error)
	GetFuelPredictionsByDateRange(productType string, from, to time.Time) ([]modelsdb.FuelPrediction, error)
	GetFuelPredictionsPage(page, limit int, search, algorithm, status, sortBy, sortDir string) ([]modelsdb.FuelPrediction, int64, error)
}
