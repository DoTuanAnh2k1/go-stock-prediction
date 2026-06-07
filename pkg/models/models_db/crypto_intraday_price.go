package modelsdb

import (
	"time"

	"github.com/shopspring/decimal"
)

// CryptoIntradayPrice stores hourly intraday price data for tracked cryptocurrencies.
type CryptoIntradayPrice struct {
	ID        uint            `gorm:"primaryKey;autoIncrement" json:"id"`
	CoinID    string          `gorm:"type:varchar(50);not null;uniqueIndex:idx_crypto_intraday_coin_ts,priority:1" json:"coin_id"`
	Timestamp time.Time       `gorm:"type:datetime(0);not null;uniqueIndex:idx_crypto_intraday_coin_ts,priority:2" json:"timestamp"`
	Price     decimal.Decimal `gorm:"type:decimal(30,8)" json:"price"`
	MarketCap decimal.Decimal `gorm:"type:decimal(30,2)" json:"market_cap"`
	Volume    decimal.Decimal `gorm:"type:decimal(30,2)" json:"volume"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

func (CryptoIntradayPrice) TableName() string { return "crypto_intraday_prices" }
