package server

import (
	"go-stock-prediction/pkg/logger"
	modelsapi "go-stock-prediction/pkg/models/models_api"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"go-stock-prediction/pkg/store/repository"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func GetPredictions(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Info("Getting predictions...")

	symbol := r.URL.Query().Get("symbol")
	algorithm := r.URL.Query().Get("algorithm")
	statusFilter := r.URL.Query().Get("status")
	limitStr := r.URL.Query().Get("limit")
	pageStr := r.URL.Query().Get("page")
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

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

	page, err := validatePage(pageStr)
	if err != nil {
		ResponseError(w, http.StatusBadRequest, err.Error())
		return
	}

	fromDate, err := validateDateParam(fromStr)
	if err != nil {
		ResponseError(w, http.StatusBadRequest, err.Error())
		return
	}

	toDate, err := validateDateParam(toStr)
	if err != nil {
		ResponseError(w, http.StatusBadRequest, err.Error())
		return
	}

	onlyConfirmed := statusFilter == "confirmed"

	// Default from date: 30 days for all predictions, 180 days for confirmed (backtest data)
	if fromDate.IsZero() {
		if onlyConfirmed {
			fromDate = time.Now().AddDate(0, -6, 0)
		} else {
			fromDate = time.Now().AddDate(0, 0, -30)
		}
	}

	store := repository.GetSingleton()

	var stockID *uint
	if symbol != "" {
		stock, err := store.GetStockBySymbol(symbol)
		if err != nil {
			ResponseError(w, http.StatusNotFound, "Stock not found")
			return
		}
		stockID = &stock.ID
	}

	offset := (page - 1) * limit

	var dbPredictions []modelsdb.Prediction
	var totalCount int64
	if onlyConfirmed {
		dbPredictions, totalCount, err = store.GetConfirmedPredictionsPage(stockID, algorithm, fromDate, toDate, offset, limit)
	} else {
		dbPredictions, totalCount, err = store.GetPredictionsFiltered(stockID, algorithm, fromDate, toDate, offset, limit)
	}
	if err != nil {
		logger.Logger.Errorf("Failed to get predictions: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get predictions")
		return
	}

	// Build stock cache to avoid redundant queries
	stockCache := make(map[uint]*modelsdb.Stock)
	var predictions []modelsapi.PredictionDetailDTO

	for _, pred := range dbPredictions {
		stock, ok := stockCache[pred.StockID]
		if !ok {
			s, err := store.GetStockByID(pred.StockID)
			if err != nil {
				logger.Logger.Warnf("Failed to get stock %d for prediction %d: %v", pred.StockID, pred.ID, err)
				continue
			}
			stockCache[pred.StockID] = s
			stock = s
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
			Status:         getPredictionStatusWithPredicted(pred.TargetDate, pred.ActualPrice, &pred.PredictedPrice),
		}
		predictions = append(predictions, predDTO)
	}

	totalPages := int(totalCount) / limit
	if int(totalCount)%limit > 0 {
		totalPages++
	}

	result := &modelsapi.PredictionListDTO{
		Predictions: predictions,
		Total:       len(predictions),
		TotalCount:  totalCount,
		Page:        page,
		PageSize:    limit,
		TotalPages:  totalPages,
		Filters: modelsapi.PredictionFilterDTO{
			StockSymbol:   symbol,
			AlgorithmName: algorithm,
		},
	}

	ResponseSuccess(w, http.StatusOK, result)
}

func GetPredictionDetail(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path /api/predictions/{id}
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	// pathParts should be: ["api", "predictions", "{id}"]
	if len(pathParts) < 3 {
		ResponseError(w, http.StatusBadRequest, "Invalid path")
		return
	}
	idStr := pathParts[2]
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		ResponseError(w, http.StatusBadRequest, "Invalid prediction ID")
		return
	}

	store := repository.GetSingleton()

	pred, err := store.GetPredictionByID(uint(id))
	if err != nil {
		ResponseError(w, http.StatusNotFound, "Prediction not found")
		return
	}

	stock, err := store.GetStockByID(pred.StockID)
	if err != nil {
		logger.Logger.Errorf("Failed to get stock %d: %v", pred.StockID, err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get stock info")
		return
	}

	dto := modelsapi.PredictionDetailDTO{
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
		Status:         getPredictionStatusWithPredicted(pred.TargetDate, pred.ActualPrice, &pred.PredictedPrice),
	}

	ResponseSuccess(w, http.StatusOK, dto)
}
