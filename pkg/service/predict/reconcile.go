package predict

import (
	"context"
	"fmt"
	"go-stock-prediction/pkg/logger"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"time"

	"github.com/shopspring/decimal"
)

// CronjobDailyReconcile is the cron entry point for daily prediction reconciliation.
func CronjobDailyReconcile() error {
	logger.Logger.Info("ReconcilePredictions: starting daily reconcile cronjob...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	return ReconcilePredictions(ctx)
}

// ReconcilePredictions looks up all pending predictions whose target_date has
// passed, fetches the actual closing price from stock_prices, computes the
// accuracy, and updates the prediction record accordingly.
//
// A prediction is considered "pending" when actual_price IS NULL and
// target_date <= today (handled by GetPendingPredictions).
//
// Accuracy formula:
//
//	diff     = |actual - predicted|
//	accuracy = max(0, 100 - diff / actual * 100)   (0 when actual == 0)
//
// Status:
//
//	accuracy >= 70  → "confirmed"
//	accuracy < 70   → "wrong"
//	no price found  → skip (stay pending)
func ReconcilePredictions(ctx context.Context) error {
	if predictor == nil {
		return fmt.Errorf("prediction service not initialized")
	}

	store := predictor.store
	cutoff := time.Now()

	pending, err := store.GetPendingPredictions(cutoff)
	if err != nil {
		return fmt.Errorf("failed to fetch pending predictions: %w", err)
	}

	if len(pending) == 0 {
		logger.Logger.Info("ReconcilePredictions: no pending predictions to reconcile")
		return nil
	}

	logger.Logger.Infof("ReconcilePredictions: reconciling %d pending predictions", len(pending))

	successCount := 0
	skipCount := 0
	errorCount := 0

	for _, p := range pending {
		select {
		case <-ctx.Done():
			logger.Logger.Warn("ReconcilePredictions: context cancelled, stopping early")
			return ctx.Err()
		default:
		}

		price, err := findActualPrice(store, p)
		if err != nil {
			logger.Logger.Debugf("ReconcilePredictions: no price for prediction id=%d stock_id=%d target=%s: %v",
				p.ID, p.StockID, p.TargetDate.Format("2006-01-02"), err)
			skipCount++
			continue
		}

		actual := price.ClosePrice
		predicted := p.PredictedPrice

		accuracy := computeAccuracy(actual, predicted)
		status := resolveStatus(accuracy)

		if err := store.UpdatePredictionActual(p.ID, &actual, &accuracy, status); err != nil {
			logger.Logger.Errorf("ReconcilePredictions: failed to update prediction id=%d: %v", p.ID, err)
			errorCount++
			continue
		}

		successCount++
		logger.Logger.Debugf("ReconcilePredictions: updated id=%d actual=%s predicted=%s accuracy=%s status=%s",
			p.ID, actual.String(), predicted.String(), accuracy.String(), status)
	}

	logger.Logger.Infof("ReconcilePredictions: done — updated=%d skipped=%d errors=%d",
		successCount, skipCount, errorCount)

	return nil
}

// findActualPrice tries to get the closing price for a prediction's target date.
// It searches within a ±3-day window to account for weekends / holidays.
func findActualPrice(store interface {
	GetStockPriceByStockIDAndDate(stockID uint, tradingDate time.Time) (*modelsdb.StockPrice, error)
	GetStockPricesByStockIDAndDateRange(stockID uint, fromDate, toDate time.Time) ([]modelsdb.StockPrice, error)
}, p modelsdb.Prediction) (*modelsdb.StockPrice, error) {
	// Try exact date first
	price, err := store.GetStockPriceByStockIDAndDate(p.StockID, p.TargetDate)
	if err == nil && price != nil {
		return price, nil
	}

	// Fall back: search within ±3 calendar days (handles weekends/holidays)
	from := p.TargetDate.AddDate(0, 0, -3)
	to := p.TargetDate.AddDate(0, 0, 3)
	prices, err := store.GetStockPricesByStockIDAndDateRange(p.StockID, from, to)
	if err != nil {
		return nil, fmt.Errorf("range query failed: %w", err)
	}
	if len(prices) == 0 {
		return nil, fmt.Errorf("no price data around target_date=%s", p.TargetDate.Format("2006-01-02"))
	}

	// Pick the price record closest to target_date
	best := prices[0]
	bestDiff := absDuration(prices[0].TradingDate.Sub(p.TargetDate))
	for _, pr := range prices[1:] {
		d := absDuration(pr.TradingDate.Sub(p.TargetDate))
		if d < bestDiff {
			best = pr
			bestDiff = d
		}
	}
	return &best, nil
}

// computeAccuracy returns a value in [0, 1] representing how close the
// predicted price was to the actual price (stored as DECIMAL(5,4) fraction).
// e.g. 0.7700 = 77% accurate.
func computeAccuracy(actual, predicted decimal.Decimal) decimal.Decimal {
	if actual.IsZero() {
		return decimal.Zero
	}

	diff := actual.Sub(predicted).Abs()
	// accuracy fraction = max(0, 1 - diff/actual)
	accuracy := decimal.NewFromInt(1).Sub(diff.Div(actual))

	if accuracy.LessThan(decimal.Zero) {
		return decimal.Zero
	}
	return accuracy
}

// resolveStatus maps accuracy fraction to a human-readable status string.
// threshold: 0.70 = 70%
func resolveStatus(accuracy decimal.Decimal) string {
	threshold := decimal.NewFromFloat(0.70)
	if accuracy.GreaterThanOrEqual(threshold) {
		return "confirmed"
	}
	return "wrong"
}

// absDuration returns the absolute value of a time.Duration.
func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
