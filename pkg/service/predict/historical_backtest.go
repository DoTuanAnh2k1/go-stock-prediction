package predict

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"go-stock-prediction/pkg/logger"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	modelssvc "go-stock-prediction/pkg/models/models_svc"
)

// HistoricalBacktestResult contains statistics from a walk-forward backtest run.
type HistoricalBacktestResult struct {
	TotalPredictions int   `json:"total_predictions"`
	StocksProcessed  int   `json:"stocks_processed"`
	AlgorithmsRun    int   `json:"algorithms_run"`
	DurationMs       int64 `json:"duration_ms"`
}

const (
	backtestDefaultTrainWindow = 30  // initial trading days before first prediction
	backtestDefaultStepSize    = 6   // days per fold (informational)
	backtestMaxHistory         = 270 // max history days fed to algorithm (~9 months)
	backtestBatchSize          = 200 // rows per DB insert batch
)

// RunHistoricalBacktest performs walk-forward backtesting over VN30 stocks.
//
// Algorithm per stock+algorithm pair:
//  1. Sort prices chronologically (oldest → newest)
//  2. Fold 1: train on days [0, trainWindow), predict days [trainWindow, trainWindow+stepSize)
//  3. Fold 2: train on days [0, trainWindow+stepSize), predict next stepSize days
//  4. Continue until end of data
//
// Within each fold, ALL stepSize predictions use the SAME fixed context (end of training window).
// This simulates a real scenario: on day T the model is retrained and used to predict
// the next stepSize days without looking at those days' actual prices.
//
// Predictions are stored with actual_price already filled (since we're backtesting).
// Calling this endpoint clears all predictions with target_date < today first,
// so it is safe to call multiple times.
func RunHistoricalBacktest(ctx context.Context, trainWindow, stepSize int) (*HistoricalBacktestResult, error) {
	if predictor == nil {
		return nil, fmt.Errorf("prediction service not initialized")
	}
	if trainWindow <= 0 {
		trainWindow = backtestDefaultTrainWindow
	}
	if stepSize <= 0 {
		stepSize = backtestDefaultStepSize
	}

	start := time.Now()
	result := &HistoricalBacktestResult{}

	// Clear historical predictions (target_date < today) to avoid duplicates on re-run
	today := time.Now().Truncate(24 * time.Hour)
	if err := predictor.store.DeletePredictionsBeforeDate(today); err != nil {
		logger.Logger.Warnf("Backtest: failed to clear old historical predictions: %v", err)
	} else {
		logger.Logger.Info("Backtest: cleared historical predictions (target_date < today)")
	}

	stocks, err := predictor.store.GetVN30Stocks()
	if err != nil {
		return nil, fmt.Errorf("get VN30 stocks: %w", err)
	}

	logger.Logger.Infof("Backtest: starting walk-forward for %d stocks × %d algorithms, trainWindow=%d",
		len(stocks), len(predictor.algorithms), trainWindow)

	for _, stock := range stocks {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		// Prices from DB are DESC (newest first); reverse to ASC for walk-forward indexing
		pricesDesc, err := predictor.store.GetStockPricesByStockID(stock.ID)
		if err != nil {
			logger.Logger.Errorf("Backtest: get prices for %s: %v", stock.Symbol, err)
			continue
		}

		n := len(pricesDesc)
		if n < trainWindow+1 {
			logger.Logger.Warnf("Backtest: %s has %d prices, need ≥%d — skipping", stock.Symbol, n, trainWindow+1)
			continue
		}

		asc := reverseStockPrices(pricesDesc)

		for algName, algorithm := range predictor.algorithms {
			batch := walkForwardStock(ctx, stock, asc, algName, algorithm, trainWindow, stepSize)
			if len(batch) == 0 {
				continue
			}
			saved := bulkInsertPredictions(batch, backtestBatchSize)
			logger.Logger.Infof("Backtest: %s/%s → %d predictions saved", stock.Symbol, algName, saved)
			result.TotalPredictions += saved
		}

		result.StocksProcessed++
	}

	result.AlgorithmsRun = len(predictor.algorithms)
	result.DurationMs = time.Since(start).Milliseconds()

	logger.Logger.Infof("Backtest: done — %d predictions across %d stocks in %dms",
		result.TotalPredictions, result.StocksProcessed, result.DurationMs)

	return result, nil
}

