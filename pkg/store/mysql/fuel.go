package mysql

import (
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"strings"
	"time"
)

// CreateFuelPrice inserts a new fuel price record.
func (c *Client) CreateFuelPrice(p *modelsdb.FuelPrice) error {
	return c.Db.Create(p).Error
}

// UpsertFuelPrice creates or updates a fuel price record identified by (product_type, trading_date).
func (c *Client) UpsertFuelPrice(p *modelsdb.FuelPrice) error {
	return c.Db.Where("product_type = ? AND trading_date = ?", p.ProductType, p.TradingDate).
		Assign(p).
		FirstOrCreate(p).Error
}

// BulkUpsertFuelPrices upserts multiple fuel price records sequentially.
func (c *Client) BulkUpsertFuelPrices(prices []modelsdb.FuelPrice) error {
	for i := range prices {
		if err := c.UpsertFuelPrice(&prices[i]); err != nil {
			return err
		}
	}
	return nil
}

// GetFuelPricesByDateRange returns fuel prices for a product type within a date range, ordered DESC.
func (c *Client) GetFuelPricesByDateRange(productType string, from, to time.Time) ([]modelsdb.FuelPrice, error) {
	var prices []modelsdb.FuelPrice
	query := c.Db.Where("trading_date BETWEEN ? AND ?", from, to).Order("trading_date DESC")
	if productType != "" {
		query = query.Where("product_type = ?", productType)
	}
	err := query.Find(&prices).Error
	return prices, err
}

// GetLatestFuelPrice returns the most recent price record for a given product type.
func (c *Client) GetLatestFuelPrice(productType string) (*modelsdb.FuelPrice, error) {
	var price modelsdb.FuelPrice
	err := c.Db.Where("product_type = ?", productType).
		Order("trading_date DESC").
		First(&price).Error
	if err != nil {
		return nil, err
	}
	return &price, nil
}

// GetFuelProducts returns all distinct product_type values in the fuel_prices table.
func (c *Client) GetFuelProducts() ([]string, error) {
	var products []string
	err := c.Db.Model(&modelsdb.FuelPrice{}).
		Distinct("product_type").
		Pluck("product_type", &products).Error
	return products, err
}

// GetAllFuelPricesForProduct returns all historical prices for a product type, ordered DESC.
func (c *Client) GetAllFuelPricesForProduct(productType string) ([]modelsdb.FuelPrice, error) {
	var prices []modelsdb.FuelPrice
	err := c.Db.Where("product_type = ?", productType).Order("trading_date DESC").Find(&prices).Error
	return prices, err
}

// BulkCreateFuelPredictions inserts multiple fuel predictions using CreateInBatches.
func (c *Client) BulkCreateFuelPredictions(preds []modelsdb.FuelPrediction) error {
	return c.Db.CreateInBatches(preds, 200).Error
}

// DeleteFuelPredictionsBeforeDate hard-deletes all fuel predictions whose target_date < before
// and that already have an actual_price (i.e. backtest rows).
func (c *Client) DeleteFuelPredictionsBeforeDate(before time.Time) error {
	return c.Db.Where("target_date < ? AND actual_price IS NOT NULL", before).
		Delete(&modelsdb.FuelPrediction{}).Error
}

// CreateFuelPrediction saves a new fuel prediction record.
func (c *Client) CreateFuelPrediction(p *modelsdb.FuelPrediction) error {
	return c.Db.Create(p).Error
}

// GetFuelPredictions returns predictions filtered by productType and algorithm.
// Empty strings mean no filter. Results are ordered by prediction_date DESC.
func (c *Client) GetFuelPredictions(productType, algorithm string, limit int) ([]modelsdb.FuelPrediction, error) {
	var preds []modelsdb.FuelPrediction
	query := c.Db.Order("prediction_date DESC")
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

// GetLatestFuelPredictions returns the most recent prediction per (product_type, algorithm_name).
func (c *Client) GetLatestFuelPredictions() ([]modelsdb.FuelPrediction, error) {
	var preds []modelsdb.FuelPrediction
	subQuery := c.Db.Model(&modelsdb.FuelPrediction{}).
		Select("product_type, algorithm_name, MAX(prediction_date) as max_date").
		Group("product_type, algorithm_name")

	err := c.Db.
		Joins("JOIN (?) as latest ON latest.product_type = fuel_predictions.product_type AND latest.algorithm_name = fuel_predictions.algorithm_name AND latest.max_date = fuel_predictions.prediction_date", subQuery).
		Where("fuel_predictions.deleted_at IS NULL").
		Find(&preds).Error
	return preds, err
}

// GetLatestConfirmedFuelPredictions returns the most recent prediction
// per (product_type, algorithm_name), regardless of status.
// Uses MAX(id) to avoid duplicates when multiple rows share the same prediction_date.
func (c *Client) GetLatestConfirmedFuelPredictions() ([]modelsdb.FuelPrediction, error) {
	var preds []modelsdb.FuelPrediction
	subQuery := c.Db.Model(&modelsdb.FuelPrediction{}).
		Select("MAX(id) as max_id").
		Group("product_type, algorithm_name")
	err := c.Db.
		Joins("JOIN (?) as latest ON latest.max_id = fuel_predictions.id", subQuery).
		Where("fuel_predictions.deleted_at IS NULL").
		Find(&preds).Error
	return preds, err
}

// GetFuelPredictionsByDateRange returns fuel predictions for a product type within a date range.
func (c *Client) GetFuelPredictionsByDateRange(productType string, from, to time.Time) ([]modelsdb.FuelPrediction, error) {
	var preds []modelsdb.FuelPrediction
	query := c.Db.Where("target_date BETWEEN ? AND ?", from, to).Order("target_date DESC")
	if productType != "" {
		query = query.Where("product_type = ?", productType)
	}
	err := query.Find(&preds).Error
	return preds, err
}

// GetFuelPredictionsPage returns paginated fuel predictions with optional filters.
func (c *Client) GetFuelPredictionsPage(page, limit int, search, algorithm, status, sortBy, sortDir string) ([]modelsdb.FuelPrediction, int64, error) {
	var preds []modelsdb.FuelPrediction
	var total int64

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

	query := c.Db.Model(&modelsdb.FuelPrediction{})

	if search != "" {
		like := "%" + search + "%"
		query = query.Where("product_type LIKE ?", like)
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
