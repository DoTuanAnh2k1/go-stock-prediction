package modelsapi

import "github.com/shopspring/decimal"

type PredictionAccuracyDTO struct {
	AlgorithmName       string          `json:"algorithm_name"`
	Period              string          `json:"period"`
	TotalPredictions    int             `json:"total_predictions"`
	AccuratePredictions int             `json:"accurate_predictions"`
	AccuracyRate        decimal.Decimal `json:"accuracy_rate"`
	AvgError            decimal.Decimal `json:"avg_error"`
}

// AccuracyTrendPointDTO - một điểm dữ liệu trong trend accuracy theo ngày
// Date là YYYY-MM-DD; các trường thuật toán là float64 (% accuracy, 0 nếu không có dữ liệu).
type AccuracyTrendPointDTO struct {
	Date          string  `json:"date"`
	LstmNN        float64 `json:"lstm_nn"`
	ArimaGarch    float64 `json:"arima_garch"`
	MovingAverage float64 `json:"moving_average"`
}
