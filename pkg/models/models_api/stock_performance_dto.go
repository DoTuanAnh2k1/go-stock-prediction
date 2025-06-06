package modelsapi

import "github.com/shopspring/decimal"

type StockPerformanceDTO struct {
	Stock         StockDTO        `json:"stock"`
	StartPrice    decimal.Decimal `json:"start_price"`
	EndPrice      decimal.Decimal `json:"end_price"`
	Change        decimal.Decimal `json:"change"`
	ChangePercent decimal.Decimal `json:"change_percent"`
	HighestPrice  decimal.Decimal `json:"highest_price"`
	LowestPrice   decimal.Decimal `json:"lowest_price"`
	Volatility    decimal.Decimal `json:"volatility"`
	TotalVolume   int64           `json:"total_volume"`
}
