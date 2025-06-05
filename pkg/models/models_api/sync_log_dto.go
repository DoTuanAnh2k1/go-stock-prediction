package modelsapi

import "time"

// SyncLogDTO - DTO cho sync log response
type SyncLogDTO struct {
	ID           uint      `json:"id"`
	SyncDate     time.Time `json:"sync_date"`
	SuccessCount int       `json:"success_count"`
	ErrorCount   int       `json:"error_count"`
	DurationMs   int64     `json:"duration_ms"`
	Source       string    `json:"source"`
	ErrorMessage string    `json:"error_message,omitempty"`
}

// CreateSyncLogRequest - request để tạo sync log
type CreateSyncLogRequest struct {
	SuccessCount int    `json:"success_count" validate:"required"`
	ErrorCount   int    `json:"error_count" validate:"required"`
	DurationMs   int64  `json:"duration_ms" validate:"required"`
	Source       string `json:"source" validate:"required"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// SyncLogListResponse - response cho danh sách sync logs
type SyncLogListResponse struct {
	SyncLogs []SyncLogDTO `json:"sync_logs"`
	Total    int          `json:"total"`
}
