package postgres

import (
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"time"
)

// UpsertNasdaqIntradayPrice creates or updates a NASDAQ intraday price record
// identified by (symbol, timestamp).
func (c *Client) UpsertNasdaqIntradayPrice(p *modelsdb.NasdaqIntradayPrice) error {
	return c.Db.Where("symbol = ? AND timestamp = ?", p.Symbol, p.Timestamp).
		Assign(p).
		FirstOrCreate(p).Error
}

// GetNasdaqIntradayByRange returns NASDAQ intraday prices for a symbol within a
// timestamp range, ordered DESC.
func (c *Client) GetNasdaqIntradayByRange(symbol string, from, to time.Time) ([]modelsdb.NasdaqIntradayPrice, error) {
	var prices []modelsdb.NasdaqIntradayPrice
	err := c.Db.Where("symbol = ? AND timestamp BETWEEN ? AND ?", symbol, from, to).
		Order("timestamp DESC").
		Find(&prices).Error
	return prices, err
}
