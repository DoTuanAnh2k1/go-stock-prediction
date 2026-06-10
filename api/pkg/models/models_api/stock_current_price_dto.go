package modelsapi

import (
	"time"

	"github.com/shopspring/decimal"
)

type StockCurrentPriceDTO struct {
	Stock         StockDTO        `json:"stock"`
	CurrentPrice  decimal.Decimal `json:"current_price"`
	Change        decimal.Decimal `json:"change"`
	ChangePercent decimal.Decimal `json:"change_percent"`
	Volume        int64           `json:"volume"`
	Value         decimal.Decimal `json:"value"`
	High          decimal.Decimal `json:"high"`
	Low           decimal.Decimal `json:"low"`
	Open          decimal.Decimal `json:"open"`
	TradingDate   time.Time       `json:"trading_date"`
	LastUpdated   time.Time       `json:"last_updated"`
	MarketStatus  string          `json:"market_status"` // "open", "closed", "pre_market", "after_hours"
}
