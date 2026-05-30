package modelsdb

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type TrainingLog struct {
	ID            uint            `gorm:"primaryKey;autoIncrement" json:"id"`
	SessionID     string          `gorm:"size:36;not null;index" json:"session_id"`
	AlgorithmName string          `gorm:"size:50;not null;index" json:"algorithm_name"`
	TotalStocks   int             `gorm:"not null" json:"total_stocks"`
	SuccessCount  int             `gorm:"not null" json:"success_count"`
	ErrorCount    int             `gorm:"not null" json:"error_count"`
	Accuracy      decimal.Decimal `gorm:"type:decimal(5,2)" json:"accuracy"`
	DurationMs    int64           `gorm:"not null" json:"duration_ms"`
	ErrorDetails  string          `gorm:"type:text" json:"error_details"`
	StartedAt     time.Time       `gorm:"not null;index" json:"started_at"`
	CompletedAt   time.Time       `gorm:"not null" json:"completed_at"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	DeletedAt     gorm.DeletedAt  `gorm:"index" json:"deleted_at"`
}
