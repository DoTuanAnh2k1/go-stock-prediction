package modelsapi

import (
	"time"

	"github.com/shopspring/decimal"
)

type TrainingSessionDTO struct {
	ID              uint                 `json:"id"`
	StartTime       time.Time            `json:"start_time"`
	Duration        int64                `json:"duration_ms"`
	TotalStocks     int                  `json:"total_stocks"`
	SuccessCount    int                  `json:"success_count"`
	ErrorCount      int                  `json:"error_count"`
	OverallAccuracy decimal.Decimal      `json:"overall_accuracy"`
	Status          string               `json:"status"`
	Algorithms      []AlgorithmResultDTO `json:"algorithms"`
}
