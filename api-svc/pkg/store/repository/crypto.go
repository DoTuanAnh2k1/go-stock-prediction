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
	// GetAllCryptoPricesForCoin returns all historical prices for a coinID, ordered DESC.
	// Used by walk-forward backtest; caller must reverse to ASC before use.
	GetAllCryptoPricesForCoin(coinID string) ([]modelsdb.CryptoPrice, error)
}

// CryptoIntradayStore handles persistence for cryptocurrency hourly intraday price records.
type CryptoIntradayStore interface {
	UpsertCryptoIntradayPrice(p *modelsdb.CryptoIntradayPrice) error
	GetCryptoIntradayByRange(coinID string, from, to time.Time) ([]modelsdb.CryptoIntradayPrice, error)
}

// CryptoPredictionStore handles persistence for cryptocurrency prediction records.
type CryptoPredictionStore interface {
	CreateCryptoPrediction(p *modelsdb.CryptoPrediction) error
	GetCryptoPredictions(coinID, algorithm string, limit int) ([]modelsdb.CryptoPrediction, error)
	GetLatestCryptoPredictions() ([]modelsdb.CryptoPrediction, error)
	// GetLatestConfirmedCryptoPredictions returns the most recent CONFIRMED prediction
	// (actual_price IS NOT NULL) per (coin_id, algorithm_name).
	GetLatestConfirmedCryptoPredictions() ([]modelsdb.CryptoPrediction, error)
	GetCryptoPredictionsByDateRange(coinID string, from, to time.Time) ([]modelsdb.CryptoPrediction, error)
	GetCryptoPredictionsPage(page, limit int, search, algorithm, status, sortBy, sortDir string) ([]modelsdb.CryptoPrediction, int64, error)
	// BulkCreateCryptoPredictions inserts multiple crypto predictions in batches.
	BulkCreateCryptoPredictions(preds []modelsdb.CryptoPrediction) error
	// DeleteCryptoPredictionsBeforeDate deletes all crypto predictions whose target_date < cutoff.
	DeleteCryptoPredictionsBeforeDate(before time.Time) error
}
