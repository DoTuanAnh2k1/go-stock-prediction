package postgres

import (
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"time"
)

// UpsertGoldIntradayPrice creates or updates a gold intraday price record
// identified by (source, product_type, timestamp).
func (c *Client) UpsertGoldIntradayPrice(p *modelsdb.GoldIntradayPrice) error {
	return c.Db.Where("source = ? AND product_type = ? AND timestamp = ?", p.Source, p.ProductType, p.Timestamp).
		Assign(p).
		FirstOrCreate(p).Error
}

// GetGoldIntradayByRange returns gold intraday prices for a source within a
// timestamp range, ordered DESC.
func (c *Client) GetGoldIntradayByRange(source string, from, to time.Time) ([]modelsdb.GoldIntradayPrice, error) {
	var prices []modelsdb.GoldIntradayPrice
	err := c.Db.Where("source = ? AND timestamp BETWEEN ? AND ?", source, from, to).
		Order("timestamp DESC").
		Find(&prices).Error
	return prices, err
}
