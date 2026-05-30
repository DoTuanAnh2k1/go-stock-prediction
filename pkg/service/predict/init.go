package predict

import (
	"context"
	"crypto/rand"
	"fmt"
	"go-stock-prediction/pkg/logger"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	modelssvc "go-stock-prediction/pkg/models/models_svc"
	arimagarch "go-stock-prediction/pkg/service/predict/arima_garch"
	"go-stock-prediction/pkg/service/predict/ensemble"
	lstmnn "go-stock-prediction/pkg/service/predict/lstm_nn"
	movingaverage "go-stock-prediction/pkg/service/predict/moving_average"
	"go-stock-prediction/pkg/store/repository"
	"go-stock-prediction/pkg/utils/cron"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

// Global prediction service instance
var (
	predictor *PredictionService
	once      sync.Once
)

// PredictionService manages all prediction algorithms and training
type PredictionService struct {
	algorithms    map[string]PredictionAlgorithm
	store         repository.DatabaseStore
	isTraining    bool
	lastTrained   time.Time
	mutex         sync.RWMutex
	// Training progress fields (protected by mutex)
	trainingProgress      float64   // 0–100
	trainingPhase         string    // human-readable current phase
	trainingStarted       time.Time // when current training began
	trainingTotalAlgos    int       // total algorithms to train
	trainingDoneAlgos     int       // algorithms completed so far
}

// TrainingResult contains results from training process
type TrainingResult struct {
	AlgorithmName string          `json:"algorithm_name"`
	TotalStocks   int             `json:"total_stocks"`
	SuccessCount  int             `json:"success_count"`
	ErrorCount    int             `json:"error_count"`
	Accuracy      decimal.Decimal `json:"accuracy"`
	Duration      time.Duration   `json:"duration"`
	TrainedAt     time.Time       `json:"trained_at"`
}

// Init initializes the prediction service and sets up cronjobs
func Init() {
	once.Do(func() {
		logger.Logger.Info("🧠 Initializing prediction service...")

		// Get database store
		store := repository.GetSingleton()
		if store == nil {
			logger.Logger.Fatal("Failed to get database store for prediction service")
			return
		}

		// Initialize prediction service
		predictor = &PredictionService{
			algorithms: make(map[string]PredictionAlgorithm),
			store:      store,
			isTraining: false,
		}

		// Register all prediction algorithms
		registerAlgorithms()

		// Setup weekly training cronjob - every Sunday at 2 AM (chill time!)
		err := cron.AddJob("Weekly Model Training", cron.WeeklySundayAM, CronjobWeeklyTraining)
		if err != nil {
			logger.Logger.Errorf("Failed to add weekly training cronjob: %v", err)
		} else {
			logger.Logger.Info("✅ Weekly training cronjob registered - every Sunday 9 AM")
		}

		// Setup daily prediction job - every day at 6 PM after market close
		err = cron.AddJob("Daily Stock Prediction", cron.Daily6PM, CronjobDailyPrediction)
		if err != nil {
			logger.Logger.Errorf("Failed to add daily prediction cronjob: %v", err)
		} else {
			logger.Logger.Info("✅ Daily prediction cronjob registered - every day 6 PM")
		}

		// Setup daily reconciliation job - every day at 6 AM to update actual prices
		err = cron.AddJob("Daily Prediction Reconcile", cron.Daily6AM, CronjobDailyReconcile)
		if err != nil {
			logger.Logger.Errorf("Failed to add daily reconcile cronjob: %v", err)
		} else {
			logger.Logger.Info("✅ Daily reconcile cronjob registered - every day 6 AM")
		}

		logger.Logger.Info("🎯 Prediction service initialized successfully!")
	})
}

// registerAlgorithms registers all available prediction algorithms
func registerAlgorithms() {
	logger.Logger.Info("📚 Registering prediction algorithms...")

	// Register Moving Average predictor
	maPredictor := movingaverage.NewMovingAveragePredictor()
	predictor.algorithms["moving_average"] = maPredictor
	logger.Logger.Infof("✅ Registered: %s", maPredictor.GetName())

	// Register LSTM Neural Network predictor
	lstmPredictor := lstmnn.NewLSTMPredictor()
	predictor.algorithms["lstm_nn"] = lstmPredictor
	logger.Logger.Infof("✅ Registered: %s", lstmPredictor.GetName())

	// Register ARIMA-GARCH predictor
	arimaPredictor := arimagarch.NewARIMAGARCHPredictor()
	predictor.algorithms["arima_garch"] = arimaPredictor
	logger.Logger.Infof("✅ Registered: %s", arimaPredictor.GetName())

	// Register Ensemble predictor (stacks the 3 base algorithms)
	ensemblePredictor := ensemble.New([]PredictionAlgorithm{maPredictor, lstmPredictor, arimaPredictor})
	predictor.algorithms["ensemble"] = ensemblePredictor
	logger.Logger.Infof("✅ Registered: %s", ensemblePredictor.GetName())

	logger.Logger.Infof("🚀 Total %d algorithms registered", len(predictor.algorithms))
}

// CronjobWeeklyTraining runs weekly model training
func CronjobWeeklyTraining() error {
	logger.Logger.Info("🏋️ Starting weekly model training cronjob...")

	if predictor == nil {
		return fmt.Errorf("prediction service not initialized")
	}

	// Check if already training
	predictor.mutex.Lock()
	if predictor.isTraining {
		predictor.mutex.Unlock()
		logger.Logger.Warn("⚠️ Training already in progress, skipping...")
		return nil
	}
	predictor.isTraining = true
	predictor.trainingProgress = 0
	predictor.trainingPhase = "Initializing"
	predictor.trainingStarted = time.Now()
	predictor.trainingDoneAlgos = 0
	predictor.trainingTotalAlgos = len(predictor.algorithms)
	predictor.mutex.Unlock()

	defer func() {
		predictor.mutex.Lock()
		predictor.isTraining = false
		predictor.lastTrained = time.Now()
		predictor.trainingProgress = 100
		predictor.trainingPhase = "idle"
		predictor.mutex.Unlock()
	}()

	startTime := time.Now()
	// ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour) // 2 hours max
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute) // 15 minutes max
	defer cancel()

	// Get all VN30 stocks for training
	stocks, err := predictor.store.GetVN30Stocks()
	if err != nil {
		logger.Logger.Errorf("❌ Failed to get VN30 stocks: %v", err)
		return err
	}

	logger.Logger.Infof("📊 Training models on %d VN30 stocks", len(stocks))

	sessionID := generateSessionID()

	// Train each algorithm
	var trainingResults []TrainingResult
	algIdx := 0
	for algName, algorithm := range predictor.algorithms {
		predictor.mutex.Lock()
		predictor.trainingPhase = fmt.Sprintf("Training %s (%d/%d)", algName, algIdx+1, len(predictor.algorithms))
		predictor.trainingProgress = float64(algIdx) / float64(len(predictor.algorithms)) * 100
		predictor.trainingDoneAlgos = algIdx
		predictor.mutex.Unlock()

		result := trainSingleAlgorithm(ctx, algName, algorithm, stocks)
		trainingResults = append(trainingResults, result)

		algIdx++
		predictor.mutex.Lock()
		predictor.trainingDoneAlgos = algIdx
		predictor.trainingProgress = float64(algIdx) / float64(len(predictor.algorithms)) * 100
		predictor.mutex.Unlock()

		logger.Logger.Infof("✅ %s training completed: %d/%d successful, accuracy: %v",
			algName, result.SuccessCount, result.TotalStocks, result.Accuracy)

		trainingLog := &modelsdb.TrainingLog{
			SessionID:     sessionID,
			AlgorithmName: algName,
			TotalStocks:   result.TotalStocks,
			SuccessCount:  result.SuccessCount,
			ErrorCount:    result.ErrorCount,
			Accuracy:      result.Accuracy,
			DurationMs:    result.Duration.Milliseconds(),
			StartedAt:     result.TrainedAt,
			CompletedAt:   result.TrainedAt.Add(result.Duration),
		}
		if err := predictor.store.CreateTrainingLog(trainingLog); err != nil {
			logger.Logger.Errorf("Failed to save training log for %s: %v", algName, err)
		}
	}

	// Log overall training results
	duration := time.Since(startTime)
	err = logTrainingResults(trainingResults, duration)
	if err != nil {
		logger.Logger.Errorf("❌ Failed to log training results: %v", err)
	}

	logger.Logger.Infof("🎉 Weekly training completed in %v", duration)
	return nil
}

