package server

import (
	"go-stock-prediction/pkg/logger"
	"net/http"
)

func addHandler() *http.ServeMux {
	mux := http.NewServeMux()

	// Initialize web handler cho dashboard
	webHandler := NewWebHandler()

	// ===========================================
	// WEB ROUTES (Serve HTML pages)
	// ===========================================

	// Static files (CSS, JS, images)
	fs := http.FileServer(http.Dir("web/static/"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Main web pages
	mux.HandleFunc("/", webHandler.DashboardHandler)              // Dashboard chính
	mux.HandleFunc("/dashboard", webHandler.DashboardHandler)     // Dashboard alias
	mux.HandleFunc("/stocks", webHandler.StocksHandler)           // Stocks page
	mux.HandleFunc("/predictions", webHandler.PredictionsHandler) // Predictions page
	mux.HandleFunc("/training", webHandler.TrainingHandler)       // Training page

	// Health check endpoints
	mux.HandleFunc("/health", webHandler.HealthCheckHandler) // Detailed health
	mux.HandleFunc("/health/simple", SimpleHealthHandler)    // Simple health for LB
	mux.HandleFunc("/health/ready", ReadyHandler)            // Kubernetes readiness

	// ===========================================
	// API ROUTES (JSON responses)
	// ===========================================

	// Training & Prediction APIs
	mux.HandleFunc("/api/training/status", GetTrainingStatus)
	mux.HandleFunc("/api/training/history", GetTrainingHistory)
	mux.HandleFunc("/api/predictions", GetPredictions)
	mux.HandleFunc("/api/algorithms/comparison", GetAlgorithmComparison)

	// Stock data APIs
	mux.HandleFunc("/api/stocks/{symbol}/chart", GetChartData)
	mux.HandleFunc("/api/stocks/{symbol}/current", GetCurrentPrice)
	mux.HandleFunc("/api/stocks/{symbol}/history", GetHistoricalData) // Fix: removed space before /api
	mux.HandleFunc("/api/stocks/watchlist", GetStockWatchlist)
	mux.HandleFunc("/api/market/overview", GetMarketOverview)

	return mux
}

// Helper function để setup all routes với logging
func SetupAllRoutes() *http.ServeMux {
	mux := addHandler()

	// Log all registered routes (for debugging)
	logRegisteredRoutes()

	return mux
}

// Function để log ra tất cả routes đã đăng ký
func logRegisteredRoutes() {
	logger.Logger.Info("🛣️ Registered Routes:")
	logger.Logger.Info("📄 Web Routes:")
	logger.Logger.Info("  GET  /                    → Dashboard")
	logger.Logger.Info("  GET  /dashboard           → Dashboard")
	logger.Logger.Info("  GET  /stocks              → Stocks Page")
	logger.Logger.Info("  GET  /predictions         → Predictions Page")
	logger.Logger.Info("  GET  /training            → Training Page")
	logger.Logger.Info("  GET  /static/*            → Static Files")

	logger.Logger.Info("❤️ Health Routes:")
	logger.Logger.Info("  GET  /health              → Detailed Health Check")
	logger.Logger.Info("  GET  /health/simple       → Simple Health Check")
	logger.Logger.Info("  GET  /health/ready        → Readiness Check")

	logger.Logger.Info("🔌 API Routes:")
	logger.Logger.Info("  GET  /api/training/status           → Training Status")
	logger.Logger.Info("  GET  /api/training/history          → Training History")
	logger.Logger.Info("  GET  /api/predictions               → Predictions List")
	logger.Logger.Info("  GET  /api/algorithms/comparison     → Algorithm Comparison")
	logger.Logger.Info("  GET  /api/stocks/{symbol}/current   → Current Stock Price")
	logger.Logger.Info("  GET  /api/stocks/{symbol}/history   → Stock Price History")
	logger.Logger.Info("  GET  /api/stocks/{symbol}/chart     → Chart Data")
	logger.Logger.Info("  GET  /api/stocks/watchlist          → Watchlist")
	logger.Logger.Info("  GET  /api/market/overview           → Market Overview")
}
