package mysql

import (
	modelsdb "go-stock-prediction/pkg/models/models_db"
)

// ===============================
// EXCHANGE HANDLERS
// ===============================

// GetAllExchanges - lấy tất cả exchanges
func (c *Client) GetAllExchanges() ([]modelsdb.Exchange, error) {
	var exchanges []modelsdb.Exchange
	err := c.Db.Find(&exchanges).Error
	return exchanges, err
}

// GetExchangeByID - lấy exchange theo ID
func (c *Client) GetExchangeByID(id uint) (*modelsdb.Exchange, error) {
	var exchange modelsdb.Exchange
	err := c.Db.First(&exchange, id).Error
	if err != nil {
		return nil, err
	}
	return &exchange, nil
}

// GetExchangeByCode - lấy exchange theo code
func (c *Client) GetExchangeByCode(code string) (*modelsdb.Exchange, error) {
	var exchange modelsdb.Exchange
	err := c.Db.Where("code = ?", code).First(&exchange).Error
	if err != nil {
		return nil, err
	}
	return &exchange, nil
}

// SaveExchange - save exchange (create hoặc update)
func (c *Client) SaveExchange(exchange *modelsdb.Exchange) error {
	return c.Db.Save(exchange).Error
}

// CreateExchange - tạo exchange mới
func (c *Client) CreateExchange(exchange *modelsdb.Exchange) error {
	return c.Db.Create(exchange).Error
}

// UpdateExchange - update exchange
func (c *Client) UpdateExchange(exchange *modelsdb.Exchange) error {
	return c.Db.Save(exchange).Error
}

// DeleteExchange - xóa exchange (soft delete)
func (c *Client) DeleteExchange(id uint) error {
	return c.Db.Delete(&modelsdb.Exchange{}, id).Error
}
