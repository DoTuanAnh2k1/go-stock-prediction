package modelsapi

import "time"

type PredictionFilterDTO struct {
	StockSymbol   string     `json:"stock_symbol,omitempty"`
	AlgorithmName string     `json:"algorithm_name,omitempty"`
	FromDate      *time.Time `json:"from_date,omitempty"`
	ToDate        *time.Time `json:"to_date,omitempty"`
	Status        string     `json:"status,omitempty"`
}
