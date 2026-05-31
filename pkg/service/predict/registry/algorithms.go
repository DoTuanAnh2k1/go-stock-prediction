// algorithms.go — edit this file to add/remove prediction algorithms.
//
// Steps to add a new algorithm:
//  1. Create pkg/service/predict/<algo_name>/ and implement iface.PredictionAlgorithm
//     (methods: Predict, GetName, GetAccuracy)
//  2. Add a Register() call below in the init() function
//  3. If the algorithm should be included in Ensemble, add it to the CompositeFactory slice
//  4. That's it — the API and prediction service pick it up automatically
package registry

import (
	arimagarch "go-stock-prediction/pkg/service/predict/arima_garch"
	"go-stock-prediction/pkg/service/predict/ensemble"
	ema "go-stock-prediction/pkg/service/predict/ema"
	"go-stock-prediction/pkg/service/predict/iface"
	lstmnn "go-stock-prediction/pkg/service/predict/lstm_nn"
	movingaverage "go-stock-prediction/pkg/service/predict/moving_average"
)

func init() {
	// ── Base algorithms ───────────────────────────────────────────────
	// To add a new base algorithm: copy one of these blocks.

	Register(AlgorithmDef{
		Key:         "moving_average",
		DisplayName: "Moving Average (VWMA)",
		Config:      map[string]interface{}{"window": 20},
		Factory:     func() iface.PredictionAlgorithm { return movingaverage.NewMovingAveragePredictor() },
	})
	Register(AlgorithmDef{
		Key:         "lstm_nn",
		DisplayName: "LSTM Neural Network",
		Config:      map[string]interface{}{"epochs": 100, "learning_rate": 0.001, "hidden_layers": 2, "batch_size": 32},
		Factory:     func() iface.PredictionAlgorithm { return lstmnn.NewLSTMPredictor() },
	})
	Register(AlgorithmDef{
		Key:         "arima_garch",
		DisplayName: "ARIMA-GARCH",
		Config:      map[string]interface{}{"p": 5, "d": 1, "q": 2},
		Factory:     func() iface.PredictionAlgorithm { return arimagarch.NewARIMAGARCHPredictor() },
	})
	Register(AlgorithmDef{
		Key:         "ema",
		DisplayName: "Exponential Moving Average",
		Config:      map[string]interface{}{"short_period": 12, "long_period": 26, "signal_period": 9},
		Factory:     func() iface.PredictionAlgorithm { return ema.NewEMAPredictor() },
	})

	// ── Composite algorithms (receive base instances) ─────────────────
	// Ensemble combines the 4 base algorithms above.
	// To add a new algorithm to Ensemble, add bases["<key>"] to the slice below.
	Register(AlgorithmDef{
		Key:         "ensemble",
		DisplayName: "Ensemble",
		Config:      map[string]interface{}{},
		IsComposite: true,
		CompositeFactory: func(bases map[string]iface.PredictionAlgorithm) iface.PredictionAlgorithm {
			return ensemble.New([]iface.PredictionAlgorithm{
				bases["moving_average"],
				bases["lstm_nn"],
				bases["arima_garch"],
				bases["ema"],
			})
		},
	})
}
