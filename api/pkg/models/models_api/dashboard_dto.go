package modelsapi

import (
	"time"

	"github.com/shopspring/decimal"
)

// DashboardStatsDTO - DTO cho dashboard statistics
type DashboardStatsDTO struct {
	TotalStocks      int64 `json:"total_stocks"`
	TotalPrices      int64 `json:"total_prices"`
	TotalPredictions int64 `json:"total_predictions"`
}

// AlgorithmStatsEntry - thống kê cho một thuật toán trong dashboard
type AlgorithmStatsEntry struct {
	Accuracy    string `json:"accuracy"`
	Predictions int    `json:"predictions"`
}

// DashboardStatsFullDTO - DTO cho GET /api/dashboard/stats
type DashboardStatsFullDTO struct {
	TotalStocks      int64                          `json:"total_stocks"`
	TotalPredictions int64                          `json:"total_predictions"`
	AvgAccuracy      string                         `json:"avg_accuracy"`
	LastCrawl        *time.Time                     `json:"last_crawl"`
	LastPrediction   *time.Time                     `json:"last_prediction"`
	Algorithms       map[string]AlgorithmStatsEntry `json:"algorithms"`
}

// MarketOverviewDTO - DTO cho market overview
type MarketOverview1DTO struct {
	IndexValue    decimal.Decimal `json:"index_value"`
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

// StockDetailDTO - DTO chi tiết cho 1 mã cổ phiếu (Phase 3)
type StockDetailDTO struct {
	Stock        StockDTO              `json:"stock"`
	CurrentPrice StockCurrentPriceDTO  `json:"current_price"`
	History      []StockPriceDTO       `json:"history"`
	Predictions  []PredictionDetailDTO `json:"predictions"`
	Stats        StockStatsDTO         `json:"stats"`
}
