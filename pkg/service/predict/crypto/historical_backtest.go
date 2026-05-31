package cryptopredict

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
	cryptoBacktestDefaultTrainWindow = 30  // initial data points before first prediction
	cryptoBacktestDefaultStepSize    = 6   // fold size (data points between retraining)
	cryptoBacktestMaxHistory         = 270 // max history points fed to algorithm
	cryptoBacktestBatchSize          = 200 // rows per DB insert batch
)

// cryptoInstrumentList defines the coins to backtest.
// CoinID matches the coin_id column in crypto_prices; Symbol is the display ticker.
var cryptoInstrumentList = []struct {
	CoinID string
	Symbol string
}{
	{"bitcoin", "BTC"},
	{"ethereum", "ETH"},
}

// CryptoBacktestResult contains statistics from a crypto walk-forward backtest run.
type CryptoBacktestResult struct {
	TotalPredictions int   `json:"total_predictions"`
	StocksProcessed  int   `json:"stocks_processed"` // represents coins processed
	DurationMs       int64 `json:"duration_ms"`
}

// RunCryptoHistoricalBacktest performs walk-forward backtesting over all tracked crypto coins.
//
// For each coin + algorithm pair it:
//  1. Fetches all available crypto prices (DESC from DB, reversed to ASC).
//  2. Applies a fold-based walk-forward loop identical to the stock/gold backtest.
//  3. Stores CryptoPrediction records with actual_price filled in (backtesting).
//
// Passing 0 for trainWindow or stepSize uses the package defaults.
func RunCryptoHistoricalBacktest(ctx context.Context, trainWindow, stepSize int) (*CryptoBacktestResult, error) {
	if trainWindow <= 0 {
		trainWindow = cryptoBacktestDefaultTrainWindow
	}
	if stepSize <= 0 {
		stepSize = cryptoBacktestDefaultStepSize
	}

	st := repository.GetSingleton()
	if st == nil {
		return nil, fmt.Errorf("database store not initialized")
	}

	start := time.Now()
	result := &CryptoBacktestResult{}

	// Clear historical crypto predictions (target_date < today) before re-inserting.
	today := time.Now().Truncate(24 * time.Hour)
	if err := st.DeleteCryptoPredictionsBeforeDate(today); err != nil {
		logger.Logger.Warnf("CryptoBacktest: failed to clear old historical predictions: %v", err)
	} else {
		logger.Logger.Info("CryptoBacktest: cleared historical crypto predictions (target_date < today)")
	}

	algos := registry.Build()
	logger.Logger.Infof("CryptoBacktest: starting walk-forward for %d coins × %d algorithms, trainWindow=%d",
		len(cryptoInstrumentList), len(algos), trainWindow)

	for _, inst := range cryptoInstrumentList {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		// DB returns DESC; reverse to ASC for walk-forward indexing.
		pricesDesc, err := st.GetAllCryptoPricesForCoin(inst.CoinID)
		if err != nil {
			logger.Logger.Errorf("CryptoBacktest: fetch prices for %s: %v", inst.CoinID, err)
			continue
		}

		n := len(pricesDesc)
		if n < trainWindow+1 {
			logger.Logger.Warnf("CryptoBacktest: %s has %d prices, need ≥%d — skipping", inst.CoinID, n, trainWindow+1)
			continue
		}

		asc := reverseCryptoPrices(pricesDesc)

		for algName, algo := range algos {
			batch := walkForwardCrypto(ctx, inst.CoinID, inst.Symbol, asc, algName, algo, trainWindow, stepSize)
			if len(batch) == 0 {
				continue
			}
			saved := bulkInsertCryptoPredictions(st, batch, cryptoBacktestBatchSize)
			logger.Logger.Infof("CryptoBacktest: %s/%s → %d predictions saved", inst.CoinID, algName, saved)
			result.TotalPredictions += saved
		}

		result.StocksProcessed++
	}

	result.DurationMs = time.Since(start).Milliseconds()
	logger.Logger.Infof("CryptoBacktest: done — %d predictions across %d coins in %dms",
		result.TotalPredictions, result.StocksProcessed, result.DurationMs)

	return result, nil
}

// walkForwardCrypto generates all historical predictions for one crypto coin + algorithm pair.
// asc contains prices in chronological order (oldest first).
func walkForwardCrypto(
	ctx context.Context,
	coinID, symbol string,
	asc []modelsdb.CryptoPrice,
	algName string,
	algo interface {
		Predict(context.Context, *modelssvc.StockData) (*modelssvc.Prediction, error)
	},
	trainWindow, stepSize int,
) []modelsdb.CryptoPrediction {

	var out []modelsdb.CryptoPrediction
	n := len(asc)

	for windowEnd := trainWindow; windowEnd < n; windowEnd += stepSize {
		// Build fixed context for this fold (capped at cryptoBacktestMaxHistory).
		histLen := windowEnd
		if histLen > cryptoBacktestMaxHistory {
			histLen = cryptoBacktestMaxHistory
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

			accuracy := computeCryptoAccuracy(actualPrice, predictedPrice)

			out = append(out, modelsdb.CryptoPrediction{
				CoinID:         coinID,
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

// computeCryptoAccuracy returns accuracy as 1 - abs(predicted-actual)/actual, clamped to [0,1].
func computeCryptoAccuracy(actual, predicted decimal.Decimal) decimal.Decimal {
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

// reverseCryptoPrices returns a new slice with elements in reverse order.
func reverseCryptoPrices(prices []modelsdb.CryptoPrice) []modelsdb.CryptoPrice {
	n := len(prices)
	out := make([]modelsdb.CryptoPrice, n)
	for i := 0; i < n; i++ {
		out[i] = prices[n-1-i]
	}
	return out
}

// bulkInsertCryptoPredictions saves crypto predictions in fixed-size batches.
// Returns total number of successfully inserted records.
func bulkInsertCryptoPredictions(st repository.DatabaseStore, preds []modelsdb.CryptoPrediction, batchSize int) int {
	total := 0
	for i := 0; i < len(preds); i += batchSize {
		end := i + batchSize
		if end > len(preds) {
			end = len(preds)
		}
		if err := st.BulkCreateCryptoPredictions(preds[i:end]); err != nil {
			logger.Logger.Errorf("CryptoBacktest: bulk insert [%d:%d] failed: %v", i, end, err)
			continue
		}
		total += end - i
	}
	return total
}
