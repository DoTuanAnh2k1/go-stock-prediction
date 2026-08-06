package server

import (
	"go-stock-prediction/pkg/logger"
	modelsapi "go-stock-prediction/pkg/models/models_api"
	pb "go-stock-prediction/proto/prediction"
	"net/http"
	"time"
)

// GetTrainingStatus godoc
//
//	@Summary      Get current training status
//	@Description  Returns whether training is in progress, the last trained time, next scheduled training, current phase, and progress percentage. Calls the prediction gRPC service internally.
//	@Tags         Training
//	@Produce      json
//	@Success      200  {object}  modelsapi.TrainingStatusDTO
//	@Failure      500  {object}  ResponseFailure
//	@Router       /api/training/status [get]
func GetTrainingStatus(w http.ResponseWriter, r *http.Request) {
	logger.Ctx(r.Context()).Info("Getting training status...")

	client := requireGRPCClient(w)
	if client == nil {
		return
	}
	grpcResp, err := client.GetTrainingStatus(r.Context(), &pb.Empty{})
	if err != nil {
		logger.Ctx(r.Context()).Errorf("GetTrainingStatus: gRPC call failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Parse last_trained RFC3339 string; zero value if empty (never trained)
	var lastTrained time.Time
	if ts := grpcResp.GetLastTrained(); ts != "" {
		if t, parseErr := time.Parse(time.RFC3339, ts); parseErr == nil {
			lastTrained = t
		}
	}

	// Calculate next training (next Sunday 9 AM)
	now := time.Now()
	daysUntilSunday := (7 - int(now.Weekday())) % 7
	if daysUntilSunday == 0 && now.Hour() >= 9 {
		daysUntilSunday = 7
	}
	nextTraining := now.AddDate(0, 0, daysUntilSunday)
	nextTraining = time.Date(nextTraining.Year(), nextTraining.Month(), nextTraining.Day(), 9, 0, 0, 0, nextTraining.Location())

	status := &modelsapi.TrainingStatusDTO{
		IsTraining:   grpcResp.GetIsTraining(),
		LastTrained:  lastTrained,
		NextTraining: nextTraining,
		CurrentPhase: grpcResp.GetCurrentPhase(),
		Progress:     grpcResp.GetProgress(),
	}

	if grpcResp.GetIsTraining() {
		status.EstimatedTime = "~20 minutes"
	}

	ResponseSuccess(w, http.StatusOK, status)
}
