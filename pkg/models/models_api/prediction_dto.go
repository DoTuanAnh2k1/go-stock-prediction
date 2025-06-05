package modelsapi

import (
	"time"

	"github.com/shopspring/decimal"
)

type PredictionDTO struct {
	Symbol         string           `json:"symbol"`
	PredictedPrice decimal.Decimal  `json:"predicted_price"`
	Confidence     decimal.Decimal  `json:"confidence"`
	AlgorithmName  string           `json:"algorithm_name"`
	PredictionDate time.Time        `json:"prediction_date"`
	TargetDate     time.Time        `json:"target_date"`
	ActualPrice    *decimal.Decimal `json:"actual_price,omitempty"`
	Accuracy       *decimal.Decimal `json:"accuracy,omitempty"`
}
