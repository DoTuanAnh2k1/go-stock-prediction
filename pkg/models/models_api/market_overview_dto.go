package modelsapi

import (
	"time"

	"github.com/shopspring/decimal"
)

type MarketOverviewDTO struct {
	VN30Index    decimal.Decimal        `json:"vn30_index"`
	IndexChange  decimal.Decimal        `json:"index_change"`
	IndexPercent decimal.Decimal        `json:"index_percent"`
	TotalStocks  int                    `json:"total_stocks"`
	Gainers      int                    `json:"gainers"`
	Losers       int                    `json:"losers"`
	Unchanged    int                    `json:"unchanged"`
	TotalVolume  int64                  `json:"total_volume"`
	TotalValue   decimal.Decimal        `json:"total_value"`
	TopGainers   []StockCurrentPriceDTO `json:"top_gainers"`
	TopLosers    []StockCurrentPriceDTO `json:"top_losers"`
	MostActive   []StockCurrentPriceDTO `json:"most_active"`
	LastUpdated  time.Time              `json:"last_updated"`
}
