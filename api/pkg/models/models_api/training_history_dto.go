package modelsapi

import "github.com/shopspring/decimal"

type TrainingHistoryDTO struct {
	Sessions    []TrainingSessionDTO `json:"sessions"`
	Total       int                  `json:"total"`
	AvgAccuracy decimal.Decimal      `json:"avg_accuracy"`
	LastWeek    int                  `json:"last_week_count"`
}
