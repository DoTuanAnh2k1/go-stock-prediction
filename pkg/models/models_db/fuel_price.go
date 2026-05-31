package modelsdb

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// FuelPrice stores Vietnamese retail fuel prices per product type.
type FuelPrice struct {
	ID          uint            `gorm:"primaryKey;autoIncrement" json:"id"`
	ProductType string          `gorm:"type:varchar(30);not null;uniqueIndex:idx_fuel_product_date,priority:1" json:"product_type"`
	Price       decimal.Decimal `gorm:"type:decimal(10,3);not null" json:"price"`
	TradingDate time.Time       `gorm:"type:date;not null;uniqueIndex:idx_fuel_product_date,priority:2" json:"trading_date"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	DeletedAt   gorm.DeletedAt  `gorm:"index" json:"deleted_at,omitempty"`
}

func (FuelPrice) TableName() string { return "fuel_prices" }

// FuelPrediction stores algorithm predictions for Vietnamese fuel products.
type FuelPrediction struct {
	ID             uint             `gorm:"primaryKey;autoIncrement" json:"id"`
	ProductType    string           `gorm:"type:varchar(30);not null;index:idx_fuel_pred,priority:1" json:"product_type"`
	AlgorithmName  string           `gorm:"type:varchar(100);not null;index:idx_fuel_pred,priority:2" json:"algorithm_name"`
	PredictedPrice decimal.Decimal  `gorm:"type:decimal(10,3);not null" json:"predicted_price"`
	CurrentPrice   decimal.Decimal  `gorm:"type:decimal(10,3);not null" json:"current_price"`
	Confidence     decimal.Decimal  `gorm:"type:decimal(5,4)" json:"confidence"`
	PredictionDate time.Time        `gorm:"not null;index" json:"prediction_date"`
	TargetDate     time.Time        `gorm:"not null;index" json:"target_date"`
	ActualPrice    *decimal.Decimal `gorm:"type:decimal(10,3)" json:"actual_price,omitempty"`
	Accuracy       *decimal.Decimal `gorm:"type:decimal(5,4)" json:"accuracy,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
	DeletedAt      gorm.DeletedAt   `gorm:"index" json:"deleted_at,omitempty"`
}

func (FuelPrediction) TableName() string { return "fuel_predictions" }
