package mysql

import (
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"strings"
	"time"

	"github.com/shopspring/decimal"
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

// GetPredictionsFiltered - lấy predictions với bộ lọc tùy chọn và phân trang
func (c *Client) GetPredictionsFiltered(stockID *uint, algorithm string, fromDate, toDate time.Time, offset, limit int) ([]modelsdb.Prediction, int64, error) {
	var predictions []modelsdb.Prediction
	var total int64

	query := c.Db.Model(&modelsdb.Prediction{})

	if stockID != nil {
		query = query.Where("stock_id = ?", *stockID)
	}
	if algorithm != "" {
		query = query.Where("algorithm_name = ?", algorithm)
	}
	if !fromDate.IsZero() {
		query = query.Where("prediction_date >= ?", fromDate)
	}
	if !toDate.IsZero() {
		query = query.Where("prediction_date <= ?", toDate)
	}

	// Count total before pagination
	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// Apply pagination and fetch
	err = query.Order("prediction_date DESC").
		Offset(offset).
		Limit(limit).
		Find(&predictions).Error

	return predictions, total, err
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

// GetConfirmedPredictionsPage - lấy predictions có actual_price với phân trang
func (c *Client) GetConfirmedPredictionsPage(stockID *uint, algorithm string, fromDate, toDate time.Time, offset, limit int) ([]modelsdb.Prediction, int64, error) {
	var predictions []modelsdb.Prediction
	var total int64

	query := c.Db.Model(&modelsdb.Prediction{}).Where("actual_price IS NOT NULL")

	if stockID != nil {
		query = query.Where("stock_id = ?", *stockID)
	}
	if algorithm != "" {
		query = query.Where("algorithm_name = ?", algorithm)
	}
	if !fromDate.IsZero() {
		query = query.Where("target_date >= ?", fromDate)
	}
	if !toDate.IsZero() {
		query = query.Where("target_date <= ?", toDate)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("target_date DESC").Offset(offset).Limit(limit).Find(&predictions).Error
	return predictions, total, err
}

// GetPredictionsWithActual - lấy predictions có actual_price, lọc theo stock/algorithm và số ngày gần đây
func (c *Client) GetPredictionsWithActual(stockID *uint, algorithm string, days int) ([]modelsdb.Prediction, error) {
	var predictions []modelsdb.Prediction

	query := c.Db.Model(&modelsdb.Prediction{}).Where("actual_price IS NOT NULL")

	if stockID != nil {
		query = query.Where("stock_id = ?", *stockID)
	}
	if algorithm != "" {
		query = query.Where("algorithm_name = ?", algorithm)
	}
	if days > 0 {
		fromDate := time.Now().AddDate(0, 0, -days)
		query = query.Where("target_date >= ?", fromDate)
	}

	err := query.Order("target_date DESC").Find(&predictions).Error
	return predictions, err
}

// GetPredictionCountByAlgorithm - đếm số prediction theo algorithm_name
func (c *Client) GetPredictionCountByAlgorithm() (map[string]int64, error) {
	type result struct {
		AlgorithmName string
		Count         int64
	}
	var rows []result
	err := c.Db.Model(&modelsdb.Prediction{}).
		Select("algorithm_name, COUNT(*) as count").
		Group("algorithm_name").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	m := make(map[string]int64, len(rows))
	for _, r := range rows {
		m[r.AlgorithmName] = r.Count
	}
	return m, nil
}

// GetSuccessfulPredictionCountByAlgorithm - đếm predictions có accuracy >= threshold theo algorithm_name
func (c *Client) GetSuccessfulPredictionCountByAlgorithm(accuracyThreshold float64) (map[string]int64, error) {
	type result struct {
		AlgorithmName string
		Count         int64
	}
	var rows []result
	err := c.Db.Model(&modelsdb.Prediction{}).
		Select("algorithm_name, COUNT(*) as count").
		Where("accuracy IS NOT NULL AND accuracy >= ?", accuracyThreshold).
		Group("algorithm_name").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	m := make(map[string]int64, len(rows))
	for _, r := range rows {
		m[r.AlgorithmName] = r.Count
	}
	return m, nil
}

// DeletePredictionsBeforeDate - xóa tất cả predictions có target_date < date
func (c *Client) DeletePredictionsBeforeDate(date time.Time) error {
	return c.Db.Where("target_date < ?", date).Delete(&modelsdb.Prediction{}).Error
}

// GetPredictionsByMarketPage returns paginated predictions filtered by market.
// Currently supports marketKey="vn30" (stocks with is_vn30=true).
func (c *Client) GetPredictionsByMarketPage(marketKey string, page, limit int, search, sortBy, sortDir, algorithm, status string) ([]modelsdb.Prediction, int64, error) {
	var preds []modelsdb.Prediction
	var total int64

	// Whitelist sortBy
	validSortBy := map[string]bool{
		"prediction_date": true,
		"target_date":     true,
		"accuracy":        true,
		"confidence":      true,
	}
	if !validSortBy[sortBy] {
		sortBy = "prediction_date"
	}
	if strings.ToLower(sortDir) != "asc" {
		sortDir = "DESC"
	} else {
		sortDir = "ASC"
	}

	query := c.Db.Model(&modelsdb.Prediction{}).
		Joins("JOIN stocks s ON s.id = predictions.stock_id AND s.deleted_at IS NULL")

	switch marketKey {
	case "vn30":
		query = query.Where("s.is_vn30 = ?", true)
	}

	if search != "" {
		like := "%" + search + "%"
		query = query.Where("(s.symbol LIKE ? OR s.company_name LIKE ?)", like, like)
	}
	if algorithm != "" {
		query = query.Where("predictions.algorithm_name = ?", algorithm)
	}
	switch status {
	case "confirmed":
		query = query.Where("predictions.actual_price IS NOT NULL")
	case "pending":
		query = query.Where("predictions.actual_price IS NULL")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.
		Preload("Stock").
		Order("predictions." + sortBy + " " + sortDir).
		Offset(offset).
		Limit(limit).
		Find(&preds).Error
	return preds, total, err
}

// GetPendingPredictions - lấy predictions chưa có actual_price và target_date <= cutoff
func (c *Client) GetPendingPredictions(cutoff time.Time) ([]modelsdb.Prediction, error) {
	var predictions []modelsdb.Prediction
	err := c.Db.Where("actual_price IS NULL AND target_date <= ?", cutoff).
		Order("target_date ASC").
		Find(&predictions).Error
	return predictions, err
}

// UpdatePredictionActual - cập nhật actual_price, accuracy và status cho một prediction
func (c *Client) UpdatePredictionActual(id uint, actualPrice, accuracy *decimal.Decimal, status string) error {
	return c.Db.Model(&modelsdb.Prediction{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"actual_price": actualPrice,
			"accuracy":     accuracy,
			"status":       status,
		}).Error
}
