package modelsapi

import (
	"time"

	"github.com/shopspring/decimal"
)

// PredictionDTO - DTO cho prediction response
type PredictionDTO struct {
	ID             uint             `json:"id"`
	StockID        uint             `json:"stock_id"`
	PredictedPrice decimal.Decimal  `json:"predicted_price"`
	Confidence     decimal.Decimal  `json:"confidence"`
	AlgorithmName  string           `json:"algorithm_name"`
	PredictionDate time.Time        `json:"prediction_date"`
	TargetDate     time.Time        `json:"target_date"`
	ActualPrice    *decimal.Decimal `json:"actual_price,omitempty"`
	Accuracy       *decimal.Decimal `json:"accuracy,omitempty"`
}

// CreatePredictionRequest - request để tạo prediction
type CreatePredictionRequest struct {
	StockID        uint            `json:"stock_id" validate:"required"`
	PredictedPrice decimal.Decimal `json:"predicted_price" validate:"required"`
	Confidence     decimal.Decimal `json:"confidence,omitempty"`
	AlgorithmName  string          `json:"algorithm_name" validate:"required"`
	PredictionDate time.Time       `json:"prediction_date" validate:"required"`
	TargetDate     time.Time       `json:"target_date" validate:"required"`
}

// UpdatePredictionRequest - request để update prediction với actual price
type UpdatePredictionRequest struct {
	ActualPrice *decimal.Decimal `json:"actual_price,omitempty"`
	Accuracy    *decimal.Decimal `json:"accuracy,omitempty"`
}

// PredictionListResponse - response cho danh sách predictions
type PredictionListResponse struct {
	Predictions []PredictionDTO `json:"predictions"`
	Total       int             `json:"total"`
}

// PredictionStatsDTO - thống kê prediction accuracy
type PredictionStatsDTO struct {
	AlgorithmName      string          `json:"algorithm_name"`
	TotalPredictions   int             `json:"total_predictions"`
	CorrectPredictions int             `json:"correct_predictions"`
	AverageAccuracy    decimal.Decimal `json:"average_accuracy"`
	LastPrediction     time.Time       `json:"last_prediction"`
}
