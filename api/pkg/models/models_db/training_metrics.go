package modelsdb

// TrainingMetricsAggregate holds pre-computed aggregate training statistics
// returned by the repository layer and consumed by the API handlers.
// Defined here so both pkg/store/mysql and pkg/store/repository can reference
// it without creating an import cycle.
type TrainingMetricsAggregate struct {
	TotalSessions    int64
	TotalPredictions int64
	AvgDurationMs    float64
	// SuccessRate = total_success_count / (total_success_count + total_error_count)
	SuccessRate float64
	// DataQuality = avg(success_count / total_stocks) across all logs
	DataQuality float64
}