// CronjobDailyPrediction runs daily stock predictions
func CronjobDailyPrediction() error {
	logger.Logger.Info("🔮 Starting daily prediction cronjob...")

	if predictor == nil {
		return fmt.Errorf("prediction service not initialized")
	}

	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Get all VN30 stocks
	stocks, err := predictor.store.GetVN30Stocks()
	if err != nil {
		logger.Logger.Errorf("❌ Failed to get VN30 stocks: %v", err)
		return err
	}

	totalPredictions := 0
	successCount := 0
	errorCount := 0

	// Generate predictions for each stock using all algorithms
	for _, stock := range stocks {
		for algName, algorithm := range predictor.algorithms {
			prediction, err := generateStockPrediction(ctx, stock, algName, algorithm)
			if err != nil {
				logger.Logger.Errorf("❌ Failed to predict %s with %s: %v", stock.Symbol, algName, err)
				errorCount++
				continue
			}

			// Save prediction to database
			err = predictor.store.CreatePrediction(prediction)
			if err != nil {
				logger.Logger.Errorf("❌ Failed to save prediction for %s: %v", stock.Symbol, err)
				errorCount++
				continue
			}

			successCount++
			totalPredictions++
			logger.Logger.Debugf("✅ Saved prediction for %s (%s): %v",
				stock.Symbol, algName, prediction.PredictedPrice)
		}
	}

	duration := time.Since(startTime)
	logger.Logger.Infof("🎯 Daily prediction completed: %d predictions generated, %d successful, %d errors in %v",
		totalPredictions, successCount, errorCount, duration)

	return nil
}

