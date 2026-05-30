package server

import (
	"net/http"
	"strconv"

	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TriggerHistoricalBacktestHandler handles POST /api/trigger/historical-backtest
//
// Delegates the walk-forward backtest to the prediction microservice via gRPC.
// Returns 409 if a backtest is already running (codes.Aborted from server).
//
// Optional query params:
//
//	train_window (int, default 30): minimum trading days before first prediction
//	step_size    (int, default 6):  fold size (days between retraining)
func TriggerHistoricalBacktestHandler(w http.ResponseWriter, r *http.Request) {
	trainWindow := queryInt(r, "train_window", 30)
	stepSize := queryInt(r, "step_size", 6)

	logger.Logger.Infof("TriggerHistoricalBacktestHandler: requesting backtest, train_window=%d step_size=%d", trainWindow, stepSize)

	client := requireGRPCClient(w)
	if client == nil {
		return
	}
	_, err := client.TriggerHistoricalBacktest(r.Context(), &pb.BacktestRequest{
		TrainWindow: int32(trainWindow),
		StepSize:    int32(stepSize),
	})
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.Aborted {
			ResponseError(w, http.StatusConflict, st.Message())
			return
		}
		logger.Logger.Errorf("HistoricalBacktest: failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}

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
