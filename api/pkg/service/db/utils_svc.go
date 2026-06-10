package db

import (
	modelsapi "go-stock-prediction/pkg/models/models_api"
	"go-stock-prediction/pkg/store/repository"
)

// ===============================
// UTILITY SERVICES
// ===============================

func GetDashboardStats() (*modelsapi.DashboardStatsDTO, error) {
	totalStocks, err := repository.GetSingleton().CountStocks()
	if err != nil {
		return nil, err
	}

	vn30Count, err := repository.GetSingleton().CountVN30Stocks()
	if err != nil {
		return nil, err
	}

	totalPrices, err := repository.GetSingleton().CountStockPrices()
	if err != nil {
		return nil, err
	}

	totalPredictions, err := repository.GetSingleton().CountPredictions()
	if err != nil {
		return nil, err
	}

	return &modelsapi.DashboardStatsDTO{
		TotalStocks:      totalStocks,
		VN30Count:        vn30Count,
		TotalPrices:      totalPrices,
		TotalPredictions: totalPredictions,
	}, nil
}
