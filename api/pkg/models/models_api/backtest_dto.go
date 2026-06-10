package modelsapi

type BacktestResultDTO struct {
	Algorithm           string  `json:"algorithm"`
	TotalPredictions    int     `json:"total_predictions"`
	MAE                 float64 `json:"mae"`
	RMSE                float64 `json:"rmse"`
	MAPE                float64 `json:"mape"`
	DirectionalAccuracy float64 `json:"directional_accuracy"`
}
