package mysql

import (
	"context"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"time"
)

// UpsertSP500IntradayPrice creates or updates an S&P 500 intraday price record
// identified by (symbol, timestamp).
func (c *Client) UpsertSP500IntradayPrice(_ context.Context, p *modelsdb.SP500IntradayPrice) error {
	return c.Db.Where("symbol = ? AND timestamp = ?", p.Symbol, p.Timestamp).
		Assign(p).
		FirstOrCreate(p).Error
}

// GetSP500IntradayByRange returns S&P 500 intraday prices for a symbol within a
// timestamp range, ordered DESC.
func (c *Client) GetSP500IntradayByRange(_ context.Context, symbol string, from, to time.Time) ([]modelsdb.SP500IntradayPrice, error) {
	var prices []modelsdb.SP500IntradayPrice
	err := c.Db.Where("symbol = ? AND timestamp BETWEEN ? AND ?", symbol, from, to).
		Order("timestamp DESC").
		Find(&prices).Error
	return prices, err
}
