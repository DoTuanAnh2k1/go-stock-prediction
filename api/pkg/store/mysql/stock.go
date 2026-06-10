package mysql

import (
	modelsdb "go-stock-prediction/pkg/models/models_db"
)

// ===============================
// STOCK HANDLERS
// ===============================

// GetAllStocks - lấy tất cả stocks
func (c *Client) GetAllStocks() ([]modelsdb.Stock, error) {
	var stocks []modelsdb.Stock
	err := c.Db.Find(&stocks).Error
	return stocks, err
}

// GetStockByID - lấy stock theo ID
func (c *Client) GetStockByID(id uint) (*modelsdb.Stock, error) {
	var stock modelsdb.Stock
	err := c.Db.First(&stock, id).Error
	if err != nil {
		return nil, err
	}
	return &stock, nil
}

// GetStockBySymbol - lấy stock theo symbol
func (c *Client) GetStockBySymbol(symbol string) (*modelsdb.Stock, error) {
	var stock modelsdb.Stock
	err := c.Db.Where("symbol = ?", symbol).First(&stock).Error
	if err != nil {
		return nil, err
	}
	return &stock, nil
}

// GetVN30Stocks - lấy tất cả stocks VN30
func (c *Client) GetVN30Stocks() ([]modelsdb.Stock, error) {
	var stocks []modelsdb.Stock
	err := c.Db.Where("is_vn30 = ?", true).Find(&stocks).Error
	return stocks, err
}

// GetVN100Stocks - lấy tất cả stocks VN100
func (c *Client) GetVN100Stocks() ([]modelsdb.Stock, error) {
	var stocks []modelsdb.Stock
	err := c.Db.Where("is_vn100 = ?", true).Find(&stocks).Error
	return stocks, err
}

// GetStocksByExchange - lấy stocks theo exchange ID
func (c *Client) GetStocksByExchange(exchangeID uint) ([]modelsdb.Stock, error) {
	var stocks []modelsdb.Stock
	err := c.Db.Where("exchange_id = ?", exchangeID).Find(&stocks).Error
	return stocks, err
}

// GetStocksBySector - lấy stocks theo sector
func (c *Client) GetStocksBySector(sector string) ([]modelsdb.Stock, error) {
	var stocks []modelsdb.Stock
	err := c.Db.Where("sector = ?", sector).Find(&stocks).Error
	return stocks, err
}

// SaveStock - save stock (create hoặc update)
func (c *Client) SaveStock(stock *modelsdb.Stock) error {
	return c.Db.Save(stock).Error
}

// CreateStock - tạo stock mới
func (c *Client) CreateStock(stock *modelsdb.Stock) error {
	return c.Db.Create(stock).Error
}

// UpdateStock - update stock
func (c *Client) UpdateStock(stock *modelsdb.Stock) error {
	return c.Db.Save(stock).Error
}

// DeleteStock - xóa stock (soft delete)
func (c *Client) DeleteStock(id uint) error {
	return c.Db.Delete(&modelsdb.Stock{}, id).Error
}

// BulkCreateStocks - tạo nhiều stocks cùng lúc
func (c *Client) BulkCreateStocks(stocks []modelsdb.Stock) error {
	return c.Db.Create(&stocks).Error
}

// CountVN30Stocks - đếm số lượng VN30 stocks
func (c *Client) CountVN30Stocks() (int64, error) {
	var count int64
	err := c.Db.Model(&modelsdb.Stock{}).Where("is_vn30 = ?", true).Count(&count).Error
	return count, err
}

// CountStocks - đếm số lượng stocks
func (c *Client) CountStocks() (int64, error) {
	var count int64
	err := c.Db.Model(&modelsdb.Stock{}).Count(&count).Error
	return count, err
}
