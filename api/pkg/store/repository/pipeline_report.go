package repository

import (
	"time"

	modelsdb "go-stock-prediction/pkg/models/models_db"
)

// PipelineReportStore — interface for pipeline_reports DB operations.
type PipelineReportStore interface {
	// GetPipelineReports returns reports ordered by created_at DESC.
	// If pipelineKey is non-empty only rows matching that key are returned.
	// limit caps the number of rows returned.
	GetPipelineReports(pipelineKey string, limit int) ([]modelsdb.PipelineReport, error)

	// GetDistinctPipelineKeys returns all distinct pipeline_key values alphabetically.
	GetDistinctPipelineKeys() ([]string, error)

	// DeletePipelineReportsBefore removes all rows whose created_at is before t.
	// Returns the number of rows deleted.
	DeletePipelineReportsBefore(t time.Time) (int64, error)
}
