package nasdaqpredict

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/shopspring/decimal"

	"go-stock-prediction/pkg/logger"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	modelssvc "go-stock-prediction/pkg/models/models_svc"
	"go-stock-prediction/pkg/service/predict/registry"
	"go-stock-prediction/pkg/store/repository"
)

const (
	nasdaqBacktestDefaultTrainWindow = 30  // initial data points before first prediction
	nasdaqBacktestDefaultStepSize    = 6   // fold size (data points between retraining)
	nasdaqBacktestMaxHistory         = 270 // max history points fed to algorithm
	nasdaqBacktestBatchSize          = 200 // rows per DB insert batch
)

// NasdaqBacktestResult contains statistics from a NASDAQ walk-forward backtest run.
type NasdaqBacktestResult struct {
	TotalPredictions int   `json:"total_predictions"`
	StocksProcessed  int   `json:"stocks_processed"` // represents symbols processed
	DurationMs       int64 `json:"duration_ms"`
}

// RunNasdaqHistoricalBacktest performs walk-forward backtesting over all NASDAQ symbols.
//
// For each symbol + algorithm pair it:
//  1. Fetches all available NASDAQ prices (DESC from DB, reversed to ASC).
//  2. Applies a fold-based walk-forward loop identical to the stock/gold backtest.
//  3. Stores NasdaqPrediction records with actual_price filled in (backtesting).
//
// Passing 0 for trainWindow or stepSize uses the package defaults.
func RunNasdaqHistoricalBacktest(ctx context.Context, trainWindow, stepSize int) (*NasdaqBacktestResult, error) {
	if trainWindow <= 0 {
		trainWindow = nasdaqBacktestDefaultTrainWindow
	}
	if stepSize <= 0 {
		stepSize = nasdaqBacktestDefaultStepSize
	}

	st := repository.GetSingleton()
	if st == nil {
		return nil, fmt.Errorf("database store not initialized")
	}

	start := time.Now()
	result := &NasdaqBacktestResult{}

	// Clear historical NASDAQ predictions (target_date < today) before re-inserting.
	today := time.Now().Truncate(24 * time.Hour)
	if err := st.DeleteNasdaqPredictionsBeforeDate(today); err != nil {
		logger.Logger.Warnf("NasdaqBacktest: failed to clear old historical predictions: %v", err)
	} else {
		logger.Logger.Info("NasdaqBacktest: cleared historical nasdaq predictions (target_date < today)")
	}

	symbols, err := st.GetNasdaqSymbols()
	if err != nil {
		return nil, fmt.Errorf("get nasdaq symbols: %w", err)
	}

	algos := registry.Build()
	logger.Logger.Infof("NasdaqBacktest: starting walk-forward for %d symbols × %d algorithms, trainWindow=%d",
		len(symbols), len(algos), trainWindow)

	for _, symbol := range symbols {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		// DB returns DESC; reverse to ASC for walk-forward indexing.
		pricesDesc, err := st.GetAllNasdaqPricesForSymbol(symbol)
		if err != nil {
			logger.Logger.Errorf("NasdaqBacktest: fetch prices for %s: %v", symbol, err)
			continue
		}

		n := len(pricesDesc)
		if n < trainWindow+1 {
			logger.Logger.Warnf("NasdaqBacktest: %s has %d prices, need ≥%d — skipping", symbol, n, trainWindow+1)
			continue
		}

		asc := reverseNasdaqPrices(pricesDesc)

		for algName, algo := range algos {
			batch := walkForwardNasdaq(ctx, symbol, asc, algName, algo, trainWindow, stepSize)
			if len(batch) == 0 {
				continue
			}
			saved := bulkInsertNasdaqPredictions(st, batch, nasdaqBacktestBatchSize)
			logger.Logger.Infof("NasdaqBacktest: %s/%s → %d predictions saved", symbol, algName, saved)
			result.TotalPredictions += saved
		}

		result.StocksProcessed++
	}

	result.DurationMs = time.Since(start).Milliseconds()
	logger.Logger.Infof("NasdaqBacktest: done — %d predictions across %d symbols in %dms",
		result.TotalPredictions, result.StocksProcessed, result.DurationMs)

	return result, nil
}