// trainSingleAlgorithm trains a single algorithm on all stocks
func trainSingleAlgorithm(ctx context.Context, algName string, algorithm PredictionAlgorithm, stocks []modelsdb.Stock) TrainingResult {
	startTime := time.Now()
	result := TrainingResult{
		AlgorithmName: algName,
		TotalStocks:   len(stocks),
		TrainedAt:     startTime,
	}

	logger.Logger.Infof("🎯 Training %s on %d stocks...", algName, len(stocks))

	var totalAccuracy float64
	validStocks := 0

	for _, stock := range stocks {
		// Get historical data for training
		stockData, err := getStockTrainingData(stock.ID)
		if err != nil {
			logger.Logger.Debugf("⚠️ Failed to get training data for %s: %v", stock.Symbol, err)
			result.ErrorCount++
			continue
		}

		// Train/validate algorithm (simplified - in real world you'd do proper ML training)
		_, err = algorithm.Predict(ctx, stockData)
		if err != nil {
			logger.Logger.Debugf("⚠️ Failed to validate %s with %s: %v", stock.Symbol, algName, err)
			result.ErrorCount++
			continue
		}

		// Add algorithm accuracy (in real world, calculate based on backtest)
		totalAccuracy += algorithm.GetAccuracy()
		validStocks++
		result.SuccessCount++
	}

	// Calculate average accuracy
	if validStocks > 0 {
		avgAccuracy := totalAccuracy / float64(validStocks) * 100
		result.Accuracy = decimal.NewFromFloat(avgAccuracy)
	}

	result.Duration = time.Since(startTime)
	return result
}

// generateStockPrediction generates prediction for a single stock
func generateStockPrediction(ctx context.Context, stock modelsdb.Stock, algName string, algorithm PredictionAlgorithm) (*modelsdb.Prediction, error) {
	// Get recent stock data
	stockData, err := getStockPredictionData(stock.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get stock data: %v", err)
	}

	// Generate prediction
	prediction, err := algorithm.Predict(ctx, stockData)
	if err != nil {
		return nil, fmt.Errorf("prediction failed: %v", err)
	}

	// Prefer the prediction's own confidence; fall back to algorithm's backtest accuracy
	confidence := prediction.Confidence
	if confidence == 0 {
		confidence = algorithm.GetAccuracy()
	}

	// Create database record
	dbPrediction := &modelsdb.Prediction{
		StockID:        stock.ID,
		PredictedPrice: decimal.NewFromFloat(prediction.PredictedPrice),
		CurrentPrice:   decimal.NewFromFloat(prediction.CurrentPrice),
		Confidence:     decimal.NewFromFloat(confidence),
		AlgorithmName:  algName,
		PredictionDate: time.Now(),
		TargetDate:     time.Now().AddDate(0, 0, 1), // Next trading day
	}

	return dbPrediction, nil
}

