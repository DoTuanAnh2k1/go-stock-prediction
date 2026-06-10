package server

import (
	"net/http"
	"strconv"

	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TriggerHistoricalBacktestHandler godoc
//
//	@Summary      Trigger walk-forward historical backtest
//	@Description  Starts a walk-forward backtest in the background via the prediction service. Accepts optional query params: train_window (default 30), step_size (default 6), and market (one of "", "VN30", "GOLD", "NASDAQ100", "CRYPTO", "SP500", "ALL"). Returns 409 if a backtest is already running.
//	@Tags         Triggers
//	@Accept       json
//	@Produce      json
//	@Param        train_window  query     int     false  "Minimum data points before first prediction (default 30)"
//	@Param        step_size     query     int     false  "Fold size — data points between retraining (default 6)"
//	@Param        market        query     string  false  "Target market key: VN30, GOLD, NASDAQ100, CRYPTO, SP500, ALL (default: all)"
//	@Success      202           {object}  map[string]string
//	@Failure      401           {object}  ResponseFailure
//	@Failure      409           {object}  ResponseFailure
//	@Failure      500           {object}  ResponseFailure
//	@Failure      503           {object}  ResponseFailure
//	@Security     BearerAuth
//	@Router       /api/trigger/historical-backtest [post]
func TriggerHistoricalBacktestHandler(w http.ResponseWriter, r *http.Request) {
	trainWindow := queryInt(r, "train_window", 30)
	stepSize := queryInt(r, "step_size", 6)
	marketKey := r.URL.Query().Get("market")

	logger.Logger.Infof("TriggerHistoricalBacktestHandler: requesting backtest, market=%q train_window=%d step_size=%d",
		marketKey, trainWindow, stepSize)

	client := requireGRPCClient(w)
	if client == nil {
		return
	}
	_, err := client.TriggerHistoricalBacktest(r.Context(), &pb.BacktestRequest{
		TrainWindow: int32(trainWindow),
		StepSize:    int32(stepSize),
		MarketKey:   marketKey,
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

	msg := "Historical backtest running in background. Check app logs for progress."
	if marketKey != "" {
		msg = marketKey + " historical backtest running in background. Check app logs for progress."
	}
	ResponseSuccess(w, http.StatusAccepted, map[string]string{
		"status":  "started",
		"message": msg,
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
