package server

import (
	"net/http"

	"go-stock-prediction/pkg/logger"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

func addHandler() *http.ServeMux {
	mux := http.NewServeMux()

	// Swagger UI
	mux.Handle("/swagger/", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	// Health check endpoints
	mux.HandleFunc("/health", HealthCheckHandler)
	mux.HandleFunc("/health/simple", SimpleHealthHandler)
	mux.HandleFunc("/health/ready", ReadyHandler)

	// ===========================================
	// API ROUTES (JSON responses)
	// ===========================================

	// Auth APIs
	mux.HandleFunc("POST /api/auth/login", LoginHandler)
	mux.HandleFunc("GET /api/auth/me", MeHandler)
	mux.HandleFunc("PUT /api/auth/password", ChangePasswordHandler)

	// Dashboard APIs
	mux.HandleFunc("/api/dashboard/stats", GetDashboardStats)

	// Training & Prediction APIs
	mux.HandleFunc("/api/training/status", GetTrainingStatus)
	mux.HandleFunc("/api/training/history", GetTrainingHistory)
	mux.HandleFunc("/api/training/algorithms", GetTrainingAlgorithms)
	mux.HandleFunc("/api/training/metrics", GetTrainingMetrics)
	mux.HandleFunc("/api/training/", GetTrainingDetail) // /api/training/{id} — must be after specific paths
	mux.HandleFunc("/api/predictions", GetPredictions)
	mux.HandleFunc("/api/predictions/accuracy", GetPredictionAccuracy)
	mux.HandleFunc("/api/predictions/accuracy-trend", GetAccuracyTrend)
	mux.HandleFunc("/api/predictions/compare/{symbol}", GetPredictionCompare)
	mux.HandleFunc("/api/predictions/error-distribution", GetErrorDistribution)
	mux.HandleFunc("/api/predictions/", GetPredictionDetail)
	mux.HandleFunc("/api/algorithms/comparison", GetAlgorithmComparison)
	mux.HandleFunc("/api/algorithms/backtest", GetAlgorithmBacktest)

	// Stock data APIs
	mux.HandleFunc("/api/stocks/{symbol}/chart", GetChartData)
	mux.HandleFunc("/api/stocks/{symbol}/current", GetCurrentPrice)
	mux.HandleFunc("/api/stocks/{symbol}/history", GetHistoricalData)
	mux.HandleFunc("/api/stocks/{symbol}/detail", GetStockDetail)
	mux.HandleFunc("/api/stocks/watchlist", GetStockWatchlist)
	mux.HandleFunc("/api/market/overview", GetMarketOverview)

	// Gold price APIs
	mux.HandleFunc("/api/gold/latest", GetGoldLatest)
	mux.HandleFunc("/api/gold/prices", GetGoldPrices)
	mux.HandleFunc("/api/gold/chart", GetGoldChart)

	// Gold prediction APIs
	mux.HandleFunc("/api/gold/predictions/latest", GetLatestGoldPredictions)
	mux.HandleFunc("/api/gold/predictions/chart", GetGoldPredictionChart)
	mux.HandleFunc("/api/gold/predictions", GetGoldPredictions)

	// NASDAQ data APIs
	mux.HandleFunc("/api/nasdaq/latest", GetNasdaqLatest)
	mux.HandleFunc("/api/nasdaq/prices", GetNasdaqPrices)
	mux.HandleFunc("/api/nasdaq/chart", GetNasdaqChart)

	// NASDAQ prediction APIs
	mux.HandleFunc("/api/nasdaq/predictions/latest", GetNasdaqPredictionsLatest)
	mux.HandleFunc("/api/nasdaq/predictions/chart", GetNasdaqPredictionsChart)
	mux.HandleFunc("/api/nasdaq/predictions", GetNasdaqPredictions)

	// Crypto data APIs
	mux.HandleFunc("/api/crypto/latest", GetCryptoLatest)
	mux.HandleFunc("/api/crypto/prices", GetCryptoPrices)
	mux.HandleFunc("/api/crypto/chart", GetCryptoChart)

	// Crypto prediction APIs
	mux.HandleFunc("/api/crypto/predictions/latest", GetCryptoPredictionsLatest)
	mux.HandleFunc("/api/crypto/predictions/chart", GetCryptoPredictionsChart)
	mux.HandleFunc("/api/crypto/predictions", GetCryptoPredictions)

	// Fuel data APIs
	mux.HandleFunc("/api/fuel/latest", GetFuelLatest)
	mux.HandleFunc("/api/fuel/prices", GetFuelPrices)
	mux.HandleFunc("/api/fuel/chart", GetFuelChart)

	// Fuel prediction APIs
	mux.HandleFunc("/api/fuel/predictions/latest", GetFuelPredictionsLatest)
	mux.HandleFunc("/api/fuel/predictions/chart", GetFuelPredictionsChart)
	mux.HandleFunc("/api/fuel/predictions", GetFuelPredictions)

	// Market-level paginated APIs
	mux.HandleFunc("/api/markets/{key}/predictions", GetMarketPredictions)
	mux.HandleFunc("/api/markets/{key}/training", GetMarketTraining)

	// Per-stock crawl and predict APIs
	mux.HandleFunc("POST /api/stocks/{symbol}/crawl", TriggerStockCrawl)
	mux.HandleFunc("POST /api/stocks/{symbol}/predict", TriggerStockPredict)

	// Trigger APIs (require JWT authentication)
	mux.HandleFunc("POST /api/trigger/crawler", AuthRequired(TriggerCrawlerHandler))
	mux.HandleFunc("POST /api/trigger/predict", AuthRequired(TriggerPredictHandler))
	mux.HandleFunc("POST /api/trigger/gold-crawler", AuthRequired(TriggerGoldCrawlerHandler))
	mux.HandleFunc("POST /api/trigger/gold-history", AuthRequired(TriggerGoldHistoryHandler))
	mux.HandleFunc("POST /api/trigger/gold-predict", AuthRequired(TriggerGoldPredictHandler))
	mux.HandleFunc("POST /api/trigger/train", AuthRequired(TriggerTrainHandler))
	mux.HandleFunc("POST /api/trigger/reconcile", AuthRequired(TriggerReconcileHandler))
	mux.HandleFunc("POST /api/trigger/stock-history", AuthRequired(TriggerStockHistoryHandler))
	mux.HandleFunc("POST /api/trigger/historical-backtest", AuthRequired(TriggerHistoricalBacktestHandler))
	mux.HandleFunc("POST /api/trigger/gold-historical-backtest", AuthRequired(TriggerGoldHistoricalBacktestHandler))
	mux.HandleFunc("POST /api/trigger/nasdaq-crawler", AuthRequired(TriggerNasdaqCrawlerHandler))
	mux.HandleFunc("POST /api/trigger/nasdaq-predict", AuthRequired(TriggerNasdaqPredictHandler))
	mux.HandleFunc("POST /api/trigger/crypto-crawler", AuthRequired(TriggerCryptoCrawlerHandler))
	mux.HandleFunc("POST /api/trigger/crypto-predict", AuthRequired(TriggerCryptoPredictHandler))
	mux.HandleFunc("POST /api/trigger/fuel-crawler", AuthRequired(TriggerFuelCrawlerHandler))
	mux.HandleFunc("POST /api/trigger/fuel-predict", AuthRequired(TriggerFuelPredictHandler))
	mux.HandleFunc("POST /api/trigger/sp500-crawler", AuthRequired(TriggerSP500CrawlerHandler))
	mux.HandleFunc("POST /api/trigger/sp500-predict", AuthRequired(TriggerSP500PredictHandler))

	// User management APIs (admin only)
	mux.HandleFunc("GET /api/users", ListUsersHandler)
	mux.HandleFunc("POST /api/users", CreateUserHandler)
	mux.HandleFunc("DELETE /api/users/{id}", DeleteUserHandler)

	// Cron schedule APIs (require JWT authentication)
	mux.HandleFunc("GET /api/schedules", AuthRequired(GetSchedulesHandler))
	mux.HandleFunc("PUT /api/schedules/{key}", AuthRequired(UpdateScheduleHandler))

	return mux
}

func SetupAllRoutes() *http.ServeMux {
	mux := addHandler()
	logRegisteredRoutes()
	return mux
}

func logRegisteredRoutes() {
	logger.Logger.Info("Registered Routes:")
	logger.Logger.Info("  GET  /swagger/            -> Swagger UI")
	logger.Logger.Info("Health Routes:")
	logger.Logger.Info("  GET  /health              -> Detailed Health Check")
	logger.Logger.Info("  GET  /health/simple       -> Simple Health Check")
	logger.Logger.Info("  GET  /health/ready        -> Readiness Check")
	logger.Logger.Info("API Routes:")
	logger.Logger.Info("  POST /api/auth/login")
	logger.Logger.Info("  GET  /api/auth/me")
	logger.Logger.Info("  PUT  /api/auth/password")
	logger.Logger.Info("  GET  /api/dashboard/stats")
	logger.Logger.Info("  GET  /api/training/status")
	logger.Logger.Info("  GET  /api/training/history")
	logger.Logger.Info("  GET  /api/training/algorithms")
	logger.Logger.Info("  GET  /api/training/metrics")
	logger.Logger.Info("  GET  /api/training/{id}")
	logger.Logger.Info("  GET  /api/predictions")
	logger.Logger.Info("  GET  /api/predictions/accuracy")
	logger.Logger.Info("  GET  /api/predictions/accuracy-trend")
	logger.Logger.Info("  GET  /api/predictions/compare/{symbol}")
	logger.Logger.Info("  GET  /api/predictions/error-distribution")
	logger.Logger.Info("  GET  /api/predictions/{id}")
	logger.Logger.Info("  GET  /api/algorithms/comparison")
	logger.Logger.Info("  GET  /api/algorithms/backtest")
	logger.Logger.Info("  GET  /api/stocks/{symbol}/chart")
	logger.Logger.Info("  GET  /api/stocks/{symbol}/current")
	logger.Logger.Info("  GET  /api/stocks/{symbol}/history")
	logger.Logger.Info("  GET  /api/stocks/{symbol}/detail")
	logger.Logger.Info("  GET  /api/stocks/watchlist")
	logger.Logger.Info("  GET  /api/market/overview")
	logger.Logger.Info("  GET  /api/gold/latest")
	logger.Logger.Info("  GET  /api/gold/prices")
	logger.Logger.Info("  GET  /api/gold/chart")
	logger.Logger.Info("  GET  /api/gold/predictions/latest")
	logger.Logger.Info("  GET  /api/gold/predictions/chart")
	logger.Logger.Info("  GET  /api/gold/predictions")
	logger.Logger.Info("  GET  /api/markets/{key}/predictions")
	logger.Logger.Info("  GET  /api/markets/{key}/training")
	logger.Logger.Info("  POST /api/stocks/{symbol}/crawl")
	logger.Logger.Info("  POST /api/stocks/{symbol}/predict")
	logger.Logger.Info("  POST /api/trigger/crawler")
	logger.Logger.Info("  POST /api/trigger/predict")
	logger.Logger.Info("  POST /api/trigger/gold-crawler")
	logger.Logger.Info("  POST /api/trigger/gold-history")
	logger.Logger.Info("  POST /api/trigger/gold-predict")
	logger.Logger.Info("  POST /api/trigger/train")
	logger.Logger.Info("  POST /api/trigger/reconcile")
	logger.Logger.Info("  POST /api/trigger/stock-history")
	logger.Logger.Info("  POST /api/trigger/historical-backtest")
	logger.Logger.Info("  POST /api/trigger/gold-historical-backtest")
	logger.Logger.Info("  GET  /api/users")
	logger.Logger.Info("  POST /api/users")
	logger.Logger.Info("  DELETE /api/users/{id}")
	logger.Logger.Info("  GET  /api/schedules")
	logger.Logger.Info("  PUT  /api/schedules/{key}")
	logger.Logger.Info("  GET  /api/nasdaq/latest")
	logger.Logger.Info("  GET  /api/nasdaq/prices")
	logger.Logger.Info("  GET  /api/nasdaq/chart")
	logger.Logger.Info("  GET  /api/nasdaq/predictions/latest")
	logger.Logger.Info("  GET  /api/nasdaq/predictions/chart")
	logger.Logger.Info("  GET  /api/nasdaq/predictions")
	logger.Logger.Info("  GET  /api/crypto/latest")
	logger.Logger.Info("  GET  /api/crypto/prices")
	logger.Logger.Info("  GET  /api/crypto/chart")
	logger.Logger.Info("  GET  /api/crypto/predictions/latest")
	logger.Logger.Info("  GET  /api/crypto/predictions/chart")
	logger.Logger.Info("  GET  /api/crypto/predictions")
	logger.Logger.Info("  GET  /api/fuel/latest")
	logger.Logger.Info("  GET  /api/fuel/prices")
	logger.Logger.Info("  GET  /api/fuel/chart")
	logger.Logger.Info("  GET  /api/fuel/predictions/latest")
	logger.Logger.Info("  GET  /api/fuel/predictions/chart")
	logger.Logger.Info("  GET  /api/fuel/predictions")
	logger.Logger.Info("  POST /api/trigger/nasdaq-crawler")
	logger.Logger.Info("  POST /api/trigger/nasdaq-predict")
	logger.Logger.Info("  POST /api/trigger/crypto-crawler")
	logger.Logger.Info("  POST /api/trigger/crypto-predict")
	logger.Logger.Info("  POST /api/trigger/fuel-crawler")
	logger.Logger.Info("  POST /api/trigger/fuel-predict")
	logger.Logger.Info("  POST /api/trigger/sp500-crawler")
	logger.Logger.Info("  POST /api/trigger/sp500-predict")
}