// getStockTrainingData gets historical data for training (last 6 months)
func getStockTrainingData(stockID uint) (*modelssvc.StockData, error) {
	// Get last 6 months of data for training
	fromDate := time.Now().AddDate(0, -6, 0)
	toDate := time.Now()

	prices, err := predictor.store.GetStockPricesByStockIDAndDateRange(stockID, fromDate, toDate)
	if err != nil {
		return nil, err
	}

	if len(prices) < 30 {
		return nil, fmt.Errorf("insufficient data: only %d records", len(prices))
	}

	// Convert to string array (as expected by algorithms)
	historical := make([]string, len(prices))
	for i, price := range prices {
		historical[i] = price.ClosePrice.String()
	}

	return &modelssvc.StockData{
		Historical: historical,
	}, nil
}

// getStockPredictionData gets recent data for prediction (last 9 months)
func getStockPredictionData(stockID uint) (*modelssvc.StockData, error) {
	// Get last 9 months of data for prediction (~180 trading days, enough for LSTM seq=60 and ARIMA min=100)
	fromDate := time.Now().AddDate(0, -9, 0)
	toDate := time.Now()

	prices, err := predictor.store.GetStockPricesByStockIDAndDateRange(stockID, fromDate, toDate)
	if err != nil {
		return nil, err
	}

	if len(prices) < 20 {
		return nil, fmt.Errorf("insufficient data: only %d records", len(prices))
	}

	// Convert to string array
	historical := make([]string, len(prices))
	for i, price := range prices {
		historical[i] = price.ClosePrice.String()
	}

	return &modelssvc.StockData{
		Historical: historical,
	}, nil
}

// logTrainingResults logs training results to sync_logs
func logTrainingResults(results []TrainingResult, totalDuration time.Duration) error {
	var messages []string
	totalSuccess := 0
	totalErrors := 0

	for _, result := range results {
		totalSuccess += result.SuccessCount
		totalErrors += result.ErrorCount

		msg := fmt.Sprintf("%s: %d/%d successful (%v accuracy)",
			result.AlgorithmName, result.SuccessCount, result.TotalStocks, result.Accuracy)
		messages = append(messages, msg)
	}

	syncLog := &modelsdb.SyncLog{
		SyncDate:     time.Now(),
		SuccessCount: totalSuccess,
		ErrorCount:   totalErrors,
		DurationMs:   totalDuration.Milliseconds(),
		Source:       "ML Training",
		ErrorMessage: strings.Join(messages, "; "),
	}

	return predictor.store.CreateSyncLog(syncLog)
}

// TrainSingleAlgorithmByName trains a single named algorithm immediately and saves a TrainingLog.
// It returns the session ID for tracking.
func TrainSingleAlgorithmByName(algorithmName string) (string, error) {
	if predictor == nil {
		return "", fmt.Errorf("prediction service not initialized")
	}

	predictor.mutex.Lock()
	if predictor.isTraining {
		predictor.mutex.Unlock()
		return "", fmt.Errorf("training already in progress")
	}
	predictor.isTraining = true
	predictor.mutex.Unlock()

	defer func() {
		predictor.mutex.Lock()
		predictor.isTraining = false
		predictor.lastTrained = time.Now()
		predictor.mutex.Unlock()
	}()

	algorithm, ok := predictor.algorithms[algorithmName]
	if !ok {
		return "", fmt.Errorf("unknown algorithm: %s", algorithmName)
	}

	stocks, err := predictor.store.GetVN30Stocks()
	if err != nil {
		return "", fmt.Errorf("failed to get stocks: %v", err)
	}

	sessionID := generateSessionID()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	result := trainSingleAlgorithm(ctx, algorithmName, algorithm, stocks)

	trainingLog := &modelsdb.TrainingLog{
		SessionID:     sessionID,
		AlgorithmName: algorithmName,
		TotalStocks:   result.TotalStocks,
		SuccessCount:  result.SuccessCount,
		ErrorCount:    result.ErrorCount,
		Accuracy:      result.Accuracy,
		DurationMs:    result.Duration.Milliseconds(),
		StartedAt:     result.TrainedAt,
		CompletedAt:   result.TrainedAt.Add(result.Duration),
	}
	predictor.store.CreateTrainingLog(trainingLog)

	return sessionID, nil
}

