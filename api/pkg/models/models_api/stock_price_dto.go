package modelsapi

import (
	"time"

	"github.com/shopspring/decimal"
)

// StockPriceDTO - DTO cho stock price response
type StockPriceDTO struct {
	ID            uint            `json:"id"`
	StockID       uint            `json:"stock_id"`
	TradingDate   time.Time       `json:"trading_date"`
	OpenPrice     decimal.Decimal `json:"open_price"`
	HighPrice     decimal.Decimal `json:"high_price"`
	LowPrice      decimal.Decimal `json:"low_price"`
	ClosePrice    decimal.Decimal `json:"close_price"`
	Volume        int64           `json:"volume"`
	Value         decimal.Decimal `json:"value"`
	Change        decimal.Decimal `json:"change"`
	ChangePercent decimal.Decimal `json:"change_percent"`
	ForeignBuy    int64           `json:"foreign_buy,omitempty"`
	ForeignSell   int64           `json:"foreign_sell,omitempty"`
}

// CreateStockPriceRequest - request để tạo stock price
type CreateStockPriceRequest struct {
	StockID       uint            `json:"stock_id" validate:"required"`
	TradingDate   time.Time       `json:"trading_date" validate:"required"`
	OpenPrice     decimal.Decimal `json:"open_price" validate:"required"`
	HighPrice     decimal.Decimal `json:"high_price" validate:"required"`
	LowPrice      decimal.Decimal `json:"low_price" validate:"required"`
	ClosePrice    decimal.Decimal `json:"close_price" validate:"required"`
	Volume        int64           `json:"volume" validate:"required"`
	Value         decimal.Decimal `json:"value,omitempty"`
	Change        decimal.Decimal `json:"change,omitempty"`
	ChangePercent decimal.Decimal `json:"change_percent,omitempty"`
	ForeignBuy    int64           `json:"foreign_buy,omitempty"`
	ForeignSell   int64           `json:"foreign_sell,omitempty"`
}

// UpdateStockPriceRequest - request để update stock price
type UpdateStockPriceRequest struct {
	OpenPrice     *decimal.Decimal `json:"open_price,omitempty"`
	HighPrice     *decimal.Decimal `json:"high_price,omitempty"`
	LowPrice      *decimal.Decimal `json:"low_price,omitempty"`
	ClosePrice    *decimal.Decimal `json:"close_price,omitempty"`
	Volume        *int64           `json:"volume,omitempty"`
	Value         *decimal.Decimal `json:"value,omitempty"`
	Change        *decimal.Decimal `json:"change,omitempty"`
	ChangePercent *decimal.Decimal `json:"change_percent,omitempty"`
	ForeignBuy    *int64           `json:"foreign_buy,omitempty"`
	ForeignSell   *int64           `json:"foreign_sell,omitempty"`
}

// StockPriceListResponse - response cho danh sách stock prices
type StockPriceListResponse struct {
	StockPrices []StockPriceDTO `json:"stock_prices"`
	Total       int             `json:"total"`
}

// VN30OverviewResponse - response cho VN30 overview
type VN30OverviewResponse struct {
	TotalStocks int             `json:"total_stocks"`
	Gainers     int             `json:"gainers"`
	Losers      int             `json:"losers"`
	Unchanged   int             `json:"unchanged"`
	TotalValue  decimal.Decimal `json:"total_value"`
	LastUpdated time.Time       `json:"last_updated"`
	StockPrices []StockPriceDTO `json:"stock_prices"`
}
