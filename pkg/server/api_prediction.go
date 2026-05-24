package server

import (
	"go-stock-prediction/pkg/logger"
	modelsapi "go-stock-prediction/pkg/models/models_api"
	"go-stock-prediction/pkg/store/repository"
	"net/http"
	"time"
)

func GetPredictions(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Info("🔮 Getting predictions...")

	// Parse query params
	symbol := r.URL.Query().Get("symbol")
	algorithm := r.URL.Query().Get("algorithm")
	limitStr := r.URL.Query().Get("limit")

	if symbol != "" {
		if err := validateSymbol(symbol); err != nil {
			ResponseError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	if err := validateAlgorithm(algorithm); err != nil {
		ResponseError(w, http.StatusBadRequest, err.Error())
		return
	}

	limit, err := validateLimit(limitStr, 20, 100)
	if err != nil {
		ResponseError(w, http.StatusBadRequest, err.Error())
		return
	}

	store := repository.GetSingleton()

	// Get predictions from DB
	var predictions []modelsapi.PredictionDetailDTO

	if symbol != "" {
		// Get stock first
		stock, err := store.GetStockBySymbol(symbol)
		if err != nil {
			ResponseError(w, http.StatusNotFound, "Stock not found")
			return
		}

		// Get predictions for this stock
		dbPredictions, err := store.GetLatestPredictionsByStockID(stock.ID, limit)
		if err != nil {
			logger.Logger.Errorf("Failed to get predictions: %v", err)
			ResponseError(w, http.StatusInternalServerError, "Failed to get predictions")
			return
		}

		// Convert to DTOs
		for _, pred := range dbPredictions {
			if algorithm != "" && pred.AlgorithmName != algorithm {
				continue
			}

			predDTO := modelsapi.PredictionDetailDTO{
				ID: pred.ID,
				Stock: modelsapi.StockDTO{
					ID:          stock.ID,
					Symbol:      stock.Symbol,
					CompanyName: stock.CompanyName,
					ExchangeID:  stock.ExchangeID,
					IsVN30:      stock.IsVN30,
					Sector:      stock.Sector,
				},
				PredictedPrice: pred.PredictedPrice,
				CurrentPrice:   pred.CurrentPrice,
				ActualPrice:    pred.ActualPrice,
				Confidence:     pred.Confidence,
				AlgorithmName:  pred.AlgorithmName,
				PredictionDate: pred.PredictionDate,
				TargetDate:     pred.TargetDate,
				Accuracy:       pred.Accuracy,
				Status:         getPredictionStatus(pred.TargetDate, pred.ActualPrice),
			}

			predictions = append(predictions, predDTO)
		}
	} else {
		// Get all recent predictions
		fromDate := time.Now().AddDate(0, 0, -7) // Last 7 days
		dbPredictions, err := store.GetPredictionsByDateRange(fromDate, time.Now())
		if err != nil {
			logger.Logger.Errorf("Failed to get predictions: %v", err)
			ResponseError(w, http.StatusInternalServerError, "Failed to get predictions")
			return
		}

		count := 0
		for _, pred := range dbPredictions {
			if count >= limit {
				break
			}

			if algorithm != "" && pred.AlgorithmName != algorithm {
				continue
			}

			// Get stock info
			stock, err := store.GetStockByID(pred.StockID)
			if err != nil {
				continue
			}

			predDTO := modelsapi.PredictionDetailDTO{
				ID: pred.ID,
				Stock: modelsapi.StockDTO{
					ID:          stock.ID,
					Symbol:      stock.Symbol,
					CompanyName: stock.CompanyName,
					ExchangeID:  stock.ExchangeID,
					IsVN30:      stock.IsVN30,
					Sector:      stock.Sector,
				},
				PredictedPrice: pred.PredictedPrice,
				ActualPrice:    pred.ActualPrice,
				Confidence:     pred.Confidence,
				AlgorithmName:  pred.AlgorithmName,
				PredictionDate: pred.PredictionDate,
				TargetDate:     pred.TargetDate,
				Accuracy:       pred.Accuracy,
				Status:         getPredictionStatus(pred.TargetDate, pred.ActualPrice),
			}

			predictions = append(predictions, predDTO)
			count++
		}
	}

	result := &modelsapi.PredictionListDTO{
		Predictions: predictions,
		Total:       len(predictions),
		Filters: modelsapi.PredictionFilterDTO{
			StockSymbol:   symbol,
			AlgorithmName: algorithm,
		},
	}

	ResponseSuccess(w, http.StatusOK, result)
}
