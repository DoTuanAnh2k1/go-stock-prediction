package mysql

import (
	"time"

	modelsdb "go-stock-prediction/pkg/models/models_db"
)

// GetPipelineReports returns pipeline reports ordered by created_at DESC.
// If pipelineKey is non-empty only rows matching that key are included.
func (c *Client) GetPipelineReports(pipelineKey string, limit int) ([]modelsdb.PipelineReport, error) {
	var reports []modelsdb.PipelineReport
	q := c.Db.Model(&modelsdb.PipelineReport{}).Order("created_at DESC").Limit(limit)
	if pipelineKey != "" {
		q = q.Where("pipeline_key = ?", pipelineKey)
	}
	err := q.Find(&reports).Error
	return reports, err
}

// GetDistinctPipelineKeys returns all distinct pipeline_key values sorted alphabetically.
func (c *Client) GetDistinctPipelineKeys() ([]string, error) {
	var keys []string
	err := c.Db.Model(&modelsdb.PipelineReport{}).
		Distinct("pipeline_key").
		Order("pipeline_key").
		Pluck("pipeline_key", &keys).Error
	return keys, err
}

// DeletePipelineReportsBefore removes all rows whose created_at is before t.
// Returns the number of rows deleted.
func (c *Client) DeletePipelineReportsBefore(t time.Time) (int64, error) {
	result := c.Db.Where("created_at < ?", t).Delete(&modelsdb.PipelineReport{})
	return result.RowsAffected, result.Error
}
