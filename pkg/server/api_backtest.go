package server

import (
	"context"
	"go-stock-prediction/pkg/logger"
	modelsapi "go-stock-prediction/pkg/models/models_api"
	arimagarch "go-stock-prediction/pkg/service/predict/arima_garch"
	"go-stock-prediction/pkg/service/predict/backtest"
	lstmnn "go-stock-prediction/pkg/service/predict/lstm_nn"
	movingaverage "go-stock-prediction/pkg/service/predict/moving_average"
	"go-stock-prediction/pkg/store/repository"
	"net/http"
)

func GetAlgorithmBacktest(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		ResponseError(w, http.StatusBadRequest, "symbol is required")
		return
	}
	if err := validateSymbol(symbol); err != nil {
		ResponseError(w, http.StatusBadRequest, err.Error())
		return
	}

	store := repository.GetSingleton()
	stock, err := store.GetStockBySymbol(symbol)
	if err != nil {
		ResponseError(w, http.StatusNotFound, "Stock not found")
		return
	}

	allPrices, err := store.GetStockPricesByStockID(stock.ID)
	if err != nil || len(allPrices) < 150 {
		ResponseError(w, http.StatusBadRequest, "insufficient historical data for backtest")
		return
	}

	// Limit to most recent 500 prices; prices are ordered DESC from the store,
	// so reverse to get chronological order (oldest first).
	limit := 500
	if len(allPrices) < limit {
		limit = len(allPrices)
	}
	recent := allPrices[:limit]

	// Convert to float64 slice in chronological order (oldest → newest)
	priceFloats := make([]float64, limit)
	for i := 0; i < limit; i++ {
		f, _ := recent[limit-1-i].ClosePrice.Float64()
		priceFloats[i] = f
	}

	ctx := context.Background()
	algos := []backtest.PredictionAlgorithm{
		movingaverage.NewMovingAveragePredictor(),
		lstmnn.NewLSTMPredictor(),
		arimagarch.NewARIMAGARCHPredictor(),
	}

	var results []modelsapi.BacktestResultDTO
	for _, algo := range algos {
		result := backtest.Run(ctx, algo, priceFloats)
		results = append(results, modelsapi.BacktestResultDTO{
			Algorithm:           result.Algorithm,
			TotalPredictions:    result.TotalPredictions,
			MAE:                 result.MAE,
			RMSE:                result.RMSE,
			MAPE:                result.MAPE,
			DirectionalAccuracy: result.DirectionalAccuracy,
		})
		logger.Logger.Infof("Backtest %s for %s: Dir=%.1f%% MAPE=%.2f%%",
			result.Algorithm, symbol, result.DirectionalAccuracy, result.MAPE)
	}

	ResponseSuccess(w, http.StatusOK, map[string]interface{}{
		"symbol":  symbol,
		"results": results,
	})
}
