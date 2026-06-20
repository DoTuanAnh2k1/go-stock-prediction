package modelsapi

import (
	"time"

	"github.com/shopspring/decimal"
)

type StockOverviewDTO struct {
	Symbol        string          `json:"symbol"`
	CompanyName   string          `json:"company_name"`
	Sector        string          `json:"sector"`
	CurrentPrice  decimal.Decimal `json:"current_price"`
	Change        decimal.Decimal `json:"change"`
	ChangePercent decimal.Decimal `json:"change_percent"`
	Volume        int64           `json:"volume"`
	TradingDate   time.Time       `json:"trading_date"`
	Predictions   []PredictionDTO `json:"predictions"`
}
