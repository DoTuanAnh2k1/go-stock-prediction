package modelsdb

import (
	"encoding/json"
	"time"
)

// PipelineReport stores per-run metadata for a prediction pipeline execution.
// status values: 'success' | 'partial' | 'failed' | 'skipped'.
// steps is a JSONB array of step objects written by the Python service.
type PipelineReport struct {
	ID               int64           `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	PipelineKey      string          `gorm:"column:pipeline_key;size:50;not null" json:"pipeline_key"`
	Market           string          `gorm:"column:market;size:20;not null" json:"market"`
	Status           string          `gorm:"column:status;size:20;not null" json:"status"`
	StartedAt        time.Time       `gorm:"column:started_at;not null" json:"started_at"`
	FinishedAt       *time.Time      `gorm:"column:finished_at" json:"finished_at"`
	DurationMs       int64           `gorm:"column:duration_ms;not null;default:0" json:"duration_ms"`
	CrawledCount     int             `gorm:"column:crawled_count;not null;default:0" json:"crawled_count"`
	PredictionsCount int             `gorm:"column:predictions_count;not null;default:0" json:"predictions_count"`
	Trained          bool            `gorm:"column:trained;not null;default:false" json:"trained"`
	Steps            json.RawMessage `gorm:"column:steps;type:jsonb" json:"steps"`
	Error            *string         `gorm:"column:error" json:"error"`
	CreatedAt        time.Time       `gorm:"column:created_at;not null;default:now()" json:"created_at"`
}

func (PipelineReport) TableName() string {
	return "pipeline_reports"
}