// walkForwardNasdaq generates all historical predictions for one NASDAQ symbol + algorithm pair.
// asc contains prices in chronological order (oldest first).
func walkForwardNasdaq(
	ctx context.Context,
	symbol string,
	asc []modelsdb.NasdaqPrice,
	algName string,
	algo interface {
		Predict(context.Context, *modelssvc.StockData) (*modelssvc.Prediction, error)
	},
	trainWindow, stepSize int,
) []modelsdb.NasdaqPrediction {

	var out []modelsdb.NasdaqPrediction
	n := len(asc)

	for windowEnd := trainWindow; windowEnd < n; windowEnd += stepSize {
		// Build fixed context for this fold (capped at nasdaqBacktestMaxHistory).
		histLen := windowEnd
		if histLen > nasdaqBacktestMaxHistory {
			histLen = nasdaqBacktestMaxHistory
		}
		histStart := windowEnd - histLen

		// ASC order (oldest first, newest last) — algorithms use prices[len-1] as current price.
		historical := make([]string, histLen)
		for i := 0; i < histLen; i++ {
			f, _ := asc[histStart+i].ClosePrice.Float64()
			historical[i] = strconv.FormatFloat(f, 'f', -1, 64)
		}

		// Predict all days in this fold using the SAME fixed context.
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

			pred, err := algo.Predict(ctx, &modelssvc.StockData{Historical: historical})
			if err != nil {
				continue
			}

			trainingEndDay := asc[windowEnd-1] // last day of training window
			targetDay := asc[t]                // day being predicted

			actualPrice := targetDay.ClosePrice
			predictedPrice := decimal.NewFromFloat(pred.PredictedPrice)
			confidence := decimal.NewFromFloat(pred.Confidence)

			accuracy := computeNasdaqAccuracy(actualPrice, predictedPrice)

			out = append(out, modelsdb.NasdaqPrediction{
				Symbol:         symbol,
				AlgorithmName:  algName,
				PredictedPrice: predictedPrice,
				CurrentPrice:   trainingEndDay.ClosePrice,
				Confidence:     confidence,
				PredictionDate: trainingEndDay.TradingDate,
				TargetDate:     targetDay.TradingDate,
				ActualPrice:    &actualPrice,
				Accuracy:       &accuracy,
			})
		}
	}

	return out
}

// computeNasdaqAccuracy returns accuracy as 1 - abs(predicted-actual)/actual, clamped to [0,1].
func computeNasdaqAccuracy(actual, predicted decimal.Decimal) decimal.Decimal {
	if actual.IsZero() {
		return decimal.Zero
	}
	diff := predicted.Sub(actual).Abs()
	ratio := diff.Div(actual)
	one := decimal.NewFromInt(1)
	acc := one.Sub(ratio)
	if acc.LessThan(decimal.Zero) {
		return decimal.Zero
	}
	return acc
}

// reverseNasdaqPrices returns a new slice with elements in reverse order.
func reverseNasdaqPrices(prices []modelsdb.NasdaqPrice) []modelsdb.NasdaqPrice {
	n := len(prices)
	out := make([]modelsdb.NasdaqPrice, n)
	for i := 0; i < n; i++ {
		out[i] = prices[n-1-i]
	}
	return out
}

// bulkInsertNasdaqPredictions saves NASDAQ predictions in fixed-size batches.
// Returns total number of successfully inserted records.
func bulkInsertNasdaqPredictions(st repository.DatabaseStore, preds []modelsdb.NasdaqPrediction, batchSize int) int {
	total := 0
	for i := 0; i < len(preds); i += batchSize {
		end := i + batchSize
		if end > len(preds) {
			end = len(preds)
		}
		if err := st.BulkCreateNasdaqPredictions(preds[i:end]); err != nil {
			logger.Logger.Errorf("NasdaqBacktest: bulk insert [%d:%d] failed: %v", i, end, err)
			continue
		}
		total += end - i
	}
	return total
}
