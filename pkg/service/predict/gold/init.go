package goldpredict

import (
	"context"
	"sync"
	"time"

	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/service/predict/orchestrator"
	"go-stock-prediction/pkg/store/repository"
)

var (
	once  sync.Once
	store repository.DatabaseStore
)

// Init initialises the gold prediction service.
// Daily gold prediction is now handled by the unified orchestrator via
// CronjobDailyPrediction in the stock predict package. No separate cron is
// registered here to avoid duplicate prediction runs.
func Init() {
	once.Do(func() {
		logger.Logger.Info("Initializing gold prediction service...")

		store = repository.GetSingleton()
		if store == nil {
			logger.Logger.Fatal("Failed to get database store for gold prediction service")
			return
		}

		// NOTE: No cron job registered here. Daily gold prediction is driven by
		// the orchestrator in pkg/service/predict (Daily6PM → RunAllMarkets →
		// RunForMarket("GOLD")). This avoids duplicate prediction runs.

		logger.Logger.Info("Gold prediction service initialized (orchestrator-driven).")
	})
}

// RunNow triggers gold prediction immediately (for manual /api/trigger/gold-predict).
// Delegates to the orchestrator so the same algorithm + save logic is used as
// the daily cron.
func RunNow() (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	if store == nil {
		store = repository.GetSingleton()
	}

	return orchestrator.RunForMarket(ctx, "GOLD")
}
