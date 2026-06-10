package mysql

import (
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"strings"
	"time"
)

// CreateSP500Price inserts a new S&P 500 price record.
func (c *Client) CreateSP500Price(p *modelsdb.SP500Price) error {
	return c.Db.Create(p).Error
}

// UpsertSP500Price creates or updates an S&P 500 price record identified by (symbol, trading_date).
func (c *Client) UpsertSP500Price(p *modelsdb.SP500Price) error {
	return c.Db.Where("symbol = ? AND trading_date = ?", p.Symbol, p.TradingDate).
		Assign(p).
		FirstOrCreate(p).Error
}

// GetSP500PricesByDateRange returns S&P 500 prices for a symbol within a date range, ordered DESC.
func (c *Client) GetSP500PricesByDateRange(symbol string, from, to time.Time) ([]modelsdb.SP500Price, error) {
	var prices []modelsdb.SP500Price
	err := c.Db.Where("symbol = ? AND trading_date BETWEEN ? AND ?", symbol, from, to).
		Order("trading_date DESC").
		Find(&prices).Error
	return prices, err
}

// GetLatestSP500Price returns the most recent price record for a given symbol.
func (c *Client) GetLatestSP500Price(symbol string) (*modelsdb.SP500Price, error) {
	var price modelsdb.SP500Price
	err := c.Db.Where("symbol = ?", symbol).
		Order("trading_date DESC").
		First(&price).Error
	if err != nil {
		return nil, err
	}
	return &price, nil
}

// GetSP500Symbols returns all distinct symbols present in the sp500_prices table.
func (c *Client) GetSP500Symbols() ([]string, error) {
	var symbols []string
	err := c.Db.Model(&modelsdb.SP500Price{}).
		Distinct("symbol").
		Pluck("symbol", &symbols).Error
	return symbols, err
}

// GetAllSP500PricesForSymbol returns all historical prices for a symbol, ordered DESC.
func (c *Client) GetAllSP500PricesForSymbol(symbol string) ([]modelsdb.SP500Price, error) {
	var prices []modelsdb.SP500Price
	err := c.Db.Where("symbol = ?", symbol).Order("trading_date DESC").Find(&prices).Error
	return prices, err
}

// BulkCreateSP500Predictions inserts multiple S&P 500 predictions using CreateInBatches.
func (c *Client) BulkCreateSP500Predictions(preds []modelsdb.SP500Prediction) error {
	return c.Db.CreateInBatches(preds, 200).Error
}

// DeleteSP500PredictionsBeforeDate hard-deletes all S&P 500 predictions whose target_date < before
// and that already have an actual_price (i.e. backtest rows).
func (c *Client) DeleteSP500PredictionsBeforeDate(before time.Time) error {
	return c.Db.Where("target_date < ? AND actual_price IS NOT NULL", before).
		Delete(&modelsdb.SP500Prediction{}).Error
}

// CreateSP500Prediction saves a new S&P 500 prediction record.
func (c *Client) CreateSP500Prediction(p *modelsdb.SP500Prediction) error {
	return c.Db.Create(p).Error
}

// GetSP500Predictions returns predictions filtered by symbol and algorithm.
// Empty strings mean no filter. Results are ordered by prediction_date DESC.
func (c *Client) GetSP500Predictions(symbol, algorithm string, limit int) ([]modelsdb.SP500Prediction, error) {
	var preds []modelsdb.SP500Prediction
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

// GetLatestSP500Predictions returns the most recent prediction per (symbol, algorithm_name).
func (c *Client) GetLatestSP500Predictions() ([]modelsdb.SP500Prediction, error) {
	var preds []modelsdb.SP500Prediction
	subQuery := c.Db.Model(&modelsdb.SP500Prediction{}).
		Select("symbol, algorithm_name, MAX(prediction_date) as max_date").
		Group("symbol, algorithm_name")

	err := c.Db.
		Joins("JOIN (?) as latest ON latest.symbol = sp500_predictions.symbol AND latest.algorithm_name = sp500_predictions.algorithm_name AND latest.max_date = sp500_predictions.prediction_date", subQuery).
		Where("sp500_predictions.deleted_at IS NULL").
		Order("sp500_predictions.prediction_date DESC").
		Find(&preds).Error
	return preds, err
}

// GetLatestConfirmedSP500Predictions returns the most recent prediction
// per (symbol, algorithm_name), regardless of status.
// Uses MAX(id) to avoid duplicates when multiple rows share the same prediction_date.
func (c *Client) GetLatestConfirmedSP500Predictions() ([]modelsdb.SP500Prediction, error) {
	var preds []modelsdb.SP500Prediction
	subQuery := c.Db.Model(&modelsdb.SP500Prediction{}).
		Select("MAX(id) as max_id").
		Group("symbol, algorithm_name")
	err := c.Db.
		Joins("JOIN (?) as latest ON latest.max_id = sp500_predictions.id", subQuery).
		Where("sp500_predictions.deleted_at IS NULL").
		Order("sp500_predictions.prediction_date DESC").
		Find(&preds).Error
	return preds, err
}

// GetSP500PredictionsByDateRange returns S&P 500 predictions for a symbol within a date range.
// Filters by prediction_date (when the prediction was created) so intraday predictions
// created today are included even though their target_date is tomorrow.
func (c *Client) GetSP500PredictionsByDateRange(symbol string, from, to time.Time) ([]modelsdb.SP500Prediction, error) {
	var preds []modelsdb.SP500Prediction
	query := c.Db.Where("prediction_date BETWEEN ? AND ?", from, to).Order("prediction_date ASC")
	if symbol != "" {
		query = query.Where("symbol = ?", symbol)
	}
	err := query.Find(&preds).Error
	return preds, err
}

// GetSP500PredictionsPage returns paginated S&P 500 predictions with optional filters.
func (c *Client) GetSP500PredictionsPage(page, limit int, search, algorithm, status, sortBy, sortDir string) ([]modelsdb.SP500Prediction, int64, error) {
	var preds []modelsdb.SP500Prediction
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

	query := c.Db.Model(&modelsdb.SP500Prediction{})

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
