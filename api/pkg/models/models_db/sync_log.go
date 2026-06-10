package modelsdb

import (
	"time"

	"gorm.io/gorm"
)

// SyncLog - Bảng sync_logs để track crawling history
type SyncLog struct {
	ID           uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	SyncDate     time.Time      `gorm:"not null;index" json:"sync_date"`
	SuccessCount int            `gorm:"not null" json:"success_count"`
	ErrorCount   int            `gorm:"not null" json:"error_count"`
	DurationMs   int64          `gorm:"not null" json:"duration_ms"` // Thời gian crawl (milliseconds)
	Source       string         `gorm:"size:50" json:"source"`       // VietStock, CafeF, etc.
	ErrorMessage string         `gorm:"type:text" json:"error_message,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

func (SyncLog) TableName() string {
	return "sync_logs"
}
