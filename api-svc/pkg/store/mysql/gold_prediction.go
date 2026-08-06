package mysql

import (
	"context"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// CreateGoldPrediction saves a new gold prediction record.
func (c *Client) CreateGoldPrediction(_ context.Context, pred *modelsdb.GoldPrediction) error {
	return c.Db.Create(pred).Error
}

// GetGoldPredictions returns gold predictions filtered by source, productType, and algorithm.
// Empty strings mean "no filter". Results are ordered by prediction_date DESC.
func (c *Client) GetGoldPredictions(_ context.Context, source, productType, algorithm string, limit int) ([]modelsdb.GoldPrediction, error) {
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
func (c *Client) GetLatestGoldPredictions(_ context.Context) ([]modelsdb.GoldPrediction, error) {
	var preds []modelsdb.GoldPrediction
	subQuery := c.Db.Model(&modelsdb.GoldPrediction{}).
		Select("source, product_type, algorithm_name, MAX(prediction_date) as max_date").
		Group("source, product_type, algorithm_name")

	err := c.Db.
		Joins("JOIN (?) as latest ON latest.source = gold_predictions.source AND latest.product_type = gold_predictions.product_type AND latest.algorithm_name = gold_predictions.algorithm_name AND latest.max_date = gold_predictions.prediction_date", subQuery).
		Where("gold_predictions.deleted_at IS NULL").
		Order("gold_predictions.prediction_date DESC").
		Find(&preds).Error
	return preds, err
}

// GetLatestConfirmedGoldPredictions returns the most recent prediction
// per (source, product_type, algorithm_name), regardless of status.
// Uses MAX(id) to avoid duplicates when multiple rows share the same prediction_date.
func (c *Client) GetLatestConfirmedGoldPredictions(_ context.Context) ([]modelsdb.GoldPrediction, error) {
	var preds []modelsdb.GoldPrediction
	subQuery := c.Db.Model(&modelsdb.GoldPrediction{}).
		Select("MAX(id) as max_id").
		Group("source, product_type, algorithm_name")
	err := c.Db.
		Joins("JOIN (?) as latest ON latest.max_id = gold_predictions.id", subQuery).
		Where("gold_predictions.deleted_at IS NULL").
		Order("gold_predictions.prediction_date DESC").
		Find(&preds).Error
	return preds, err
}

// GetGoldPredictionsByDateRange returns gold predictions within a date range.
// Filters by prediction_date (when the prediction was created) so intraday predictions
// created today are included even though their target_date is tomorrow.
func (c *Client) GetGoldPredictionsByDateRange(_ context.Context, source, productType string, from, to time.Time) ([]modelsdb.GoldPrediction, error) {
	var preds []modelsdb.GoldPrediction
	query := c.Db.Where("prediction_date BETWEEN ? AND ?", from, to).Order("prediction_date ASC")
	if source != "" {
		query = query.Where("source = ?", source)
	}
	if productType != "" {
		query = query.Where("product_type = ?", productType)
	}
	err := query.Find(&preds).Error
	return preds, err
}

// GetGoldPredictionsPage returns paginated gold predictions with optional filters.
func (c *Client) GetGoldPredictionsPage(_ context.Context, page, limit int, search, algorithm, status, sortBy, sortDir string) ([]modelsdb.GoldPrediction, int64, error) {
	var preds []modelsdb.GoldPrediction
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

	query := c.Db.Model(&modelsdb.GoldPrediction{})

	if search != "" {
		like := "%" + search + "%"
		query = query.Where("source LIKE ? OR product_type LIKE ?", like, like)
	}
	if algorithm != "" {
		query = query.Where("algorithm_name = ?", algorithm)
	}
	switch status {
	case "confirmed":
		query = query.Where("actual_price IS NOT NULL")
	case "pending":
		query = query.Where("actual_price IS NULL")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.Order(sortBy + " " + sortDir).
		Offset(offset).
		Limit(limit).
		Find(&preds).Error
	return preds, total, err
}

// GetPendingGoldPredictions returns gold predictions where actual_price IS NULL and target_date <= cutoff.
func (c *Client) GetPendingGoldPredictions(_ context.Context, cutoff time.Time) ([]modelsdb.GoldPrediction, error) {
	var preds []modelsdb.GoldPrediction
	err := c.Db.Where("actual_price IS NULL AND target_date <= ? AND deleted_at IS NULL", cutoff).Find(&preds).Error
	return preds, err
}

// UpdateGoldPredictionActual sets actual_price and accuracy for a gold prediction.
func (c *Client) UpdateGoldPredictionActual(_ context.Context, id uint, actual, accuracy *decimal.Decimal) error {
	return c.Db.Model(&modelsdb.GoldPrediction{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"actual_price": actual,
			"accuracy":     accuracy,
		}).Error
}

// DeleteGoldPredictionsBeforeDate deletes all gold predictions whose target_date < cutoff.
func (c *Client) DeleteGoldPredictionsBeforeDate(_ context.Context, cutoff time.Time) error {
	return c.Db.Where("target_date < ? AND deleted_at IS NULL", cutoff).Delete(&modelsdb.GoldPrediction{}).Error
}

// BulkCreateGoldPredictions inserts multiple gold predictions in batches of 200.
func (c *Client) BulkCreateGoldPredictions(_ context.Context, preds []modelsdb.GoldPrediction) error {
	return c.Db.CreateInBatches(preds, 200).Error
}
