package modelsapi

// TrainingMetricsDTO is the JSON response for GET /api/training/metrics.
type TrainingMetricsDTO struct {
	AvgTrainingTime   string  `json:"avg_training_time"`
	AvgTrainingTimeMs float64 `json:"avg_training_time_ms"`
	DataQuality       float64 `json:"data_quality"`
	MemoryUsageMB     uint64  `json:"memory_usage_mb"`
	SuccessRate       float64 `json:"success_rate"`
	TotalSessions     int64   `json:"total_sessions"`
	TotalPredictions  int64   `json:"total_predictions"`
}

// TrainingAlgorithmDTO is one entry in GET /api/training/algorithms.
type TrainingAlgorithmDTO struct {
	Name                string                 `json:"name"`
	Key                 string                 `json:"key"`
	Status              string                 `json:"status"`
	LastTrained         *string                `json:"last_trained,omitempty"`
	Accuracy            float64                `json:"accuracy"`
	Config              map[string]interface{} `json:"config"`
	TrainingTimeSeconds float64                `json:"training_time_seconds"`
	TotalPredictions    int64                  `json:"total_predictions"`
	SuccessRate         float64                `json:"success_rate"`
}
