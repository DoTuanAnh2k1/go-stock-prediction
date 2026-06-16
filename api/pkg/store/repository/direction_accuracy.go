package repository

import modelsapi "go-stock-prediction/pkg/models/models_api"

// DirectionAccuracyStore provides direction-accuracy queries across all market prediction tables.
type DirectionAccuracyStore interface {
	// GetDirectionAccuracy returns per-algorithm direction accuracy stats for the given market.
	// market must be one of: GOLD, NASDAQ, CRYPTO, SP500.
	// Only rows where direction_correct IS NOT NULL are counted.
	GetDirectionAccuracy(market string) ([]modelsapi.DirectionAccuracyRow, error)
}
