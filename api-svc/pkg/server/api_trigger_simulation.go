package server

import (
	"encoding/json"
	"net/http"

	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"
)

// TriggerSimulationBacktestHandler godoc
//
//	@Summary      Trigger trading simulation backtest
//	@Description  Sends a gRPC request to run backtest simulation for a specific bot or all bots. Accepts optional JSON body with bot_id, start_date, end_date.
//	@Tags         Triggers
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        body  body      object  false  "Optional: {bot_id: '', start_date: '2024-01-01', end_date: ''}"
//	@Success      200   {object}  map[string]string
//	@Failure      401   {object}  ResponseFailure
//	@Failure      500   {object}  ResponseFailure
//	@Failure      503   {object}  ResponseFailure
//	@Router       /api/trigger/simulation-backtest [post]
func TriggerSimulationBacktestHandler(w http.ResponseWriter, r *http.Request) {
	client := requireGRPCClient(w)
	if client == nil {
		return
	}

	var body struct {
		BotID     string `json:"bot_id"`
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
	}
	if r.Body != nil && r.ContentLength != 0 {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}

	resp, err := client.TriggerSimulationBacktest(r.Context(), &pb.SimulationRequest{
		BotId:     body.BotID,
		StartDate: body.StartDate,
		EndDate:   body.EndDate,
	})
	if err != nil {
		logger.Logger.Errorf("TriggerSimulationBacktest: %v", err)
		ResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ResponseSuccess(w, http.StatusOK, map[string]string{"message": resp.Message})
}

// TriggerSimulationLiveStepHandler godoc
//
//	@Summary      Trigger simulation live step
//	@Description  Sends a gRPC request to advance all live simulation bots by one step using the latest available predictions.
//	@Tags         Triggers
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200  {object}  map[string]string
//	@Failure      401  {object}  ResponseFailure
//	@Failure      500  {object}  ResponseFailure
//	@Failure      503  {object}  ResponseFailure
//	@Router       /api/trigger/simulation-live-step [post]
func TriggerSimulationLiveStepHandler(w http.ResponseWriter, r *http.Request) {
	client := requireGRPCClient(w)
	if client == nil {
		return
	}

	resp, err := client.TriggerSimulationLiveStep(r.Context(), &pb.Empty{})
	if err != nil {
		logger.Logger.Errorf("TriggerSimulationLiveStep: %v", err)
		ResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ResponseSuccess(w, http.StatusOK, map[string]string{"message": resp.Message})
}

// TriggerSimResetHandler godoc
//
//	@Summary      Reset all active simulation bots
//	@Description  Closes stale live sessions and creates fresh running sessions for all active bots. Use when bots stop trading after the first day.
//	@Tags         Triggers
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200  {object}  map[string]string
//	@Failure      401  {object}  ResponseFailure
//	@Failure      500  {object}  ResponseFailure
//	@Failure      503  {object}  ResponseFailure
//	@Router       /api/trigger/sim-reset [post]
func TriggerSimResetHandler(w http.ResponseWriter, r *http.Request) {
	client := requireGRPCClient(w)
	if client == nil {
		return
	}

	resp, err := client.ResetSimBots(r.Context(), &pb.Empty{})
	if err != nil {
		logger.Logger.Errorf("TriggerSimReset: %v", err)
		ResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ResponseSuccess(w, http.StatusOK, map[string]string{"message": resp.Message})
}
