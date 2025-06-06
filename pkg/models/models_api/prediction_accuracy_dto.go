package modelsapi

import "github.com/shopspring/decimal"

type PredictionAccuracyDTO struct {
	AlgorithmName       string          `json:"algorithm_name"`
	Period              string          `json:"period"`
	TotalPredictions    int             `json:"total_predictions"`
	AccuratePredictions int             `json:"accurate_predictions"`
	AccuracyRate        decimal.Decimal `json:"accuracy_rate"`
	AvgError            decimal.Decimal `json:"avg_error"`
}
