package postgres

import (
	"context"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"time"
)

// UpsertGoldIntradayPrice creates or updates a gold intraday price record
// identified by (source, product_type, timestamp).
func (c *Client) UpsertGoldIntradayPrice(ctx context.Context, p *modelsdb.GoldIntradayPrice) error {
	return c.db(ctx).Where("source = ? AND product_type = ? AND timestamp = ?", p.Source, p.ProductType, p.Timestamp).
		Assign(p).
		FirstOrCreate(p).Error
}

// GetGoldIntradayByRange returns gold intraday prices for a source within a
// timestamp range, ordered DESC.
func (c *Client) GetGoldIntradayByRange(ctx context.Context, source string, from, to time.Time) ([]modelsdb.GoldIntradayPrice, error) {
	var prices []modelsdb.GoldIntradayPrice
	err := c.db(ctx).Where("source = ? AND timestamp BETWEEN ? AND ?", source, from, to).
		Order("timestamp DESC").
		Find(&prices).Error
	return prices, err
}
