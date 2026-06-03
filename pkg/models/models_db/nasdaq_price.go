package modelsdb

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// NasdaqPrice stores daily OHLCV data for NASDAQ 100 constituent stocks.
type NasdaqPrice struct {
	ID          uint            `gorm:"primaryKey;autoIncrement" json:"id"`
	Symbol      string          `gorm:"type:varchar(10);not null;uniqueIndex:idx_nasdaq_symbol_date,priority:1" json:"symbol"`
	CompanyName string          `gorm:"type:varchar(200)" json:"company_name"`
	OpenPrice   decimal.Decimal `gorm:"type:decimal(15,4)" json:"open_price"`
	HighPrice   decimal.Decimal `gorm:"type:decimal(15,4)" json:"high_price"`
	LowPrice    decimal.Decimal `gorm:"type:decimal(15,4)" json:"low_price"`
	ClosePrice  decimal.Decimal `gorm:"type:decimal(15,4);not null" json:"close_price"`
	Volume      int64           `json:"volume"`
	TradingDate time.Time       `gorm:"type:date;not null;uniqueIndex:idx_nasdaq_symbol_date,priority:2" json:"trading_date"`
	Currency    string          `gorm:"type:varchar(3);not null;default:'USD'" json:"currency"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	DeletedAt   gorm.DeletedAt  `gorm:"index" json:"deleted_at,omitempty"`
}

func (NasdaqPrice) TableName() string { return "nasdaq_prices" }

// NasdaqPrediction stores algorithm predictions for NASDAQ 100 instruments.
type NasdaqPrediction struct {
	ID             uint             `gorm:"primaryKey;autoIncrement" json:"id"`
	Symbol         string           `gorm:"type:varchar(10);not null;index:idx_nasdaq_pred,priority:1" json:"symbol"`
	AlgorithmName  string           `gorm:"type:varchar(100);not null;index:idx_nasdaq_pred,priority:2" json:"algorithm_name"`
	PredictedPrice decimal.Decimal  `gorm:"type:decimal(15,4);not null" json:"predicted_price"`
	CurrentPrice   decimal.Decimal  `gorm:"type:decimal(15,4);not null" json:"current_price"`
	Confidence     decimal.Decimal  `gorm:"type:decimal(5,4)" json:"confidence"`
	PredictionDate time.Time        `gorm:"not null;index" json:"prediction_date"`
	TargetDate     time.Time        `gorm:"not null;index" json:"target_date"`
	ActualPrice    *decimal.Decimal `gorm:"type:decimal(15,4)" json:"actual_price,omitempty"`
	Accuracy       *decimal.Decimal `gorm:"type:decimal(5,4)" json:"accuracy,omitempty"`
	Status         string           `gorm:"type:varchar(20);default:'pending'" json:"status"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
	DeletedAt      gorm.DeletedAt   `gorm:"index" json:"deleted_at,omitempty"`
}

func (NasdaqPrediction) TableName() string { return "nasdaq_predictions" }
