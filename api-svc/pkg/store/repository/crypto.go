package repository

import (
	"context"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"time"
)

// CryptoPriceStore handles persistence for cryptocurrency daily price records.
type CryptoPriceStore interface {
	CreateCryptoPrice(ctx context.Context, p *modelsdb.CryptoPrice) error
	UpsertCryptoPrice(ctx context.Context, p *modelsdb.CryptoPrice) error
	GetCryptoPricesByDateRange(ctx context.Context, coinID string, from, to time.Time) ([]modelsdb.CryptoPrice, error)
	GetLatestCryptoPrice(ctx context.Context, coinID string) (*modelsdb.CryptoPrice, error)
	// GetCryptoCoins returns one representative record per distinct coinID.
	GetCryptoCoins(ctx context.Context) ([]modelsdb.CryptoPrice, error)
	// GetAllCryptoPricesForCoin returns all historical prices for a coinID, ordered DESC.
	// Used by walk-forward backtest; caller must reverse to ASC before use.
	GetAllCryptoPricesForCoin(ctx context.Context, coinID string) ([]modelsdb.CryptoPrice, error)
}

// CryptoIntradayStore handles persistence for cryptocurrency hourly intraday price records.
type CryptoIntradayStore interface {
	UpsertCryptoIntradayPrice(ctx context.Context, p *modelsdb.CryptoIntradayPrice) error
	GetCryptoIntradayByRange(ctx context.Context, coinID string, from, to time.Time) ([]modelsdb.CryptoIntradayPrice, error)
}

// CryptoPredictionStore handles persistence for cryptocurrency prediction records.
type CryptoPredictionStore interface {
	CreateCryptoPrediction(ctx context.Context, p *modelsdb.CryptoPrediction) error
	GetCryptoPredictions(ctx context.Context, coinID, algorithm string, limit int) ([]modelsdb.CryptoPrediction, error)
	GetLatestCryptoPredictions(ctx context.Context) ([]modelsdb.CryptoPrediction, error)
	// GetLatestConfirmedCryptoPredictions returns the most recent CONFIRMED prediction
	// (actual_price IS NOT NULL) per (coin_id, algorithm_name).
	GetLatestConfirmedCryptoPredictions(ctx context.Context) ([]modelsdb.CryptoPrediction, error)
	GetCryptoPredictionsByDateRange(ctx context.Context, coinID string, from, to time.Time) ([]modelsdb.CryptoPrediction, error)
	GetCryptoPredictionsPage(ctx context.Context, page, limit int, search, algorithm, status, sortBy, sortDir string) ([]modelsdb.CryptoPrediction, int64, error)
	// BulkCreateCryptoPredictions inserts multiple crypto predictions in batches.
	BulkCreateCryptoPredictions(ctx context.Context, preds []modelsdb.CryptoPrediction) error
	// DeleteCryptoPredictionsBeforeDate deletes all crypto predictions whose target_date < cutoff.
	DeleteCryptoPredictionsBeforeDate(ctx context.Context, before time.Time) error
}
