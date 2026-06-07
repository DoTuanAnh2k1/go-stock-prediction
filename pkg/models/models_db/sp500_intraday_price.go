package modelsdb

import (
	"time"

	"github.com/shopspring/decimal"
)

// SP500IntradayPrice stores hourly OHLCV intraday data for S&P 500 constituent stocks and ETFs.
type SP500IntradayPrice struct {
	ID         uint            `gorm:"primaryKey;autoIncrement" json:"id"`
	Symbol     string          `gorm:"type:varchar(20);not null;uniqueIndex:idx_sp500_intraday_symbol_ts,priority:1" json:"symbol"`
	Timestamp  time.Time       `gorm:"type:datetime(0);not null;uniqueIndex:idx_sp500_intraday_symbol_ts,priority:2" json:"timestamp"`
	OpenPrice  decimal.Decimal `gorm:"type:decimal(20,6)" json:"open_price"`
	HighPrice  decimal.Decimal `gorm:"type:decimal(20,6)" json:"high_price"`
	LowPrice   decimal.Decimal `gorm:"type:decimal(20,6)" json:"low_price"`
	ClosePrice decimal.Decimal `gorm:"type:decimal(20,6)" json:"close_price"`
	Volume     int64           `gorm:"type:bigint" json:"volume"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

func (SP500IntradayPrice) TableName() string { return "sp500_intraday_prices" }
