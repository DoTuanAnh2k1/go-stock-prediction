package mysql

import (
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"time"
)

// CreateGoldPrediction saves a new gold prediction record.
func (c *Client) CreateGoldPrediction(pred *modelsdb.GoldPrediction) error {
	return c.Db.Create(pred).Error
}

// GetGoldPredictions returns gold predictions filtered by source, productType, and algorithm.
// Empty strings mean "no filter". Results are ordered by prediction_date DESC.
func (c *Client) GetGoldPredictions(source, productType, algorithm string, limit int) ([]modelsdb.GoldPrediction, error) {
	var preds []modelsdb.GoldPrediction
	query := c.Db.Order("prediction_date DESC")
	if source != "" {
		query = query.Where("source = ?", source)
	}
	if productType != "" {
		query = query.Where("product_type = ?", productType)
	}
	if algorithm != "" {
		query = query.Where("algorithm_name = ?", algorithm)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&preds).Error
	return preds, err
}

// GetLatestGoldPredictions returns the most recent prediction per (source, product_type, algorithm_name).
func (c *Client) GetLatestGoldPredictions() ([]modelsdb.GoldPrediction, error) {
	var preds []modelsdb.GoldPrediction
	subQuery := c.Db.Model(&modelsdb.GoldPrediction{}).
		Select("source, product_type, algorithm_name, MAX(prediction_date) as max_date").
		Group("source, product_type, algorithm_name")

	err := c.Db.
		Joins("JOIN (?) as latest ON latest.source = gold_predictions.source AND latest.product_type = gold_predictions.product_type AND latest.algorithm_name = gold_predictions.algorithm_name AND latest.max_date = gold_predictions.prediction_date", subQuery).
		Where("gold_predictions.deleted_at IS NULL").
		Find(&preds).Error
	return preds, err
}

// GetGoldPredictionsByDateRange returns gold predictions within a date range.
func (c *Client) GetGoldPredictionsByDateRange(source, productType string, from, to time.Time) ([]modelsdb.GoldPrediction, error) {
	var preds []modelsdb.GoldPrediction
	query := c.Db.Where("prediction_date BETWEEN ? AND ?", from, to).Order("prediction_date DESC")
	if source != "" {
		query = query.Where("source = ?", source)
	}
	if productType != "" {
		query = query.Where("product_type = ?", productType)
	}
	err := query.Find(&preds).Error
	return preds, err
}
