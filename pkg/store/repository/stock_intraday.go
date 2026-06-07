package repository

import (
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"time"
)

// StockIntradayStore handles persistence for VN30 hourly intraday price records.
type StockIntradayStore interface {
	UpsertStockIntradayPrice(p *modelsdb.StockIntradayPrice) error
	GetStockIntradayByRange(symbol string, from, to time.Time) ([]modelsdb.StockIntradayPrice, error)
}
