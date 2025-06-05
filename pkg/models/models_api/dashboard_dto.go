package modelsapi

import (
	"time"

	"github.com/shopspring/decimal"
)

// DashboardStatsDTO - DTO cho dashboard statistics
type DashboardStatsDTO struct {
	TotalStocks      int64 `json:"total_stocks"`
	VN30Count        int64 `json:"vn30_count"`
	TotalPrices      int64 `json:"total_prices"`
	TotalPredictions int64 `json:"total_predictions"`
}

// MarketOverviewDTO - DTO cho market overview
type MarketOverviewDTO struct {
	VN30Index     decimal.Decimal `json:"vn30_index"`
	Change        decimal.Decimal `json:"change"`
	ChangePercent decimal.Decimal `json:"change_percent"`
	TotalVolume   int64           `json:"total_volume"`
	TotalValue    decimal.Decimal `json:"total_value"`
	Gainers       int             `json:"gainers"`
	Losers        int             `json:"losers"`
	Unchanged     int             `json:"unchanged"`
	TopGainers    []StockPriceDTO `json:"top_gainers"`
	TopLosers     []StockPriceDTO `json:"top_losers"`
	MostActive    []StockPriceDTO `json:"most_active"`
	LastUpdated   time.Time       `json:"last_updated"`
}

// StockDetailDTO - DTO chi tiết cho 1 mã cổ phiếu
type StockDetailDTO struct {
	Stock             StockDTO             `json:"stock"`
	LatestPrice       *StockPriceDTO       `json:"latest_price,omitempty"`
	PriceHistory      []StockPriceDTO      `json:"price_history,omitempty"`
	LatestPredictions []PredictionDTO      `json:"latest_predictions,omitempty"`
	PredictionStats   []PredictionStatsDTO `json:"prediction_stats,omitempty"`
}
