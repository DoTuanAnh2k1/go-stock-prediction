package modelsapi

import "time"

type StockWatchlistDTO struct {
	Stocks      []StockCurrentPriceDTO `json:"stocks"`
	Total       int                    `json:"total"`
	LastUpdated time.Time              `json:"last_updated"`
}
