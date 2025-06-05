package modelsdb

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Prediction - Bảng predictions
type Prediction struct {
	ID             uint             `gorm:"primaryKey;autoIncrement" json:"id"`
	StockID        uint             `gorm:"not null;index" json:"stock_id"`
	PredictedPrice decimal.Decimal  `gorm:"type:decimal(15,2);not null" json:"predicted_price"`
	Confidence     decimal.Decimal  `gorm:"type:decimal(5,4)" json:"confidence"` // Độ tin cậy 0-1
	AlgorithmName  string           `gorm:"size:50;not null" json:"algorithm_name"`
	PredictionDate time.Time        `gorm:"not null;index" json:"prediction_date"`
	TargetDate     time.Time        `gorm:"not null" json:"target_date"`            // Ngày dự đoán
	ActualPrice    *decimal.Decimal `gorm:"type:decimal(15,2)" json:"actual_price"` // Giá thực tế
	Accuracy       *decimal.Decimal `gorm:"type:decimal(5,4)" json:"accuracy"`      // Độ chính xác
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
	DeletedAt      gorm.DeletedAt   `gorm:"index" json:"deleted_at,omitempty"`
}

func (Prediction) TableName() string {
	return "predictions"
}
