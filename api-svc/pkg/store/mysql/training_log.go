package mysql

import (
	"context"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"strings"
)

func (c *Client) CreateTrainingLog(_ context.Context, log *modelsdb.TrainingLog) error {
	return c.Db.Create(log).Error
}

func (c *Client) GetTrainingLogByID(_ context.Context, id uint) (*modelsdb.TrainingLog, error) {
	var log modelsdb.TrainingLog
	err := c.Db.First(&log, id).Error
	return &log, err
}

func (c *Client) GetTrainingLogsBySessionID(_ context.Context, sessionID string) ([]modelsdb.TrainingLog, error) {
	var logs []modelsdb.TrainingLog
	err := c.Db.Where("session_id = ?", sessionID).Order("algorithm_name ASC").Find(&logs).Error
	return logs, err
}

// GetTrainingSessions returns training logs grouped by session, ordered by most recent first.
func (c *Client) GetTrainingSessions(_ context.Context, limit, offset int) ([]modelsdb.TrainingLog, int64, error) {
	var total int64

	c.Db.Model(&modelsdb.TrainingLog{}).Select("COUNT(DISTINCT session_id)").Scan(&total)

	var sessionIDs []string
	c.Db.Model(&modelsdb.TrainingLog{}).
		Select("session_id").
		Group("session_id").
		Order("MAX(started_at) DESC").
		Offset(offset).
		Limit(limit).
		Pluck("session_id", &sessionIDs)

	if len(sessionIDs) == 0 {
		return nil, total, nil
	}

	var logs []modelsdb.TrainingLog
	err := c.Db.Where("session_id IN ?", sessionIDs).
		Order("started_at DESC, algorithm_name ASC").
		Find(&logs).Error

	return logs, total, err
}

// GetLatestTrainingLogByAlgorithm returns the single most-recent TrainingLog per algorithm.
func (c *Client) GetLatestTrainingLogByAlgorithm(_ context.Context) ([]modelsdb.TrainingLog, error) {
	type algMax struct {
		AlgorithmName string
		MaxID         uint
	}
	var maxRows []algMax
	err := c.Db.Model(&modelsdb.TrainingLog{}).
		Select("algorithm_name, MAX(id) as max_id").
		Group("algorithm_name").
		Scan(&maxRows).Error
	if err != nil {
		return nil, err
	}

	if len(maxRows) == 0 {
		return nil, nil
	}

	ids := make([]uint, 0, len(maxRows))
	for _, r := range maxRows {
		ids = append(ids, r.MaxID)
	}

	var logs []modelsdb.TrainingLog
	err = c.Db.Where("id IN ?", ids).Find(&logs).Error
	return logs, err
}

// GetTrainingSessionsByMarket returns paginated training logs filtered by market_key.
func (c *Client) GetTrainingSessionsByMarket(_ context.Context, marketKey string, page, limit int, algorithm, sortBy, sortDir string) ([]modelsdb.TrainingLog, int64, error) {
	var logs []modelsdb.TrainingLog
	var total int64

	validSortBy := map[string]bool{
		"started_at":  true,
		"accuracy":    true,
		"duration_ms": true,
	}
	if !validSortBy[sortBy] {
		sortBy = "started_at"
	}
	if strings.ToLower(sortDir) != "asc" {
		sortDir = "DESC"
	} else {
		sortDir = "ASC"
	}

	query := c.Db.Model(&modelsdb.TrainingLog{}).Where("market_key = ?", marketKey)

	if algorithm != "" {
		query = query.Where("algorithm_name = ?", algorithm)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.Order(sortBy + " " + sortDir).
		Offset(offset).
		Limit(limit).
		Find(&logs).Error
	return logs, total, err
}

// GetTrainingMetricsAggregate returns aggregated stats across all training logs.
func (c *Client) GetTrainingMetricsAggregate(_ context.Context) (modelsdb.TrainingMetricsAggregate, error) {
	var agg modelsdb.TrainingMetricsAggregate

	c.Db.Model(&modelsdb.TrainingLog{}).
		Select("COUNT(DISTINCT session_id)").
		Scan(&agg.TotalSessions)

	type rawStats struct {
		AvgDurationMs  float64
		TotalSuccess   float64
		TotalError     float64
		AvgDataQuality float64
	}
	var raw rawStats
	err := c.Db.Model(&modelsdb.TrainingLog{}).
		Select(`
			AVG(duration_ms)                                          AS avg_duration_ms,
			SUM(success_count)                                        AS total_success,
			SUM(error_count)                                          AS total_error,
			AVG(CASE WHEN total_stocks > 0
			         THEN CAST(success_count AS DECIMAL(10,4)) / total_stocks
			         ELSE 1 END)                                      AS avg_data_quality
		`).
		Scan(&raw).Error
	if err != nil {
		return agg, err
	}

	agg.AvgDurationMs = raw.AvgDurationMs
	agg.DataQuality = raw.AvgDataQuality
	total := raw.TotalSuccess + raw.TotalError
	if total > 0 {
		agg.SuccessRate = raw.TotalSuccess / total
	}

	c.Db.Raw("SELECT COUNT(*) FROM predictions WHERE deleted_at IS NULL").Scan(&agg.TotalPredictions)

	return agg, nil
}
