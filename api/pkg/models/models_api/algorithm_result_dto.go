package modelsapi

import "github.com/shopspring/decimal"

type AlgorithmResultDTO struct {
	Name         string          `json:"name"`
	SuccessCount int             `json:"success_count"`
	ErrorCount   int             `json:"error_count"`
	Accuracy     decimal.Decimal `json:"accuracy"`
	Duration     int64           `json:"duration_ms"`
}
