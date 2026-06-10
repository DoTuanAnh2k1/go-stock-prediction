package modelsdb

import (
	"time"

	"github.com/shopspring/decimal"
)

// NasdaqIntradayPrice stores hourly OHLCV intraday data for NASDAQ 100 constituent stocks.
type NasdaqIntradayPrice struct {
	ID         uint            `gorm:"primaryKey;autoIncrement" json:"id"`
	Symbol     string          `gorm:"type:varchar(20);not null;uniqueIndex:idx_nasdaq_intraday_symbol_ts,priority:1" json:"symbol"`
	Timestamp  time.Time       `gorm:"type:datetime(0);not null;uniqueIndex:idx_nasdaq_intraday_symbol_ts,priority:2" json:"timestamp"`
	OpenPrice  decimal.Decimal `gorm:"type:decimal(20,6)" json:"open_price"`
	HighPrice  decimal.Decimal `gorm:"type:decimal(20,6)" json:"high_price"`
	LowPrice   decimal.Decimal `gorm:"type:decimal(20,6)" json:"low_price"`
	ClosePrice decimal.Decimal `gorm:"type:decimal(20,6)" json:"close_price"`
	Volume     int64           `gorm:"type:bigint" json:"volume"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

func (NasdaqIntradayPrice) TableName() string { return "nasdaq_intraday_prices" }