// TrainAllAlgorithms kicks off a full training run in the background and returns a session ID.
func TrainAllAlgorithms() (string, error) {
	if predictor == nil {
		return "", fmt.Errorf("prediction service not initialized")
	}

	predictor.mutex.Lock()
	if predictor.isTraining {
		predictor.mutex.Unlock()
		return "", fmt.Errorf("training already in progress")
	}
	predictor.isTraining = true
	predictor.mutex.Unlock()

	sessionID := generateSessionID()

	go func() {
		defer func() {
			predictor.mutex.Lock()
			predictor.isTraining = false
			predictor.lastTrained = time.Now()
			predictor.mutex.Unlock()
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()

		stocks, err := predictor.store.GetVN30Stocks()
		if err != nil {
			logger.Logger.Errorf("Failed to get VN30 stocks for manual training: %v", err)
			return
		}

		var trainingResults []TrainingResult
		for algName, algorithm := range predictor.algorithms {
			result := trainSingleAlgorithm(ctx, algName, algorithm, stocks)
			trainingResults = append(trainingResults, result)

			trainingLog := &modelsdb.TrainingLog{
				SessionID:     sessionID,
				AlgorithmName: algName,
				TotalStocks:   result.TotalStocks,
				SuccessCount:  result.SuccessCount,
				ErrorCount:    result.ErrorCount,
				Accuracy:      result.Accuracy,
				DurationMs:    result.Duration.Milliseconds(),
				StartedAt:     result.TrainedAt,
				CompletedAt:   result.TrainedAt.Add(result.Duration),
			}
			if err := predictor.store.CreateTrainingLog(trainingLog); err != nil {
				logger.Logger.Errorf("Failed to save training log: %v", err)
			}
		}

		duration := time.Since(trainingResults[0].TrainedAt)
		logTrainingResults(trainingResults, duration)
		logger.Logger.Info("Manual training completed")
	}()

	return sessionID, nil
}

// generateSessionID generates a random UUID-like session identifier using crypto/rand
func generateSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// PredictSingleStock runs all algorithms on a single stock and saves predictions
func PredictSingleStock(symbol string) (int, error) {
	if predictor == nil {
		return 0, fmt.Errorf("prediction service not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	stock, err := predictor.store.GetStockBySymbol(symbol)
	if err != nil {
		return 0, fmt.Errorf("stock not found: %s", symbol)
	}

	successCount := 0
	for algName, algorithm := range predictor.algorithms {
		prediction, err := generateStockPrediction(ctx, *stock, algName, algorithm)
		if err != nil {
			logger.Logger.Errorf("Failed to predict %s with %s: %v", symbol, algName, err)
			continue
		}

		err = predictor.store.CreatePrediction(prediction)
		if err != nil {
			logger.Logger.Errorf("Failed to save prediction for %s: %v", symbol, err)
			continue
		}
		successCount++
	}

	return successCount, nil
}

// GetPredictionService returns the global prediction service instance
func GetPredictionService() *PredictionService {
	return predictor
}

// IsTraining returns whether the service is currently training
func IsTraining() bool {
	if predictor == nil {
		return false
	}

	predictor.mutex.RLock()
	defer predictor.mutex.RUnlock()
	return predictor.isTraining
}

// GetLastTrainedTime returns the last training time
func GetLastTrainedTime() time.Time {
	if predictor == nil {
		return time.Time{}
	}

	predictor.mutex.RLock()
	defer predictor.mutex.RUnlock()
	return predictor.lastTrained
}
