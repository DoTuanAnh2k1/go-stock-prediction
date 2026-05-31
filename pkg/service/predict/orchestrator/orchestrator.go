// Package orchestrator runs predictions for all registered markets.
// This is the single entry point for daily prediction across stocks, gold, and future markets.
package orchestrator

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/service/market"
	modelssvc "go-stock-prediction/pkg/models/models_svc"
	"go-stock-prediction/pkg/service/predict/registry"
)

// RunAllMarkets runs predictions for every registered market.
// Called by the daily 6PM cron job.
func RunAllMarkets(ctx context.Context) (int, error) {
	total := 0
	var lastErr error
	for _, mkt := range market.All() {
		n, err := RunForMarket(ctx, mkt.MarketKey())
		if err != nil {
			logger.Logger.Errorf("Prediction failed for market %s: %v", mkt.MarketKey(), err)
			lastErr = err
		}
		total += n
	}
	return total, lastErr
}

// RunForMarket runs predictions for a single market by key.
// Called by gRPC TriggerPredict (for specific market) and manual triggers.
func RunForMarket(ctx context.Context, marketKey string) (int, error) {
	mkt, ok := market.Get(marketKey)
	if !ok {
		return 0, fmt.Errorf("market %q not registered", marketKey)
	}
	return runPredictionsForMarket(ctx, mkt)
}

func runPredictionsForMarket(ctx context.Context, mkt market.AssetMarket) (int, error) {
	algos := registry.Build()

	instruments, err := mkt.GetInstruments(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get instruments for %s: %v", mkt.MarketKey(), err)
	}

	logger.Logger.Infof("Running predictions for %s: %d instruments, %d algorithms",
		mkt.MarketName(), len(instruments), len(algos))

	total := 0
	targetDate := time.Now().AddDate(0, 0, 1)

	for _, inst := range instruments {
		prices, err := mkt.FetchPrices(ctx, inst, 9) // 9 months of history
		if err != nil {
			logger.Logger.Debugf("Skipping %s/%s — fetch error: %v", mkt.MarketKey(), inst.ID, err)
			continue
		}
		if len(prices) < 20 {
			logger.Logger.Debugf("Skipping %s/%s — only %d data points", mkt.MarketKey(), inst.ID, len(prices))
			continue
		}

		// Convert []float64 → []string for algorithm input
		historical := make([]string, len(prices))
		for i, p := range prices {
			historical[i] = strconv.FormatFloat(p, 'f', -1, 64)
		}
		data := &modelssvc.StockData{Historical: historical}

		for algName, algo := range algos {
			result, err := algo.Predict(ctx, data)
			if err != nil {
				logger.Logger.Debugf("Predict failed %s/%s (%s): %v", mkt.MarketKey(), inst.ID, algName, err)
				continue
			}

			confidence := result.Confidence
			if confidence == 0 {
				confidence = algo.GetAccuracy()
			}

			if err := mkt.SavePrediction(ctx, inst, market.PredictionData{
				AlgorithmName:  algName,
				PredictedPrice: result.PredictedPrice,
				CurrentPrice:   result.CurrentPrice,
				Confidence:     confidence,
				TargetDate:     targetDate,
			}); err != nil {
				logger.Logger.Errorf("Save prediction failed %s/%s (%s): %v", mkt.MarketKey(), inst.ID, algName, err)
				continue
			}
			total++
		}
	}

	logger.Logger.Infof("Completed predictions for %s: %d saved", mkt.MarketName(), total)
	return total, nil
}
