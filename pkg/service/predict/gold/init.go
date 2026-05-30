package goldpredict

import (
	"context"
	"fmt"
	"go-stock-prediction/pkg/logger"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	modelssvc "go-stock-prediction/pkg/models/models_svc"
	arimagarch "go-stock-prediction/pkg/service/predict/arima_garch"
	"go-stock-prediction/pkg/service/predict/ensemble"
	"go-stock-prediction/pkg/service/predict/iface"
	lstmnn "go-stock-prediction/pkg/service/predict/lstm_nn"
	movingaverage "go-stock-prediction/pkg/service/predict/moving_average"
	"go-stock-prediction/pkg/store/repository"
	"go-stock-prediction/pkg/utils/cron"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

// goldSources defines the (source, productType) pairs to predict.
// Adjust to match what your crawler actually stores.
var goldSources = []struct {
	Source      string
	ProductType string
}{
	{"XAU", "spot"},
	{"BTMC", "sjc"},
	{"BTMC", "nhan_tron"},
}

var (
	once       sync.Once
	algorithms map[string]iface.PredictionAlgorithm
	store      repository.DatabaseStore
)

// Init initialises the gold prediction service and registers a daily cron job.
func Init() {
	once.Do(func() {
		logger.Logger.Info("🥇 Initializing gold prediction service...")

		store = repository.GetSingleton()
		if store == nil {
			logger.Logger.Fatal("Failed to get database store for gold prediction service")
			return
		}

		registerAlgorithms()

		err := cron.AddJob("Daily Gold Prediction", cron.Daily6PM, CronjobDailyGoldPrediction)
		if err != nil {
			logger.Logger.Errorf("Failed to add daily gold prediction cronjob: %v", err)
		} else {
			logger.Logger.Info("✅ Daily gold prediction cronjob registered - every day 6 PM")
		}

		logger.Logger.Info("🎯 Gold prediction service initialized successfully!")
	})
}

func registerAlgorithms() {
	ma := movingaverage.NewMovingAveragePredictor()
	lstm := lstmnn.NewLSTMPredictor()
	arima := arimagarch.NewARIMAGARCHPredictor()
	ens := ensemble.New([]iface.PredictionAlgorithm{ma, lstm, arima})

	algorithms = map[string]iface.PredictionAlgorithm{
		"moving_average": ma,
		"lstm_nn":        lstm,
		"arima_garch":    arima,
		"ensemble":       ens,
	}
}

// CronjobDailyGoldPrediction generates predictions for all tracked gold sources.
func CronjobDailyGoldPrediction() error {
	logger.Logger.Info("🔮 Starting daily gold prediction cronjob...")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	success, errors := 0, 0
	for _, src := range goldSources {
		data, err := getGoldData(src.Source, src.ProductType, 9)
		if err != nil {
			logger.Logger.Warnf("⚠️ Skipping %s/%s — insufficient data: %v", src.Source, src.ProductType, err)
			continue
		}

		for algName, alg := range algorithms {
			pred, err := runPrediction(ctx, src.Source, src.ProductType, algName, alg, data)
			if err != nil {
				logger.Logger.Errorf("❌ Gold predict %s/%s (%s): %v", src.Source, src.ProductType, algName, err)
				errors++
				continue
			}
			if err := store.CreateGoldPrediction(pred); err != nil {
				logger.Logger.Errorf("❌ Save gold prediction %s/%s (%s): %v", src.Source, src.ProductType, algName, err)
				errors++
				continue
			}
			success++
		}
	}

	logger.Logger.Infof("🎯 Gold prediction done: %d saved, %d errors", success, errors)
	return nil
}

// RunNow triggers gold prediction immediately (for manual /api/trigger/gold-predict).
func RunNow() (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	if store == nil {
		store = repository.GetSingleton()
	}
	if algorithms == nil {
		registerAlgorithms()
	}

	success := 0
	for _, src := range goldSources {
		data, err := getGoldData(src.Source, src.ProductType, 9)
		if err != nil {
			logger.Logger.Warnf("⚠️ Skipping %s/%s: %v", src.Source, src.ProductType, err)
			continue
		}
		for algName, alg := range algorithms {
			pred, err := runPrediction(ctx, src.Source, src.ProductType, algName, alg, data)
			if err != nil {
				continue
			}
			if err := store.CreateGoldPrediction(pred); err != nil {
				continue
			}
			success++
		}
	}
	return success, nil
}

// getGoldData fetches the last `months` months of buy prices for a given source/productType.
func getGoldData(source, productType string, months int) (*modelssvc.StockData, error) {
	from := time.Now().AddDate(0, -months, 0)
	to := time.Now()

	prices, err := store.GetGoldPricesByDateRange(source, productType, from, to)
	if err != nil {
		return nil, err
	}
	if len(prices) < 20 {
		return nil, fmt.Errorf("only %d data points (need ≥ 20)", len(prices))
	}

	// prices are ordered DESC — reverse to get chronological order for algorithms
	historical := make([]string, len(prices))
	for i, p := range prices {
		historical[len(prices)-1-i] = p.BuyPrice.String()
	}

	return &modelssvc.StockData{Historical: historical}, nil
}

// runPrediction calls an algorithm and wraps the result in a GoldPrediction record.
func runPrediction(ctx context.Context, source, productType, algName string, alg iface.PredictionAlgorithm, data *modelssvc.StockData) (*modelsdb.GoldPrediction, error) {
	result, err := alg.Predict(ctx, data)
	if err != nil {
		return nil, err
	}

	confidence := result.Confidence
	if confidence == 0 {
		confidence = alg.GetAccuracy()
	}

	return &modelsdb.GoldPrediction{
		Source:         source,
		ProductType:    productType,
		PredictedPrice: decimal.NewFromFloat(result.PredictedPrice),
		CurrentPrice:   decimal.NewFromFloat(result.CurrentPrice),
		Confidence:     decimal.NewFromFloat(confidence),
		AlgorithmName:  algName,
		PredictionDate: time.Now(),
		TargetDate:     time.Now().AddDate(0, 0, 1),
	}, nil
}
