package mysql

import (
	"context"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"time"
)

// GetAllSyncLogs returns all sync logs ordered by sync_date DESC.
func (c *Client) GetAllSyncLogs(_ context.Context) ([]modelsdb.SyncLog, error) {
	var syncLogs []modelsdb.SyncLog
	err := c.Db.Order("sync_date DESC").Find(&syncLogs).Error
	return syncLogs, err
}

// GetSyncLogByID returns a sync log by ID.
func (c *Client) GetSyncLogByID(_ context.Context, id uint) (*modelsdb.SyncLog, error) {
	var syncLog modelsdb.SyncLog
	err := c.Db.First(&syncLog, id).Error
	if err != nil {
		return nil, err
	}
	return &syncLog, nil
}

// GetLatestSyncLogs returns the most recent sync logs up to the given limit.
func (c *Client) GetLatestSyncLogs(_ context.Context, limit int) ([]modelsdb.SyncLog, error) {
	var syncLogs []modelsdb.SyncLog
	err := c.Db.Order("sync_date DESC").Limit(limit).Find(&syncLogs).Error
	return syncLogs, err
}

// GetSyncLogsBySource returns sync logs for a given source.
func (c *Client) GetSyncLogsBySource(_ context.Context, source string) ([]modelsdb.SyncLog, error) {
	var syncLogs []modelsdb.SyncLog
	err := c.Db.Where("source = ?", source).Order("sync_date DESC").Find(&syncLogs).Error
	return syncLogs, err
}

// GetSyncLogsByDateRange returns sync logs within a date range.
func (c *Client) GetSyncLogsByDateRange(_ context.Context, fromDate, toDate time.Time) ([]modelsdb.SyncLog, error) {
	var syncLogs []modelsdb.SyncLog
	err := c.Db.Where("sync_date BETWEEN ? AND ?", fromDate, toDate).
		Order("sync_date DESC").Find(&syncLogs).Error
	return syncLogs, err
}

// SaveSyncLog saves (create or update) a sync log.
func (c *Client) SaveSyncLog(_ context.Context, syncLog *modelsdb.SyncLog) error {
	return c.Db.Save(syncLog).Error
}

// CreateSyncLog inserts a new sync log.
func (c *Client) CreateSyncLog(_ context.Context, syncLog *modelsdb.SyncLog) error {
	return c.Db.Create(syncLog).Error
}

// UpdateSyncLog updates a sync log.
func (c *Client) UpdateSyncLog(_ context.Context, syncLog *modelsdb.SyncLog) error {
	return c.Db.Save(syncLog).Error
}

// DeleteSyncLog soft-deletes a sync log by ID.
func (c *Client) DeleteSyncLog(_ context.Context, id uint) error {
	return c.Db.Delete(&modelsdb.SyncLog{}, id).Error
}

// TruncateSyncLogs truncates the sync_logs table.
func (c *Client) TruncateSyncLogs(_ context.Context) error {
	return c.Db.Exec("TRUNCATE TABLE sync_logs").Error
}
