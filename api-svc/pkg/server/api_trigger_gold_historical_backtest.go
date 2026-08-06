package server

import (
	"net/http"

	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TriggerGoldHistoricalBacktestHandler godoc
//
//	@Summary      Trigger gold historical backtest
//	@Description  Starts a walk-forward backtest over all gold instruments (XAU/spot, BTMC/sjc, BTMC/nhan_tron) in the background. Returns 202 immediately. Returns 409 if a backtest is already running.
//	@Tags         Triggers
//	@Accept       json
//	@Produce      json
//	@Success      202  {object}  map[string]string
//	@Failure      401  {object}  ResponseFailure
//	@Failure      409  {object}  ResponseFailure
//	@Failure      500  {object}  ResponseFailure
//	@Failure      503  {object}  ResponseFailure
//	@Security     BearerAuth
//	@Router       /api/trigger/gold-historical-backtest [post]
func TriggerGoldHistoricalBacktestHandler(w http.ResponseWriter, r *http.Request) {
	logger.Ctx(r.Context()).Info("TriggerGoldHistoricalBacktestHandler: requesting gold historical backtest")

	client := requireGRPCClient(w)
	if client == nil {
		return
	}

	// train_window = -1 signals gold backtest to the prediction service.
	_, err := client.TriggerHistoricalBacktest(r.Context(), &pb.BacktestRequest{
		TrainWindow: -1,
		StepSize:    0,
	})
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.Aborted {
			ResponseError(w, http.StatusConflict, st.Message())
			return
		}
		logger.Ctx(r.Context()).Errorf("GoldHistoricalBacktest: failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}

	ResponseSuccess(w, http.StatusAccepted, map[string]string{
		"status":  "started",
		"message": "Gold historical backtest running in background. Check logs for progress.",
	})
}
