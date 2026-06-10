package mysql

import (
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"time"
)

// ===============================
// STOCK PRICE HANDLERS
// ===============================

// GetAllStockPrices - lấy tất cả stock prices
func (c *Client) GetAllStockPrices() ([]modelsdb.StockPrice, error) {
	var stockPrices []modelsdb.StockPrice
	err := c.Db.Find(&stockPrices).Error
	return stockPrices, err
}

// GetStockPriceByID - lấy stock price theo ID
func (c *Client) GetStockPriceByID(id uint) (*modelsdb.StockPrice, error) {
	var stockPrice modelsdb.StockPrice
	err := c.Db.First(&stockPrice, id).Error
	if err != nil {
		return nil, err
	}
	return &stockPrice, nil
}

// GetStockPricesByStockID - lấy tất cả prices của 1 stock
func (c *Client) GetStockPricesByStockID(stockID uint) ([]modelsdb.StockPrice, error) {
	var stockPrices []modelsdb.StockPrice
	err := c.Db.Where("stock_id = ?", stockID).Order("trading_date DESC").Find(&stockPrices).Error
	return stockPrices, err
}

// GetStockPricesByStockIDAndDateRange - lấy prices theo stock ID và khoảng thời gian
func (c *Client) GetStockPricesByStockIDAndDateRange(stockID uint, fromDate, toDate time.Time) ([]modelsdb.StockPrice, error) {
	var stockPrices []modelsdb.StockPrice
	err := c.Db.Where("stock_id = ? AND trading_date BETWEEN ? AND ?", stockID, fromDate, toDate).
		Order("trading_date DESC").Find(&stockPrices).Error
	return stockPrices, err
}

// GetLatestStockPriceByStockID - lấy giá mới nhất của 1 stock
func (c *Client) GetLatestStockPriceByStockID(stockID uint) (*modelsdb.StockPrice, error) {
	var stockPrice modelsdb.StockPrice
	err := c.Db.Where("stock_id = ?", stockID).Order("trading_date DESC").First(&stockPrice).Error
	if err != nil {
		return nil, err
	}
	return &stockPrice, nil
}

// GetStockPriceByStockIDAndDate - lấy giá theo stock ID và ngày cụ thể
func (c *Client) GetStockPriceByStockIDAndDate(stockID uint, tradingDate time.Time) (*modelsdb.StockPrice, error) {
	var stockPrice modelsdb.StockPrice
	err := c.Db.Where("stock_id = ? AND trading_date = ?", stockID, tradingDate).First(&stockPrice).Error
	if err != nil {
		return nil, err
	}
	return &stockPrice, nil
}

// GetLatestStockPricesForVN30 - lấy giá mới nhất của tất cả VN30
func (c *Client) GetLatestStockPricesForVN30() ([]modelsdb.StockPrice, error) {
	var stockPrices []modelsdb.StockPrice
	subQuery := c.Db.Model(&modelsdb.StockPrice{}).
		Select("stock_id, MAX(trading_date) as max_date").
		Group("stock_id")

	err := c.Db.
		Joins("JOIN stocks ON stocks.id = stock_prices.stock_id").
		Joins("JOIN (?) as latest ON latest.stock_id = stock_prices.stock_id AND latest.max_date = stock_prices.trading_date", subQuery).
		Where("stocks.is_vn30 = ? AND stocks.deleted_at IS NULL", true).
		Find(&stockPrices).Error

	return stockPrices, err
}

// SaveStockPrice - save stock price (create hoặc update)
func (c *Client) SaveStockPrice(stockPrice *modelsdb.StockPrice) error {
	return c.Db.Save(stockPrice).Error
}

// CreateStockPrice - tạo stock price mới
func (c *Client) CreateStockPrice(stockPrice *modelsdb.StockPrice) error {
	return c.Db.Create(stockPrice).Error
}

// UpsertStockPrice - upsert stock price (update nếu có, create nếu chưa)
func (c *Client) UpsertStockPrice(stockPrice *modelsdb.StockPrice) error {
	return c.Db.Where("stock_id = ? AND trading_date = ?", stockPrice.StockID, stockPrice.TradingDate).
		Assign(stockPrice).
		FirstOrCreate(stockPrice).Error
}

// UpdateStockPrice - update stock price
func (c *Client) UpdateStockPrice(stockPrice *modelsdb.StockPrice) error {
	return c.Db.Save(stockPrice).Error
}

// DeleteStockPrice - xóa stock price (soft delete)
func (c *Client) DeleteStockPrice(id uint) error {
	return c.Db.Delete(&modelsdb.StockPrice{}, id).Error
}

// BulkCreateStockPrices - tạo nhiều stock prices cùng lúc
func (c *Client) BulkCreateStockPrices(stockPrices []modelsdb.StockPrice) error {
	return c.Db.Create(&stockPrices).Error
}

// BulkUpsertStockPrices - upsert nhiều stock prices cùng lúc
func (c *Client) BulkUpsertStockPrices(stockPrices []modelsdb.StockPrice) error {
	// Với MySQL, ta dùng ON DUPLICATE KEY UPDATE
	return c.Db.Create(&stockPrices).Error
}

// CountStockPrices - đếm số lượng stock prices
func (c *Client) CountStockPrices() (int64, error) {
	var count int64
	err := c.Db.Model(&modelsdb.StockPrice{}).Count(&count).Error
	return count, err
}

// TruncateTable - xóa hết data của table (dùng cẩn thận!)
func (c *Client) TruncateStockPrices() error {
	return c.Db.Exec("TRUNCATE TABLE stock_prices").Error
}
