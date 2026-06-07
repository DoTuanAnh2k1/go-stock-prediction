package mysql

import (
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"time"
)

// UpsertStockIntradayPrice creates or updates a VN30 stock intraday price record
// identified by (symbol, timestamp).
func (c *Client) UpsertStockIntradayPrice(p *modelsdb.StockIntradayPrice) error {
	return c.Db.Where("symbol = ? AND timestamp = ?", p.Symbol, p.Timestamp).
		Assign(p).
		FirstOrCreate(p).Error
}

// GetStockIntradayByRange returns VN30 stock intraday prices for a symbol within a
// timestamp range, ordered DESC.
func (c *Client) GetStockIntradayByRange(symbol string, from, to time.Time) ([]modelsdb.StockIntradayPrice, error) {
	var prices []modelsdb.StockIntradayPrice
	err := c.Db.Where("symbol = ? AND timestamp BETWEEN ? AND ?", symbol, from, to).
		Order("timestamp DESC").
		Find(&prices).Error
	return prices, err
}
