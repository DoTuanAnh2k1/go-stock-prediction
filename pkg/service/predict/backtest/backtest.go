package backtest

import (
	"context"
	modelssvc "go-stock-prediction/pkg/models/models_svc"
	"math"
	"strconv"
)

// PredictionAlgorithm mirrors the predict package interface locally to avoid circular imports.
type PredictionAlgorithm interface {
	Predict(ctx context.Context, data *modelssvc.StockData) (*modelssvc.Prediction, error)
	GetName() string
}

// BacktestResult holds walk-forward backtest metrics for a single algorithm.
type BacktestResult struct {
	Algorithm           string
	TotalPredictions    int
	MAE                 float64
	RMSE                float64
	MAPE                float64
	DirectionalAccuracy float64
}

// Run executes a walk-forward backtest on the given price series.
// trainWindow is the number of prices used to train each fold; testWindow is
// the step between folds (the first price after the training window is the actual).
func Run(ctx context.Context, algo PredictionAlgorithm, prices []float64) BacktestResult {
	const trainWindow = 120
	const testWindow = 20

	var squaredErrors []float64
	var absErrors []float64
	var pctErrors []float64
	correct := 0
	total := 0

	for start := 0; start+trainWindow+testWindow <= len(prices); start += testWindow {
		trainPrices := prices[start : start+trainWindow]

		// Convert to []string for StockData
		trainStrs := make([]string, len(trainPrices))
		for i, p := range trainPrices {
			trainStrs[i] = strconv.FormatFloat(p, 'f', 2, 64)
		}
		trainData := &modelssvc.StockData{Historical: trainStrs}

		pred, err := algo.Predict(ctx, trainData)
		if err != nil {
			continue
		}

		actual := prices[start+trainWindow]
		predicted := pred.PredictedPrice

		absErr := math.Abs(actual - predicted)
		pctErr := absErr / actual * 100
		absErrors = append(absErrors, absErr)
		squaredErrors = append(squaredErrors, absErr*absErr)
		pctErrors = append(pctErrors, pctErr)

		// Directional accuracy: was the direction of movement predicted correctly?
		lastTrainPrice := trainPrices[len(trainPrices)-1]
		if (predicted > lastTrainPrice) == (actual > lastTrainPrice) {
			correct++
		}
		total++
	}

	if total == 0 {
		return BacktestResult{Algorithm: algo.GetName()}
	}

	mae := mean(absErrors)
	rmse := math.Sqrt(mean(squaredErrors))
	mape := mean(pctErrors)
	dirAcc := float64(correct) / float64(total) * 100

	return BacktestResult{
		Algorithm:           algo.GetName(),
		TotalPredictions:    total,
		MAE:                 mae,
		RMSE:                rmse,
		MAPE:                mape,
		DirectionalAccuracy: dirAcc,
	}
}

func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}
