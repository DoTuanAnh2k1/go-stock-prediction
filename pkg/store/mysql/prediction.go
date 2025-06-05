package mysql

import (
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"time"
)

// ===============================
// PREDICTION HANDLERS
// ===============================

// GetAllPredictions - lấy tất cả predictions
func (c *Client) GetAllPredictions() ([]modelsdb.Prediction, error) {
	var predictions []modelsdb.Prediction
	err := c.Db.Find(&predictions).Error
	return predictions, err
}

// GetPredictionByID - lấy prediction theo ID
func (c *Client) GetPredictionByID(id uint) (*modelsdb.Prediction, error) {
	var prediction modelsdb.Prediction
	err := c.Db.First(&prediction, id).Error
	if err != nil {
		return nil, err
	}
	return &prediction, nil
}

// GetPredictionsByStockID - lấy tất cả predictions của 1 stock
func (c *Client) GetPredictionsByStockID(stockID uint) ([]modelsdb.Prediction, error) {
	var predictions []modelsdb.Prediction
	err := c.Db.Where("stock_id = ?", stockID).Order("prediction_date DESC").Find(&predictions).Error
	return predictions, err
}

// GetPredictionsByStockIDAndAlgorithm - lấy predictions theo stock và algorithm
func (c *Client) GetPredictionsByStockIDAndAlgorithm(stockID uint, algorithmName string) ([]modelsdb.Prediction, error) {
	var predictions []modelsdb.Prediction
	err := c.Db.Where("stock_id = ? AND algorithm_name = ?", stockID, algorithmName).
		Order("prediction_date DESC").Find(&predictions).Error
	return predictions, err
}

// GetLatestPredictionsByStockID - lấy predictions mới nhất của 1 stock (limit)
func (c *Client) GetLatestPredictionsByStockID(stockID uint, limit int) ([]modelsdb.Prediction, error) {
	var predictions []modelsdb.Prediction
	err := c.Db.Where("stock_id = ?", stockID).
		Order("prediction_date DESC").
		Limit(limit).
		Find(&predictions).Error
	return predictions, err
}

// GetPredictionsByDateRange - lấy predictions theo khoảng thời gian
func (c *Client) GetPredictionsByDateRange(fromDate, toDate time.Time) ([]modelsdb.Prediction, error) {
	var predictions []modelsdb.Prediction
	err := c.Db.Where("prediction_date BETWEEN ? AND ?", fromDate, toDate).
		Order("prediction_date DESC").Find(&predictions).Error
	return predictions, err
}

// SavePrediction - save prediction (create hoặc update)
func (c *Client) SavePrediction(prediction *modelsdb.Prediction) error {
	return c.Db.Save(prediction).Error
}

// CreatePrediction - tạo prediction mới
func (c *Client) CreatePrediction(prediction *modelsdb.Prediction) error {
	return c.Db.Create(prediction).Error
}

// UpdatePrediction - update prediction
func (c *Client) UpdatePrediction(prediction *modelsdb.Prediction) error {
	return c.Db.Save(prediction).Error
}

// DeletePrediction - xóa prediction (soft delete)
func (c *Client) DeletePrediction(id uint) error {
	return c.Db.Delete(&modelsdb.Prediction{}, id).Error
}

// BulkCreatePredictions - tạo nhiều predictions cùng lúc
func (c *Client) BulkCreatePredictions(predictions []modelsdb.Prediction) error {
	return c.Db.Create(&predictions).Error
}

// CountPredictions - đếm số lượng predictions
func (c *Client) CountPredictions() (int64, error) {
	var count int64
	err := c.Db.Model(&modelsdb.Prediction{}).Count(&count).Error
	return count, err
}

func (c *Client) TruncatePredictions() error {
	return c.Db.Exec("TRUNCATE TABLE predictions").Error
}
