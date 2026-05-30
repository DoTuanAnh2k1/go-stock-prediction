package mysql

import (
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"time"
)

// GetGoldPrices returns gold prices filtered by source and productType (empty = all), ordered by trading_date DESC.
func (c *Client) GetGoldPrices(source, productType string, limit int) ([]modelsdb.GoldPrice, error) {
	var prices []modelsdb.GoldPrice
	query := c.Db.Order("trading_date DESC")
	if source != "" {
		query = query.Where("source = ?", source)
	}
	if productType != "" {
		query = query.Where("product_type = ?", productType)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&prices).Error
	return prices, err
}

// GetGoldPricesByDateRange returns gold prices within a date range.
func (c *Client) GetGoldPricesByDateRange(source, productType string, from, to time.Time) ([]modelsdb.GoldPrice, error) {
	var prices []modelsdb.GoldPrice
	query := c.Db.Where("trading_date BETWEEN ? AND ?", from, to).Order("trading_date DESC")
	if source != "" {
		query = query.Where("source = ?", source)
	}
	if productType != "" {
		query = query.Where("product_type = ?", productType)
	}
	err := query.Find(&prices).Error
	return prices, err
}

// GetLatestGoldPrices returns the latest row per (source, product_type) combination.
func (c *Client) GetLatestGoldPrices() ([]modelsdb.GoldPrice, error) {
	var prices []modelsdb.GoldPrice
	subQuery := c.Db.Model(&modelsdb.GoldPrice{}).
		Select("source, product_type, MAX(trading_date) as max_date").
		Group("source, product_type")

	err := c.Db.
		Joins("JOIN (?) as latest ON latest.source = gold_prices.source AND latest.product_type = gold_prices.product_type AND latest.max_date = gold_prices.trading_date", subQuery).
		Where("gold_prices.deleted_at IS NULL").
		Find(&prices).Error
	return prices, err
}

// UpsertGoldPrice creates or updates a gold price record.
func (c *Client) UpsertGoldPrice(price *modelsdb.GoldPrice) error {
	return c.Db.Where("source = ? AND product_type = ? AND trading_date = ?",
		price.Source, price.ProductType, price.TradingDate).
		Assign(price).
		FirstOrCreate(price).Error
}

// BulkUpsertGoldPrices upserts multiple gold price records.
func (c *Client) BulkUpsertGoldPrices(prices []modelsdb.GoldPrice) error {
	for i := range prices {
		if err := c.UpsertGoldPrice(&prices[i]); err != nil {
			return err
		}
	}
	return nil
}
