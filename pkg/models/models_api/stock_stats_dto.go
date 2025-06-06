package modelsapi

import "github.com/shopspring/decimal"

type StockStatsDTO struct {
	HighestPrice  decimal.Decimal `json:"highest_price"`
	LowestPrice   decimal.Decimal `json:"lowest_price"`
	AveragePrice  decimal.Decimal `json:"average_price"`
	TotalVolume   int64           `json:"total_volume"`
	TotalValue    decimal.Decimal `json:"total_value"`
	PriceChange   decimal.Decimal `json:"price_change"`   // Thay đổi từ đầu period
	PercentChange decimal.Decimal `json:"percent_change"` // % thay đổi từ đầu period
	Volatility    decimal.Decimal `json:"volatility"`     // Độ biến động
	TradingDays   int             `json:"trading_days"`
}
