package postgres

import (
	"context"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"time"
)

// GetAllSyncLogs returns all sync logs ordered by sync_date DESC.
func (c *Client) GetAllSyncLogs(ctx context.Context) ([]modelsdb.SyncLog, error) {
	var syncLogs []modelsdb.SyncLog
	err := c.db(ctx).Order("sync_date DESC").Find(&syncLogs).Error
	return syncLogs, err
}

// GetSyncLogByID returns a sync log by ID.
func (c *Client) GetSyncLogByID(ctx context.Context, id uint) (*modelsdb.SyncLog, error) {
	var syncLog modelsdb.SyncLog
	err := c.db(ctx).First(&syncLog, id).Error
	if err != nil {
		return nil, err
	}
	return &syncLog, nil
}

// GetLatestSyncLogs returns the most recent sync logs up to the given limit.
func (c *Client) GetLatestSyncLogs(ctx context.Context, limit int) ([]modelsdb.SyncLog, error) {
	var syncLogs []modelsdb.SyncLog
	err := c.db(ctx).Order("sync_date DESC").Limit(limit).Find(&syncLogs).Error
	return syncLogs, err
}

// GetSyncLogsBySource returns sync logs for a given source.
func (c *Client) GetSyncLogsBySource(ctx context.Context, source string) ([]modelsdb.SyncLog, error) {
	var syncLogs []modelsdb.SyncLog
	err := c.db(ctx).Where("source = ?", source).Order("sync_date DESC").Find(&syncLogs).Error
	return syncLogs, err
}

// GetSyncLogsByDateRange returns sync logs within a date range.
func (c *Client) GetSyncLogsByDateRange(ctx context.Context, fromDate, toDate time.Time) ([]modelsdb.SyncLog, error) {
	var syncLogs []modelsdb.SyncLog
	err := c.db(ctx).Where("sync_date BETWEEN ? AND ?", fromDate, toDate).
		Order("sync_date DESC").Find(&syncLogs).Error
	return syncLogs, err
}

// SaveSyncLog saves (create or update) a sync log.
func (c *Client) SaveSyncLog(ctx context.Context, syncLog *modelsdb.SyncLog) error {
	return c.db(ctx).Save(syncLog).Error
}

// CreateSyncLog inserts a new sync log.
func (c *Client) CreateSyncLog(ctx context.Context, syncLog *modelsdb.SyncLog) error {
	return c.db(ctx).Create(syncLog).Error
}

// UpdateSyncLog updates a sync log.
func (c *Client) UpdateSyncLog(ctx context.Context, syncLog *modelsdb.SyncLog) error {
	return c.db(ctx).Save(syncLog).Error
}

// DeleteSyncLog soft-deletes a sync log by ID.
func (c *Client) DeleteSyncLog(ctx context.Context, id uint) error {
	return c.db(ctx).Delete(&modelsdb.SyncLog{}, id).Error
}

// TruncateSyncLogs truncates the sync_logs table.
func (c *Client) TruncateSyncLogs(ctx context.Context) error {
	return c.db(ctx).Exec("TRUNCATE sync_logs").Error
}
