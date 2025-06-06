package modelsapi

import (
	"time"

	"github.com/shopspring/decimal"
)

type StockChartDataDTO struct {
	Symbol   string           `json:"symbol"`
	Period   string           `json:"period"`
	Interval string           `json:"interval"` // "1m", "5m", "15m", "1h", "1d"
	Data     []ChartPointDTO  `json:"data"`
	Volume   []VolumePointDTO `json:"volume"`
	Total    int              `json:"total"`
}

type ChartPointDTO struct {
	Timestamp time.Time       `json:"timestamp"`
	Open      decimal.Decimal `json:"open"`
	High      decimal.Decimal `json:"high"`
	Low       decimal.Decimal `json:"low"`
	Close     decimal.Decimal `json:"close"`
}

type VolumePointDTO struct {
	Timestamp time.Time       `json:"timestamp"`
	Volume    int64           `json:"volume"`
	Value     decimal.Decimal `json:"value"`
}
