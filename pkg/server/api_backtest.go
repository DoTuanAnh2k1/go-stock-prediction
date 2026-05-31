package server

import (
	"math"
	"net/http"

	"go-stock-prediction/pkg/logger"
	modelsapi "go-stock-prediction/pkg/models/models_api"
	"go-stock-prediction/pkg/store/repository"
)

// GetAlgorithmBacktest godoc
//
//	@Summary      Run algorithm backtest for a stock
//	@Description  Returns walk-forward backtest metrics (MAE, RMSE, MAPE, directional accuracy) per algorithm derived from confirmed predictions (status='confirmed') stored in the database. Requires at least 10 confirmed predictions for the symbol.
//	@Tags         Algorithms
//	@Produce      json
//	@Param        symbol  query     string  true  "Stock symbol (e.g. VCB)"
//	@Success      200     {object}  map[string]interface{}
//	@Failure      400     {object}  ResponseFailure
//	@Failure      404     {object}  ResponseFailure
//	@Failure      500     {object}  ResponseFailure
//	@Router       /api/algorithms/backtest [get]
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

	// Fetch all confirmed predictions with actual_price filled in
	confirmed, err := store.GetPredictionsWithActual(&stock.ID, "", 365)
	if err != nil {
		logger.Logger.Errorf("GetPredictionsWithActual for %s: %v", symbol, err)
		ResponseError(w, http.StatusInternalServerError, "Failed to fetch prediction data")
		return
	}
	if len(confirmed) < 10 {
		ResponseError(w, http.StatusBadRequest, "insufficient confirmed predictions for backtest (need at least 10)")
		return
	}

	// Group by algorithm and compute metrics
	type algoData struct {
		absErrors  []float64
		sqrErrors  []float64
		pctErrors  []float64
		correct    int
		total      int
	}
	byAlgo := make(map[string]*algoData)

	for _, p := range confirmed {
		if p.ActualPrice == nil {
			continue
		}
		predicted, _ := p.PredictedPrice.Float64()
		actual, _ := p.ActualPrice.Float64()
		current, _ := p.CurrentPrice.Float64()
		if actual <= 0 {
			continue
		}

		absErr := math.Abs(actual - predicted)
		pctErr := absErr / actual * 100

		ad := byAlgo[p.AlgorithmName]
		if ad == nil {
			ad = &algoData{}
			byAlgo[p.AlgorithmName] = ad
		}
		ad.absErrors = append(ad.absErrors, absErr)
		ad.sqrErrors = append(ad.sqrErrors, absErr*absErr)
		ad.pctErrors = append(ad.pctErrors, pctErr)
		if current > 0 {
			if (predicted > current) == (actual > current) {
				ad.correct++
			}
		}
		ad.total++
	}

	if len(byAlgo) == 0 {
		ResponseError(w, http.StatusBadRequest, "insufficient confirmed predictions for backtest")
		return
	}

	results := make([]modelsapi.BacktestResultDTO, 0, len(byAlgo))
	for algoName, ad := range byAlgo {
		if ad.total == 0 {
			continue
		}
		mae := meanF(ad.absErrors)
		rmse := math.Sqrt(meanF(ad.sqrErrors))
		mape := meanF(ad.pctErrors)
		dirAcc := float64(ad.correct) / float64(ad.total) * 100

		results = append(results, modelsapi.BacktestResultDTO{
			Algorithm:           algoName,
			TotalPredictions:    ad.total,
			MAE:                 mae,
			RMSE:                rmse,
			MAPE:                mape,
			DirectionalAccuracy: dirAcc,
		})
		logger.Logger.Infof("Backtest %s for %s: Dir=%.1f%% MAPE=%.2f%%", algoName, symbol, dirAcc, mape)
	}

	ResponseSuccess(w, http.StatusOK, map[string]interface{}{
		"symbol":  symbol,
		"results": results,
	})
}

func meanF(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}
