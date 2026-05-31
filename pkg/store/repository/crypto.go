package repository

import (
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"time"
)

// CryptoPriceStore handles persistence for cryptocurrency daily price records.
type CryptoPriceStore interface {
	CreateCryptoPrice(p *modelsdb.CryptoPrice) error
	UpsertCryptoPrice(p *modelsdb.CryptoPrice) error
	GetCryptoPricesByDateRange(coinID string, from, to time.Time) ([]modelsdb.CryptoPrice, error)
	GetLatestCryptoPrice(coinID string) (*modelsdb.CryptoPrice, error)
	// GetCryptoCoins returns one representative record per distinct coinID.
	GetCryptoCoins() ([]modelsdb.CryptoPrice, error)
}

// CryptoPredictionStore handles persistence for cryptocurrency prediction records.
type CryptoPredictionStore interface {
	CreateCryptoPrediction(p *modelsdb.CryptoPrediction) error
	GetCryptoPredictions(coinID, algorithm string, limit int) ([]modelsdb.CryptoPrediction, error)
	GetLatestCryptoPredictions() ([]modelsdb.CryptoPrediction, error)
	GetCryptoPredictionsByDateRange(coinID string, from, to time.Time) ([]modelsdb.CryptoPrediction, error)
	GetCryptoPredictionsPage(page, limit int, search, algorithm, status, sortBy, sortDir string) ([]modelsdb.CryptoPrediction, int64, error)
}
