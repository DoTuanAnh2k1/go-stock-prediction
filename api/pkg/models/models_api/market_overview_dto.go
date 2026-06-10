package modelsapi

import (
	"time"

	"github.com/shopspring/decimal"
)

// StockFlatDTO is a flattened representation of a stock with its latest price,
// suitable for direct consumption by the frontend without nested objects.
type StockFlatDTO struct {
	Symbol        string          `json:"symbol"`
	CompanyName   string          `json:"company_name"`
	Sector        string          `json:"sector"`
	Exchange      string          `json:"exchange"`
	IsVN30        bool            `json:"is_vn30"`
	CurrentPrice  decimal.Decimal `json:"current_price"`
	Change        decimal.Decimal `json:"change"`
	ChangePercent decimal.Decimal `json:"change_percent"`
	Volume        int64           `json:"volume"`
	Value         decimal.Decimal `json:"value"`
}

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
	Stocks           []StockFlatDTO         `json:"stocks"`
	StocksTotal      int                    `json:"stocks_total"`
	StocksPage       int                    `json:"stocks_page"`
	StocksPageSize   int                    `json:"stocks_page_size"`
	StocksTotalPages int                    `json:"stocks_total_pages"`
	LastUpdated      time.Time              `json:"last_updated"`
}
