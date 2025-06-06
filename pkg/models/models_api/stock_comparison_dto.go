package modelsapi

import "time"

type StockComparisonDTO struct {
	Period         string                `json:"period"`
	StartDate      time.Time             `json:"start_date"`
	EndDate        time.Time             `json:"end_date"`
	Stocks         []StockPerformanceDTO `json:"stocks"`
	BestPerformer  string                `json:"best_performer"`
	WorstPerformer string                `json:"worst_performer"`
}
