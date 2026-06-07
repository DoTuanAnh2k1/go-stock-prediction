package mysql

import (
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"strings"
	"time"
)

// CreateNasdaqPrice inserts a new NASDAQ price record.
func (c *Client) CreateNasdaqPrice(p *modelsdb.NasdaqPrice) error {
	return c.Db.Create(p).Error
}

// UpsertNasdaqPrice creates or updates a NASDAQ price record identified by (symbol, trading_date).
func (c *Client) UpsertNasdaqPrice(p *modelsdb.NasdaqPrice) error {
	return c.Db.Where("symbol = ? AND trading_date = ?", p.Symbol, p.TradingDate).
		Assign(p).
		FirstOrCreate(p).Error
}

// GetNasdaqPricesByDateRange returns NASDAQ prices for a symbol within a date range, ordered DESC.
func (c *Client) GetNasdaqPricesByDateRange(symbol string, from, to time.Time) ([]modelsdb.NasdaqPrice, error) {
	var prices []modelsdb.NasdaqPrice
	err := c.Db.Where("symbol = ? AND trading_date BETWEEN ? AND ?", symbol, from, to).
		Order("trading_date DESC").
		Find(&prices).Error
	return prices, err
}

// GetLatestNasdaqPrice returns the most recent price record for a given symbol.
func (c *Client) GetLatestNasdaqPrice(symbol string) (*modelsdb.NasdaqPrice, error) {
	var price modelsdb.NasdaqPrice
	err := c.Db.Where("symbol = ?", symbol).
		Order("trading_date DESC").
		First(&price).Error
	if err != nil {
		return nil, err
	}
	return &price, nil
}

// GetNasdaqSymbols returns all distinct symbols present in the nasdaq_prices table.
func (c *Client) GetNasdaqSymbols() ([]string, error) {
	var symbols []string
	err := c.Db.Model(&modelsdb.NasdaqPrice{}).
		Distinct("symbol").
		Pluck("symbol", &symbols).Error
	return symbols, err
}

// GetAllNasdaqPricesForSymbol returns all historical prices for a symbol, ordered DESC.
func (c *Client) GetAllNasdaqPricesForSymbol(symbol string) ([]modelsdb.NasdaqPrice, error) {
	var prices []modelsdb.NasdaqPrice
	err := c.Db.Where("symbol = ?", symbol).Order("trading_date DESC").Find(&prices).Error
	return prices, err
}

// BulkCreateNasdaqPredictions inserts multiple NASDAQ predictions using CreateInBatches.
func (c *Client) BulkCreateNasdaqPredictions(preds []modelsdb.NasdaqPrediction) error {
	return c.Db.CreateInBatches(preds, 200).Error
}

// DeleteNasdaqPredictionsBeforeDate hard-deletes all NASDAQ predictions whose target_date < before
// and that already have an actual_price (i.e. backtest rows).
func (c *Client) DeleteNasdaqPredictionsBeforeDate(before time.Time) error {
	return c.Db.Where("target_date < ? AND actual_price IS NOT NULL", before).
		Delete(&modelsdb.NasdaqPrediction{}).Error
}

// CreateNasdaqPrediction saves a new NASDAQ prediction record.
func (c *Client) CreateNasdaqPrediction(p *modelsdb.NasdaqPrediction) error {
	return c.Db.Create(p).Error
}

// GetNasdaqPredictions returns predictions filtered by symbol and algorithm.
// Empty strings mean no filter. Results are ordered by prediction_date DESC.
func (c *Client) GetNasdaqPredictions(symbol, algorithm string, limit int) ([]modelsdb.NasdaqPrediction, error) {
	var preds []modelsdb.NasdaqPrediction
	query := c.Db.Order("prediction_date DESC")
	if symbol != "" {
		query = query.Where("symbol = ?", symbol)
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

// GetLatestNasdaqPredictions returns the most recent prediction per (symbol, algorithm_name).
func (c *Client) GetLatestNasdaqPredictions() ([]modelsdb.NasdaqPrediction, error) {
	var preds []modelsdb.NasdaqPrediction
	subQuery := c.Db.Model(&modelsdb.NasdaqPrediction{}).
		Select("symbol, algorithm_name, MAX(prediction_date) as max_date").
		Group("symbol, algorithm_name")

	err := c.Db.
		Joins("JOIN (?) as latest ON latest.symbol = nasdaq_predictions.symbol AND latest.algorithm_name = nasdaq_predictions.algorithm_name AND latest.max_date = nasdaq_predictions.prediction_date", subQuery).
		Where("nasdaq_predictions.deleted_at IS NULL").
		Order("nasdaq_predictions.prediction_date DESC").
		Find(&preds).Error
	return preds, err
}

// GetLatestConfirmedNasdaqPredictions returns the most recent prediction
// per (symbol, algorithm_name), regardless of status.
// Uses MAX(id) to avoid duplicates when multiple rows share the same prediction_date.
func (c *Client) GetLatestConfirmedNasdaqPredictions() ([]modelsdb.NasdaqPrediction, error) {
	var preds []modelsdb.NasdaqPrediction
	subQuery := c.Db.Model(&modelsdb.NasdaqPrediction{}).
		Select("MAX(id) as max_id").
		Group("symbol, algorithm_name")
	err := c.Db.
		Joins("JOIN (?) as latest ON latest.max_id = nasdaq_predictions.id", subQuery).
		Where("nasdaq_predictions.deleted_at IS NULL").
		Order("nasdaq_predictions.prediction_date DESC").
		Find(&preds).Error
	return preds, err
}

// GetNasdaqPredictionsByDateRange returns NASDAQ predictions for a symbol within a date range.
// Filters by prediction_date (when the prediction was created) so intraday predictions
// created today are included even though their target_date is tomorrow.
func (c *Client) GetNasdaqPredictionsByDateRange(symbol string, from, to time.Time) ([]modelsdb.NasdaqPrediction, error) {
	var preds []modelsdb.NasdaqPrediction
	query := c.Db.Where("prediction_date BETWEEN ? AND ?", from, to).Order("prediction_date ASC")
	if symbol != "" {
		query = query.Where("symbol = ?", symbol)
	}
	err := query.Find(&preds).Error
	return preds, err
}

// GetNasdaqPredictionsPage returns paginated NASDAQ predictions with optional filters.
func (c *Client) GetNasdaqPredictionsPage(page, limit int, search, algorithm, status, sortBy, sortDir string) ([]modelsdb.NasdaqPrediction, int64, error) {
	var preds []modelsdb.NasdaqPrediction
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

	query := c.Db.Model(&modelsdb.NasdaqPrediction{})

	if search != "" {
		like := "%" + search + "%"
		query = query.Where("symbol LIKE ?", like)
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
