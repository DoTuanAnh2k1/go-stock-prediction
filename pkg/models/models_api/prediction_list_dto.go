package modelsapi

type PredictionListDTO struct {
	Predictions []PredictionDetailDTO `json:"predictions"`
	Total       int                   `json:"total"`
	Filters     PredictionFilterDTO   `json:"filters"`
}
