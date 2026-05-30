package server

import (
	"go-stock-prediction/pkg/logger"
	"net/http"
)

func addHandler() *http.ServeMux {
	mux := http.NewServeMux()

	// Health check endpoints
	mux.HandleFunc("/health", HealthCheckHandler)
	mux.HandleFunc("/health/simple", SimpleHealthHandler)
	mux.HandleFunc("/health/ready", ReadyHandler)

	// ===========================================
	// API ROUTES (JSON responses)
	// ===========================================

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

	// Per-stock crawl and predict APIs
	mux.HandleFunc("POST /api/stocks/{symbol}/crawl", TriggerStockCrawl)
	mux.HandleFunc("POST /api/stocks/{symbol}/predict", TriggerStockPredict)

	// Trigger APIs
	mux.HandleFunc("POST /api/trigger/crawler", APIKeyMiddleware(TriggerCrawlerHandler))
	mux.HandleFunc("POST /api/trigger/predict", APIKeyMiddleware(TriggerPredictHandler))
	mux.HandleFunc("POST /api/trigger/gold-crawler", TriggerGoldCrawlerHandler)
	mux.HandleFunc("POST /api/trigger/gold-history", TriggerGoldHistoryHandler)
	mux.HandleFunc("POST /api/trigger/gold-predict", TriggerGoldPredictHandler)
	mux.HandleFunc("POST /api/trigger/train", TriggerTrainHandler)
	mux.HandleFunc("POST /api/trigger/reconcile", TriggerReconcileHandler)
	mux.HandleFunc("POST /api/trigger/stock-history", TriggerStockHistoryHandler)
	mux.HandleFunc("POST /api/trigger/historical-backtest", TriggerHistoricalBacktestHandler)

	return mux
}

func SetupAllRoutes() *http.ServeMux {
	mux := addHandler()
	logRegisteredRoutes()
	return mux
}

func logRegisteredRoutes() {
	logger.Logger.Info("Registered Routes:")
	logger.Logger.Info("Health Routes:")
	logger.Logger.Info("  GET  /health              -> Detailed Health Check")
	logger.Logger.Info("  GET  /health/simple       -> Simple Health Check")
	logger.Logger.Info("  GET  /health/ready        -> Readiness Check")
	logger.Logger.Info("API Routes:")
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
}
