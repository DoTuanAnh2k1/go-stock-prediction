package modelsapi

import (
	"time"

	"github.com/shopspring/decimal"
)

type PredictionDetailDTO struct {
	ID             uint             `json:"id"`
	Stock          StockDTO         `json:"stock"`
	PredictedPrice decimal.Decimal  `json:"predicted_price"`
	CurrentPrice   decimal.Decimal  `json:"current_price"` // <- Thêm field này!
	ActualPrice    *decimal.Decimal `json:"actual_price,omitempty"`
	Confidence     decimal.Decimal  `json:"confidence"`
	AlgorithmName  string           `json:"algorithm_name"`
	PredictionDate time.Time        `json:"prediction_date"`
	TargetDate     time.Time        `json:"target_date"`
	Accuracy       *decimal.Decimal `json:"accuracy,omitempty"`
	Status         string           `json:"status"`
}
