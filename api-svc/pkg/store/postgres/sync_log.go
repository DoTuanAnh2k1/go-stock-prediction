package postgres

import (
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"time"
)

// ===============================
// SYNC LOG HANDLERS
// ===============================

// GetAllSyncLogs - lấy tất cả sync logs
func (c *Client) GetAllSyncLogs() ([]modelsdb.SyncLog, error) {
	var syncLogs []modelsdb.SyncLog
	err := c.Db.Order("sync_date DESC").Find(&syncLogs).Error
	return syncLogs, err
}

// GetSyncLogByID - lấy sync log theo ID
func (c *Client) GetSyncLogByID(id uint) (*modelsdb.SyncLog, error) {
	var syncLog modelsdb.SyncLog
	err := c.Db.First(&syncLog, id).Error
	if err != nil {
		return nil, err
	}
	return &syncLog, nil
}

// GetLatestSyncLogs - lấy sync logs mới nhất (limit)
func (c *Client) GetLatestSyncLogs(limit int) ([]modelsdb.SyncLog, error) {
	var syncLogs []modelsdb.SyncLog
	err := c.Db.Order("sync_date DESC").Limit(limit).Find(&syncLogs).Error
	return syncLogs, err
}

// GetSyncLogsBySource - lấy sync logs theo source
func (c *Client) GetSyncLogsBySource(source string) ([]modelsdb.SyncLog, error) {
	var syncLogs []modelsdb.SyncLog
	err := c.Db.Where("source = ?", source).Order("sync_date DESC").Find(&syncLogs).Error
	return syncLogs, err
}

// GetSyncLogsByDateRange - lấy sync logs theo khoảng thời gian
func (c *Client) GetSyncLogsByDateRange(fromDate, toDate time.Time) ([]modelsdb.SyncLog, error) {
	var syncLogs []modelsdb.SyncLog
	err := c.Db.Where("sync_date BETWEEN ? AND ?", fromDate, toDate).
		Order("sync_date DESC").Find(&syncLogs).Error
	return syncLogs, err
}

// SaveSyncLog - save sync log (create hoặc update)
func (c *Client) SaveSyncLog(syncLog *modelsdb.SyncLog) error {
	return c.Db.Save(syncLog).Error
}

// CreateSyncLog - tạo sync log mới
func (c *Client) CreateSyncLog(syncLog *modelsdb.SyncLog) error {
	return c.Db.Create(syncLog).Error
}

// UpdateSyncLog - update sync log
func (c *Client) UpdateSyncLog(syncLog *modelsdb.SyncLog) error {
	return c.Db.Save(syncLog).Error
}

// DeleteSyncLog - xóa sync log (soft delete)
func (c *Client) DeleteSyncLog(id uint) error {
	return c.Db.Delete(&modelsdb.SyncLog{}, id).Error
}

func (c *Client) TruncateSyncLogs() error {
	return c.Db.Exec("TRUNCATE sync_logs").Error
}
