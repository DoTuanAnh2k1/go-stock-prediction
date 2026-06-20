package modelssvc

type Prediction struct {
	PredictedPrice float64 `json:"predicted_price"`
	CurrentPrice   float64 `json:"current_price"` // <- Thêm cái này vào!
	Confidence     float64 `json:"confidence"`
	Symbol         string  `json:"symbol"` // Thêm luôn cho dễ debug
}
