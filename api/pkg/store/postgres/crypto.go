package postgres

import (
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"strings"
	"time"
)

// CreateCryptoPrice inserts a new cryptocurrency price record.
func (c *Client) CreateCryptoPrice(p *modelsdb.CryptoPrice) error {
	return c.Db.Create(p).Error
}

// UpsertCryptoPrice creates or updates a crypto price record identified by (coin_id, trading_date).
func (c *Client) UpsertCryptoPrice(p *modelsdb.CryptoPrice) error {
	return c.Db.Where("coin_id = ? AND trading_date = ?", p.CoinID, p.TradingDate).
		Assign(p).
		FirstOrCreate(p).Error
}

// GetCryptoPricesByDateRange returns crypto prices for a coinID within a date range, ordered DESC.
func (c *Client) GetCryptoPricesByDateRange(coinID string, from, to time.Time) ([]modelsdb.CryptoPrice, error) {
	var prices []modelsdb.CryptoPrice
	err := c.Db.Where("coin_id = ? AND trading_date BETWEEN ? AND ?", coinID, from, to).
		Order("trading_date DESC").
		Find(&prices).Error
	return prices, err
}

// GetLatestCryptoPrice returns the most recent price record for a given coinID.
func (c *Client) GetLatestCryptoPrice(coinID string) (*modelsdb.CryptoPrice, error) {
	var price modelsdb.CryptoPrice
	err := c.Db.Where("coin_id = ?", coinID).
		Order("trading_date DESC").
		First(&price).Error
	if err != nil {
		return nil, err
	}
	return &price, nil
}

// GetCryptoCoins returns one representative (latest) record per distinct coinID.
func (c *Client) GetCryptoCoins() ([]modelsdb.CryptoPrice, error) {
	var coins []modelsdb.CryptoPrice
	subQuery := c.Db.Model(&modelsdb.CryptoPrice{}).
		Select("coin_id, MAX(trading_date) as max_date").
		Group("coin_id")

	err := c.Db.
		Joins("JOIN (?) as latest ON latest.coin_id = crypto_prices.coin_id AND latest.max_date = crypto_prices.trading_date", subQuery).
		Where("crypto_prices.deleted_at IS NULL").
		Find(&coins).Error
	return coins, err
}

// GetAllCryptoPricesForCoin returns all historical prices for a coinID, ordered DESC.
func (c *Client) GetAllCryptoPricesForCoin(coinID string) ([]modelsdb.CryptoPrice, error) {
	var prices []modelsdb.CryptoPrice
	err := c.Db.Where("coin_id = ?", coinID).Order("trading_date DESC").Find(&prices).Error
	return prices, err
}

// BulkCreateCryptoPredictions inserts multiple crypto predictions using CreateInBatches.
func (c *Client) BulkCreateCryptoPredictions(preds []modelsdb.CryptoPrediction) error {
	return c.Db.CreateInBatches(preds, 200).Error
}

// DeleteCryptoPredictionsBeforeDate hard-deletes all crypto predictions whose target_date < before
// and that already have an actual_price (i.e. backtest rows).
func (c *Client) DeleteCryptoPredictionsBeforeDate(before time.Time) error {
	return c.Db.Where("target_date < ? AND actual_price IS NOT NULL", before).
		Delete(&modelsdb.CryptoPrediction{}).Error
}

// CreateCryptoPrediction saves a new cryptocurrency prediction record.
func (c *Client) CreateCryptoPrediction(p *modelsdb.CryptoPrediction) error {
	return c.Db.Create(p).Error
}

// GetCryptoPredictions returns predictions filtered by coinID and algorithm.
// Empty strings mean no filter. Results are ordered by prediction_date DESC.
func (c *Client) GetCryptoPredictions(coinID, algorithm string, limit int) ([]modelsdb.CryptoPrediction, error) {
	var preds []modelsdb.CryptoPrediction
	query := c.Db.Order("prediction_date DESC")
	if coinID != "" {
		query = query.Where("coin_id = ?", coinID)
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

// GetLatestCryptoPredictions returns the most recent prediction per (coin_id, algorithm_name).
// Uses MAX(id) to avoid duplicates when multiple rows share the same prediction_date.
func (c *Client) GetLatestCryptoPredictions() ([]modelsdb.CryptoPrediction, error) {
	var preds []modelsdb.CryptoPrediction
	subQuery := c.Db.Model(&modelsdb.CryptoPrediction{}).
		Select("MAX(id) as max_id").
		Group("coin_id, algorithm_name")

	err := c.Db.
		Joins("JOIN (?) as latest ON latest.max_id = crypto_predictions.id", subQuery).
		Where("crypto_predictions.deleted_at IS NULL").
		Order("crypto_predictions.prediction_date DESC").
		Find(&preds).Error
	return preds, err
}

// GetLatestConfirmedCryptoPredictions returns the most recent prediction
// per (coin_id, algorithm_name), regardless of status.
// Uses MAX(id) to avoid duplicates when multiple rows share the same prediction_date.
func (c *Client) GetLatestConfirmedCryptoPredictions() ([]modelsdb.CryptoPrediction, error) {
	var preds []modelsdb.CryptoPrediction
	subQuery := c.Db.Model(&modelsdb.CryptoPrediction{}).
		Select("MAX(id) as max_id").
		Group("coin_id, algorithm_name")

	err := c.Db.
		Joins("JOIN (?) as latest ON latest.max_id = crypto_predictions.id", subQuery).
		Where("crypto_predictions.deleted_at IS NULL").
		Order("crypto_predictions.prediction_date DESC").
		Find(&preds).Error
	return preds, err
}

// GetCryptoPredictionsByDateRange returns crypto predictions for a coinID within a date range.
// Filters by prediction_date (when the prediction was created).
func (c *Client) GetCryptoPredictionsByDateRange(coinID string, from, to time.Time) ([]modelsdb.CryptoPrediction, error) {
	var preds []modelsdb.CryptoPrediction
	query := c.Db.Where("prediction_date BETWEEN ? AND ?", from, to).Order("prediction_date ASC")
	if coinID != "" {
		query = query.Where("coin_id = ?", coinID)
	}
	err := query.Find(&preds).Error
	return preds, err
}

// GetCryptoPredictionsPage returns paginated crypto predictions with optional filters.
func (c *Client) GetCryptoPredictionsPage(page, limit int, search, algorithm, status, sortBy, sortDir string) ([]modelsdb.CryptoPrediction, int64, error) {
	var preds []modelsdb.CryptoPrediction
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

	query := c.Db.Model(&modelsdb.CryptoPrediction{})

	if search != "" {
		like := "%" + search + "%"
		query = query.Where("coin_id LIKE ? OR symbol LIKE ?", like, like)
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
