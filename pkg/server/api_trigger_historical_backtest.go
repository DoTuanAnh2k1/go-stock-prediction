package server

import (
	"context"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/service/predict"
)

// backtestRunning is 1 while a backtest is in progress, 0 otherwise.
var backtestRunning atomic.Int32

// TriggerHistoricalBacktestHandler handles POST /api/trigger/historical-backtest
//
// Returns 202 immediately and runs the walk-forward backtest in a background goroutine.
// Returns 409 if a backtest is already running.
//
// Optional query params:
//
//	train_window (int, default 30): minimum trading days before first prediction
//	step_size    (int, default 6):  fold size (days between retraining)
func TriggerHistoricalBacktestHandler(w http.ResponseWriter, r *http.Request) {
	if !backtestRunning.CompareAndSwap(0, 1) {
		ResponseError(w, http.StatusConflict, "historical backtest already running")
		return
	}

	trainWindow := queryInt(r, "train_window", 30)
	stepSize := queryInt(r, "step_size", 6)

	logger.Logger.Infof("TriggerHistoricalBacktestHandler: starting in background, train_window=%d step_size=%d", trainWindow, stepSize)

	go func() {
		defer backtestRunning.Store(0)
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer cancel()

		res, err := predict.RunHistoricalBacktest(ctx, trainWindow, stepSize)
		if err != nil {
			logger.Logger.Errorf("HistoricalBacktest: failed: %v", err)
			return
		}
		logger.Logger.Infof("HistoricalBacktest: done — %d predictions, %d stocks, %dms",
			res.TotalPredictions, res.StocksProcessed, res.DurationMs)
	}()

	ResponseSuccess(w, http.StatusAccepted, map[string]string{
		"status":  "started",
		"message": "Historical backtest running in background. Check app logs for progress.",
	})
}

// queryInt reads an integer query parameter, returning defaultVal on missing/invalid input.
func queryInt(r *http.Request, name string, defaultVal int) int {
	s := r.URL.Query().Get(name)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return defaultVal
	}
	return v
}
