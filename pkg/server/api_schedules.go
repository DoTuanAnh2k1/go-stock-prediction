package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/robfig/cron/v3"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"go-stock-prediction/pkg/store/repository"
)

type scheduleResponse struct {
	JobKey         string    `json:"job_key"`
	JobName        string    `json:"job_name"`
	CronExpression string    `json:"cron_expression"`
	Enabled        bool      `json:"enabled"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type updateScheduleRequest struct {
	CronExpression string `json:"cron_expression"`
	Enabled        bool   `json:"enabled"`
}

// GetSchedulesHandler handles GET /api/schedules — requires auth.
func GetSchedulesHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}
	store := repository.GetSingleton()
	schedules, err := store.GetAllCronSchedules()
	if err != nil {
		ResponseError(w, http.StatusInternalServerError, "failed to fetch schedules")
		return
	}
	resp := make([]scheduleResponse, len(schedules))
	for i, s := range schedules {
		resp[i] = scheduleResponse{
			JobKey:         s.JobKey,
			JobName:        s.JobName,
			CronExpression: s.CronExpression,
			Enabled:        s.Enabled,
			UpdatedAt:      s.UpdatedAt,
		}
	}
	ResponseSuccess(w, http.StatusOK, resp)
}

// UpdateScheduleHandler handles PUT /api/schedules/{key} — requires auth.
func UpdateScheduleHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}

	key := r.PathValue("key")
	if key == "" {
		ResponseError(w, http.StatusBadRequest, "job key is required")
		return
	}

	var req updateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ResponseError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.CronExpression == "" {
		ResponseError(w, http.StatusBadRequest, "cron_expression is required")
		return
	}

	// Validate cron expression using robfig/cron parser (with seconds support).
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.DowOptional)
	if _, err := parser.Parse(req.CronExpression); err != nil {
		ResponseError(w, http.StatusBadRequest, "invalid cron expression: "+err.Error())
		return
	}

	store := repository.GetSingleton()
	existing, err := store.GetCronScheduleByKey(key)
	if err != nil {
		ResponseError(w, http.StatusNotFound, "schedule not found")
		return
	}

	// No-op if nothing changed.
	if existing.CronExpression == req.CronExpression && existing.Enabled == req.Enabled {
		ResponseSuccess(w, http.StatusOK, map[string]string{"message": "no changes"})
		return
	}

	existing.CronExpression = req.CronExpression
	existing.Enabled = req.Enabled

	if err := store.UpsertCronSchedule(existing); err != nil {
		ResponseError(w, http.StatusInternalServerError, "failed to update schedule")
		return
	}

	ResponseSuccess(w, http.StatusOK, map[string]string{"message": "schedule updated"})
}

// seedCronSchedules inserts default schedule rows for well-known jobs if they do not already exist.
// Called at startup so the table is never empty on first run.
func seedCronSchedules(store repository.DatabaseStore) {
	defaults := []modelsdb.CronSchedule{
		{JobKey: "crawler_daily", JobName: "Daily Stock Crawler", CronExpression: "0 0 12 * * *", Enabled: true},
		{JobKey: "predict_daily", JobName: "Daily Prediction", CronExpression: "0 0 18 * * *", Enabled: true},
		{JobKey: "train_weekly", JobName: "Weekly Model Training", CronExpression: "0 0 9 * * SUN", Enabled: true},
		{JobKey: "reconcile_daily", JobName: "Daily Reconcile", CronExpression: "0 0 6 * * *", Enabled: true},
		{JobKey: "gold_crawler_daily", JobName: "Daily Gold Crawler", CronExpression: "0 0 10 * * *", Enabled: true},
	}
	for i := range defaults {
		existing, err := store.GetCronScheduleByKey(defaults[i].JobKey)
		if err != nil || existing == nil {
			// Row does not exist — insert.
			_ = store.UpsertCronSchedule(&defaults[i])
		}
	}
}
