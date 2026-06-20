package modelsapi

import (
	"time"

	"github.com/shopspring/decimal"
)

type AlgorithmPerformanceDTO struct {
	Name             string          `json:"name"`
	TotalPredictions int             `json:"total_predictions"`
	AccuracyRate     decimal.Decimal `json:"accuracy_rate"`
	AvgError         decimal.Decimal `json:"avg_error"`
	LastPrediction   time.Time       `json:"last_prediction"`
	Trend            string          `json:"trend"` // improving, declining, stable
}
