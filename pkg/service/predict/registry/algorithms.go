// algorithms.go — algorithm metadata registry.
//
// Prediction is now handled by the Python service (services/prediction/).
// This file registers algorithm metadata used by /api/training/algorithms.
// To add/remove algorithms: edit the Register() calls in init() below.
package registry

func init() {
	Register(AlgorithmDef{
		Key:         "moving_average",
		DisplayName: "Moving Average (VWMA)",
		Config:      map[string]interface{}{"window": 20},
	})
	Register(AlgorithmDef{
		Key:         "ema",
		DisplayName: "Exponential Moving Average",
		Config:      map[string]interface{}{"short_period": 12, "long_period": 26, "signal_period": 9},
	})
	Register(AlgorithmDef{
		Key:         "lstm_nn",
		DisplayName: "LSTM Neural Network",
		Config:      map[string]interface{}{"epochs": 100, "learning_rate": 0.001, "hidden_layers": 2, "batch_size": 32},
	})
	Register(AlgorithmDef{
		Key:         "arima_garch",
		DisplayName: "ARIMA-GARCH",
		Config:      map[string]interface{}{"p": 2, "d": 1, "q": 2},
	})
	Register(AlgorithmDef{
		Key:         "lightgbm",
		DisplayName: "LightGBM",
		Config:      map[string]interface{}{"n_estimators": 200, "learning_rate": 0.05, "num_leaves": 31},
	})
	Register(AlgorithmDef{
		Key:         "ensemble",
		DisplayName: "Ensemble",
		Config:      map[string]interface{}{},
		IsComposite: true,
	})
}
