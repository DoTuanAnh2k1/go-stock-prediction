package modelsdb

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// SP500Price stores daily OHLCV data for S&P 500 constituent stocks and ETFs.
type SP500Price struct {
	ID          uint            `gorm:"primaryKey;autoIncrement" json:"id"`
	Symbol      string          `gorm:"type:varchar(10);not null;uniqueIndex:idx_sp500_symbol_date,priority:1" json:"symbol"`
	CompanyName string          `gorm:"type:varchar(200)" json:"company_name"`
	OpenPrice   decimal.Decimal `gorm:"type:decimal(15,4)" json:"open_price"`
	HighPrice   decimal.Decimal `gorm:"type:decimal(15,4)" json:"high_price"`
	LowPrice    decimal.Decimal `gorm:"type:decimal(15,4)" json:"low_price"`
	ClosePrice  decimal.Decimal `gorm:"type:decimal(15,4);not null" json:"close_price"`
	Volume      int64           `json:"volume"`
	TradingDate time.Time       `gorm:"type:date;not null;uniqueIndex:idx_sp500_symbol_date,priority:2" json:"trading_date"`
	Currency    string          `gorm:"type:varchar(3);not null;default:'USD'" json:"currency"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	DeletedAt   gorm.DeletedAt  `gorm:"index" json:"deleted_at,omitempty"`
}

func (SP500Price) TableName() string { return "sp500_prices" }

// SP500Prediction stores algorithm predictions for S&P 500 instruments.
type SP500Prediction struct {
	ID             uint             `gorm:"primaryKey;autoIncrement" json:"id"`
	Symbol         string           `gorm:"type:varchar(10);not null;index:idx_sp500_pred,priority:1" json:"symbol"`
	AlgorithmName  string           `gorm:"type:varchar(100);not null;index:idx_sp500_pred,priority:2" json:"algorithm_name"`
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

func (SP500Prediction) TableName() string { return "sp500_predictions" }
