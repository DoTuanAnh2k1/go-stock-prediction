package modelsapi

import "time"

type StockHistoricalDataDTO struct {
	Stock      StockDTO        `json:"stock"`
	Period     string          `json:"period"` // "1D", "1W", "1M", "3M", "6M", "1Y"
	StartDate  time.Time       `json:"start_date"`
	EndDate    time.Time       `json:"end_date"`
	PriceData  []StockPriceDTO `json:"price_data"`
	Statistics StockStatsDTO   `json:"statistics"`
	Total      int             `json:"total"`
}
