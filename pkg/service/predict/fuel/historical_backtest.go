package fuelpredict

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
	// fuelBacktestDefaultTrainWindow is lower than stock/gold because fuel prices
	// are updated ~biweekly (~26 periods/year), giving fewer data points per year.
	fuelBacktestDefaultTrainWindow = 15  // initial data points before first prediction
	fuelBacktestDefaultStepSize    = 4   // fold size (data points between retraining)
	fuelBacktestMaxHistory         = 120 // max history points fed to algorithm (~5 years of biweekly data)
	fuelBacktestBatchSize          = 200 // rows per DB insert batch
)

// fuelProductList defines the product types to backtest.
// These match product_type values stored in fuel_prices table.
var fuelProductList = []string{
	"ron95_iii",
	"e5_ron92",
	"do_005s",
	"kerosene",
}

// FuelBacktestResult contains statistics from a fuel walk-forward backtest run.
type FuelBacktestResult struct {
	TotalPredictions int   `json:"total_predictions"`
	StocksProcessed  int   `json:"stocks_processed"` // represents product types processed
	DurationMs       int64 `json:"duration_ms"`
}

// RunFuelHistoricalBacktest performs walk-forward backtesting over all tracked fuel products.
//
// For each product + algorithm pair it:
//  1. Fetches all available fuel prices (DESC from DB, reversed to ASC).
//  2. Applies a fold-based walk-forward loop identical to the stock/gold backtest.
//  3. Stores FuelPrediction records with actual_price filled in (backtesting).
//
// Fuel data is sparse (~biweekly intervals), so trainWindow defaults to 15 instead of 30.
// Passing 0 for trainWindow or stepSize uses the package defaults.
func RunFuelHistoricalBacktest(ctx context.Context, trainWindow, stepSize int) (*FuelBacktestResult, error) {
	if trainWindow <= 0 {
		trainWindow = fuelBacktestDefaultTrainWindow
	}
	if stepSize <= 0 {
		stepSize = fuelBacktestDefaultStepSize
	}

	st := repository.GetSingleton()
	if st == nil {
		return nil, fmt.Errorf("database store not initialized")
	}

	start := time.Now()
	result := &FuelBacktestResult{}

	// Clear historical fuel predictions (target_date < today) before re-inserting.
	today := time.Now().Truncate(24 * time.Hour)
	if err := st.DeleteFuelPredictionsBeforeDate(today); err != nil {
		logger.Logger.Warnf("FuelBacktest: failed to clear old historical predictions: %v", err)
	} else {
		logger.Logger.Info("FuelBacktest: cleared historical fuel predictions (target_date < today)")
	}

	algos := registry.Build()
	logger.Logger.Infof("FuelBacktest: starting walk-forward for %d products × %d algorithms, trainWindow=%d",
		len(fuelProductList), len(algos), trainWindow)

	for _, productType := range fuelProductList {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		// DB returns DESC; reverse to ASC for walk-forward indexing.
		pricesDesc, err := st.GetAllFuelPricesForProduct(productType)
		if err != nil {
			logger.Logger.Errorf("FuelBacktest: fetch prices for %s: %v", productType, err)
			continue
		}

		n := len(pricesDesc)
		if n < trainWindow+1 {
			logger.Logger.Warnf("FuelBacktest: %s has %d prices, need ≥%d — skipping", productType, n, trainWindow+1)
			continue
		}

		asc := reverseFuelPrices(pricesDesc)

		for algName, algo := range algos {
			batch := walkForwardFuel(ctx, productType, asc, algName, algo, trainWindow, stepSize)
			if len(batch) == 0 {
				continue
			}
			saved := bulkInsertFuelPredictions(st, batch, fuelBacktestBatchSize)
			logger.Logger.Infof("FuelBacktest: %s/%s → %d predictions saved", productType, algName, saved)
			result.TotalPredictions += saved
		}

		result.StocksProcessed++
	}

	result.DurationMs = time.Since(start).Milliseconds()
	logger.Logger.Infof("FuelBacktest: done — %d predictions across %d products in %dms",
		result.TotalPredictions, result.StocksProcessed, result.DurationMs)

	return result, nil
}

// walkForwardFuel generates all historical predictions for one fuel product + algorithm pair.
// asc contains prices in chronological order (oldest first).
func walkForwardFuel(
	ctx context.Context,
	productType string,
	asc []modelsdb.FuelPrice,
	algName string,
	algo interface {
		Predict(context.Context, *modelssvc.StockData) (*modelssvc.Prediction, error)
	},
	trainWindow, stepSize int,
) []modelsdb.FuelPrediction {

	var out []modelsdb.FuelPrediction
	n := len(asc)

	for windowEnd := trainWindow; windowEnd < n; windowEnd += stepSize {
		// Build fixed context for this fold (capped at fuelBacktestMaxHistory).
		histLen := windowEnd
		if histLen > fuelBacktestMaxHistory {
			histLen = fuelBacktestMaxHistory
		}
		histStart := windowEnd - histLen

		// ASC order (oldest first, newest last) — algorithms use prices[len-1] as current price.
		// Fuel uses Price field (not ClosePrice).
		historical := make([]string, histLen)
		for i := 0; i < histLen; i++ {
			f, _ := asc[histStart+i].Price.Float64()
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

			actualPrice := targetDay.Price
			predictedPrice := decimal.NewFromFloat(pred.PredictedPrice)
			confidence := decimal.NewFromFloat(pred.Confidence)

			accuracy := computeFuelAccuracy(actualPrice, predictedPrice)

			out = append(out, modelsdb.FuelPrediction{
				ProductType:    productType,
				AlgorithmName:  algName,
				PredictedPrice: predictedPrice,
				CurrentPrice:   trainingEndDay.Price,
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

// computeFuelAccuracy returns accuracy as 1 - abs(predicted-actual)/actual, clamped to [0,1].
func computeFuelAccuracy(actual, predicted decimal.Decimal) decimal.Decimal {
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

// reverseFuelPrices returns a new slice with elements in reverse order.
func reverseFuelPrices(prices []modelsdb.FuelPrice) []modelsdb.FuelPrice {
	n := len(prices)
	out := make([]modelsdb.FuelPrice, n)
	for i := 0; i < n; i++ {
		out[i] = prices[n-1-i]
	}
	return out
}

// bulkInsertFuelPredictions saves fuel predictions in fixed-size batches.
// Returns total number of successfully inserted records.
func bulkInsertFuelPredictions(st repository.DatabaseStore, preds []modelsdb.FuelPrediction, batchSize int) int {
	total := 0
	for i := 0; i < len(preds); i += batchSize {
		end := i + batchSize
		if end > len(preds) {
			end = len(preds)
		}
		if err := st.BulkCreateFuelPredictions(preds[i:end]); err != nil {
			logger.Logger.Errorf("FuelBacktest: bulk insert [%d:%d] failed: %v", i, end, err)
			continue
		}
		total += end - i
	}
	return total
}
