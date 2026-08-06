package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/store/repository"
)

// pipelineReportItem is the JSON shape for a single pipeline report row.
type pipelineReportItem struct {
	ID               int64           `json:"id"`
	PipelineKey      string          `json:"pipeline_key"`
	Market           string          `json:"market"`
	Status           string          `json:"status"`
	StartedAt        string          `json:"started_at"`
	FinishedAt       string          `json:"finished_at"`
	DurationMs       int64           `json:"duration_ms"`
	CrawledCount     int             `json:"crawled_count"`
	PredictionsCount int             `json:"predictions_count"`
	Trained          bool            `json:"trained"`
	Steps            json.RawMessage `json:"steps"`
	Error            string          `json:"error"`
}

// pipelineReportsResponse is the top-level JSON response for GET /api/pipeline-reports.
type pipelineReportsResponse struct {
	Data      []pipelineReportItem `json:"data"`
	Pipelines []string             `json:"pipelines"`
}

const (
	pipelineReportsDefaultLimit = 50
	pipelineReportsMaxLimit     = 200
)

// GetPipelineReports godoc
//
//	@Summary      Get pipeline reports
//	@Description  Returns a list of pipeline execution reports, optionally filtered by pipeline key. Each report includes per-step detail, status, counts, and duration. Requires JWT authentication.
//	@Tags         Settings
//	@Produce      json
//	@Security     BearerAuth
//	@Param        pipeline  query     string  false  "Filter by pipeline key (e.g. crawler_gold)"
//	@Param        limit     query     int     false  "Max rows to return (default 50, max 200)"
//	@Success      200  {object}  pipelineReportsResponse
//	@Failure      401  {object}  ResponseFailure
//	@Failure      500  {object}  ResponseFailure
//	@Router       /api/pipeline-reports [get]
func GetPipelineReports(w http.ResponseWriter, r *http.Request) {
	pipelineKey := r.URL.Query().Get("pipeline")

	limitStr := r.URL.Query().Get("limit")
	limit := pipelineReportsDefaultLimit
	if limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil || parsed < 1 {
			ResponseError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		if parsed > pipelineReportsMaxLimit {
			parsed = pipelineReportsMaxLimit
		}
		limit = parsed
	}

	store := repository.GetSingleton()

	reports, err := store.GetPipelineReports(r.Context(), pipelineKey, limit)
	if err != nil {
		logger.Ctx(r.Context()).Errorf("[api/pipeline-reports] GetPipelineReports: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get pipeline reports")
		return
	}

	keys, err := store.GetDistinctPipelineKeys(r.Context())
	if err != nil {
		logger.Ctx(r.Context()).Errorf("[api/pipeline-reports] GetDistinctPipelineKeys: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get pipeline keys")
		return
	}

	const tsLayout = "2006-01-02T15:04:05"

	items := make([]pipelineReportItem, 0, len(reports))
	for _, rep := range reports {
		finishedAt := ""
		if rep.FinishedAt != nil {
			finishedAt = rep.FinishedAt.Format(tsLayout)
		}

		errStr := ""
		if rep.Error != nil {
			errStr = *rep.Error
		}

		steps := rep.Steps
		if len(steps) == 0 {
			steps = json.RawMessage("[]")
		}

		items = append(items, pipelineReportItem{
			ID:               rep.ID,
			PipelineKey:      rep.PipelineKey,
			Market:           rep.Market,
			Status:           rep.Status,
			StartedAt:        rep.StartedAt.Format(tsLayout),
			FinishedAt:       finishedAt,
			DurationMs:       rep.DurationMs,
			CrawledCount:     rep.CrawledCount,
			PredictionsCount: rep.PredictionsCount,
			Trained:          rep.Trained,
			Steps:            steps,
			Error:            errStr,
		})
	}

	if keys == nil {
		keys = []string{}
	}

	ResponseSuccess(w, http.StatusOK, pipelineReportsResponse{
		Data:      items,
		Pipelines: keys,
	})
}
