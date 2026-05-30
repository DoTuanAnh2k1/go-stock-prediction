package modelsapi

type PredictionListDTO struct {
	Predictions []PredictionDetailDTO `json:"predictions"`
	Total       int                   `json:"total"`
	TotalCount  int64                 `json:"total_count"`
	Page        int                   `json:"page"`
	PageSize    int                   `json:"page_size"`
	TotalPages  int                   `json:"total_pages"`
	Filters     PredictionFilterDTO   `json:"filters"`
}