// walkForwardStock generates all historical predictions for one stock+algorithm pair.
// asc contains prices in chronological order (oldest first).
//
// Batch walk-forward logic:
//   - Fold 1: context = asc[0:trainWindow], predict asc[trainWindow : trainWindow+stepSize]
//   - Fold 2: context = asc[0:trainWindow+stepSize], predict next stepSize days
//   - ...
//
// All predictions within a fold use the SAME fixed context (end of training window),
// simulating "the model was trained on day T and used to forecast the next N days
// without seeing those days' actual prices".
func walkForwardStock(
	ctx context.Context,
	stock modelsdb.Stock,
	asc []modelsdb.StockPrice,
	algName string,
	algorithm PredictionAlgorithm,
	trainWindow, stepSize int,
) []modelsdb.Prediction {

	var out []modelsdb.Prediction
	n := len(asc)

	for windowEnd := trainWindow; windowEnd < n; windowEnd += stepSize {
		// Build fixed context for this fold (capped at backtestMaxHistory)
		histLen := windowEnd
		if histLen > backtestMaxHistory {
			histLen = backtestMaxHistory
		}
		histStart := windowEnd - histLen

		// DESC order (newest first) matching production format
		historical := make([]string, histLen)
		for i := 0; i < histLen; i++ {
			historical[i] = asc[histStart+histLen-1-i].ClosePrice.String()
		}

		// Predict all days in this fold using the SAME context
		foldEnd := windowEnd + stepSize
		if foldEnd > n {
			foldEnd = n
		}

		for t := windowEnd; t < foldEnd; t++ {
			select {
			case <-ctx.Done():
				return out
			default:
			}

			pred, err := algorithm.Predict(ctx, &modelssvc.StockData{Historical: historical})
			if err != nil {
				continue
			}

			trainingEndDay := asc[windowEnd-1] // last day of training window
			targetDay := asc[t]                // day being predicted

			actualPrice := targetDay.ClosePrice
			predictedPrice := decimal.NewFromFloat(pred.PredictedPrice)
			confidence := decimal.NewFromFloat(pred.Confidence)

			accuracy := computeAccuracy(actualPrice, predictedPrice)
			status := resolveStatus(accuracy)

			out = append(out, modelsdb.Prediction{
				StockID:        stock.ID,
				PredictedPrice: predictedPrice,
				CurrentPrice:   trainingEndDay.ClosePrice,
				Confidence:     confidence,
				AlgorithmName:  algName,
				PredictionDate: trainingEndDay.TradingDate,
				TargetDate:     targetDay.TradingDate,
				ActualPrice:    &actualPrice,
				Accuracy:       &accuracy,
				Status:         status,
			})
		}
	}

	return out
}

// reverseStockPrices returns a new slice with elements in reverse order.
func reverseStockPrices(prices []modelsdb.StockPrice) []modelsdb.StockPrice {
	n := len(prices)
	out := make([]modelsdb.StockPrice, n)
	for i := 0; i < n; i++ {
		out[i] = prices[n-1-i]
	}
	return out
}

// bulkInsertPredictions saves predictions in fixed-size batches.
// Returns total number of successfully inserted records.
func bulkInsertPredictions(predictions []modelsdb.Prediction, batchSize int) int {
	total := 0
	for i := 0; i < len(predictions); i += batchSize {
		end := i + batchSize
		if end > len(predictions) {
			end = len(predictions)
		}
		if err := predictor.store.BulkCreatePredictions(predictions[i:end]); err != nil {
			logger.Logger.Errorf("Backtest: bulk insert [%d:%d] failed: %v", i, end, err)
			continue
		}
		total += end - i
	}
	return total
}
