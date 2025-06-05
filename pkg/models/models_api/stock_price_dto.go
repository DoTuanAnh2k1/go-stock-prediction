package modelsapi

import (
	"time"

	"github.com/shopspring/decimal"
)

type StockPriceDTO struct {
	Symbol        string          `json:"symbol"`
	TradingDate   time.Time       `json:"trading_date"`
	OpenPrice     decimal.Decimal `json:"open_price"`
	HighPrice     decimal.Decimal `json:"high_price"`
	LowPrice      decimal.Decimal `json:"low_price"`
	ClosePrice    decimal.Decimal `json:"close_price"`
	Volume        int64           `json:"volume"`
	Value         decimal.Decimal `json:"value"`
	Change        decimal.Decimal `json:"change"`
	ChangePercent decimal.Decimal `json:"change_percent"`
}
