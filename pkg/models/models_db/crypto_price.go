package modelsdb

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// CryptoPrice stores daily price data for tracked cryptocurrencies.
type CryptoPrice struct {
	ID          uint            `gorm:"primaryKey;autoIncrement" json:"id"`
	CoinID      string          `gorm:"type:varchar(50);not null;uniqueIndex:idx_crypto_coin_date,priority:1" json:"coin_id"`
	Symbol      string          `gorm:"type:varchar(10);not null" json:"symbol"`
	ClosePrice  decimal.Decimal `gorm:"type:decimal(20,2);not null" json:"close_price"`
	MarketCap   decimal.Decimal `gorm:"type:decimal(30,2)" json:"market_cap"`
	Volume24h   decimal.Decimal `gorm:"type:decimal(30,2)" json:"volume_24h"`
	TradingDate time.Time       `gorm:"type:date;not null;uniqueIndex:idx_crypto_coin_date,priority:2" json:"trading_date"`
	Currency    string          `gorm:"type:varchar(3);not null;default:'USD'" json:"currency"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	DeletedAt   gorm.DeletedAt  `gorm:"index" json:"deleted_at,omitempty"`
}

func (CryptoPrice) TableName() string { return "crypto_prices" }

// CryptoPrediction stores algorithm predictions for tracked cryptocurrencies.
type CryptoPrediction struct {
	ID             uint             `gorm:"primaryKey;autoIncrement" json:"id"`
	CoinID         string           `gorm:"type:varchar(50);not null;index:idx_crypto_pred,priority:1" json:"coin_id"`
	Symbol         string           `gorm:"type:varchar(10);not null" json:"symbol"`
	AlgorithmName  string           `gorm:"type:varchar(100);not null;index:idx_crypto_pred,priority:2" json:"algorithm_name"`
	PredictedPrice decimal.Decimal  `gorm:"type:decimal(20,2);not null" json:"predicted_price"`
	CurrentPrice   decimal.Decimal  `gorm:"type:decimal(20,2);not null" json:"current_price"`
	Confidence     decimal.Decimal  `gorm:"type:decimal(5,4)" json:"confidence"`
	PredictionDate time.Time        `gorm:"not null;index" json:"prediction_date"`
	TargetDate     time.Time        `gorm:"not null;index" json:"target_date"`
	ActualPrice    *decimal.Decimal `gorm:"type:decimal(20,2)" json:"actual_price,omitempty"`
	Accuracy       *decimal.Decimal `gorm:"type:decimal(5,4)" json:"accuracy,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
	DeletedAt      gorm.DeletedAt   `gorm:"index" json:"deleted_at,omitempty"`
}

func (CryptoPrediction) TableName() string { return "crypto_predictions" }
