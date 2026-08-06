package mysql

import (
	"context"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"time"
)

// UpsertCryptoIntradayPrice creates or updates a crypto intraday price record
// identified by (coin_id, timestamp).
func (c *Client) UpsertCryptoIntradayPrice(_ context.Context, p *modelsdb.CryptoIntradayPrice) error {
	return c.Db.Where("coin_id = ? AND timestamp = ?", p.CoinID, p.Timestamp).
		Assign(p).
		FirstOrCreate(p).Error
}

// GetCryptoIntradayByRange returns crypto intraday prices for a coinID within a
// timestamp range, ordered DESC.
func (c *Client) GetCryptoIntradayByRange(_ context.Context, coinID string, from, to time.Time) ([]modelsdb.CryptoIntradayPrice, error) {
	var prices []modelsdb.CryptoIntradayPrice
	err := c.Db.Where("coin_id = ? AND timestamp BETWEEN ? AND ?", coinID, from, to).
		Order("timestamp DESC").
		Find(&prices).Error
	return prices, err
}
