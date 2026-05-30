package modelsdb

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// GoldPrediction - Bảng gold_predictions
type GoldPrediction struct {
	ID             uint             `gorm:"primaryKey;autoIncrement" json:"id"`
	Source         string           `gorm:"size:50;not null;index:idx_gold_pred_src_type_algo,priority:1" json:"source"`
	ProductType    string           `gorm:"size:50;not null;index:idx_gold_pred_src_type_algo,priority:2" json:"product_type"`
	PredictedPrice decimal.Decimal  `gorm:"type:decimal(20,2);not null" json:"predicted_price"`
	CurrentPrice   decimal.Decimal  `gorm:"type:decimal(20,2);not null" json:"current_price"`
	Confidence     decimal.Decimal  `gorm:"type:decimal(5,4)" json:"confidence"`
	AlgorithmName  string           `gorm:"size:50;not null;index:idx_gold_pred_src_type_algo,priority:3" json:"algorithm_name"`
	PredictionDate time.Time        `gorm:"not null;index" json:"prediction_date"`
	TargetDate     time.Time        `gorm:"not null;index" json:"target_date"`
	ActualPrice    *decimal.Decimal `gorm:"type:decimal(20,2)" json:"actual_price"`
	Accuracy       *decimal.Decimal `gorm:"type:decimal(5,4)" json:"accuracy"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
	DeletedAt      gorm.DeletedAt   `gorm:"index" json:"deleted_at,omitempty"`
}

func (GoldPrediction) TableName() string {
	return "gold_predictions"
}
