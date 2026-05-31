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
}

// NasdaqPredictionStore handles persistence for NASDAQ 100 prediction records.
type NasdaqPredictionStore interface {
	CreateNasdaqPrediction(p *modelsdb.NasdaqPrediction) error
	GetNasdaqPredictions(symbol, algorithm string, limit int) ([]modelsdb.NasdaqPrediction, error)
	GetLatestNasdaqPredictions() ([]modelsdb.NasdaqPrediction, error)
	GetNasdaqPredictionsByDateRange(symbol string, from, to time.Time) ([]modelsdb.NasdaqPrediction, error)
	GetNasdaqPredictionsPage(page, limit int, search, algorithm, status, sortBy, sortDir string) ([]modelsdb.NasdaqPrediction, int64, error)
}
