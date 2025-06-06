package server

import (
	"go-stock-prediction/pkg/config"
	"go-stock-prediction/pkg/logger"
	"net/http"
	"time"
)

func StartHTTPServer() {
	// Setup all routes (web + API)
	mux := SetupAllRoutes()

	// Get server config
	serverAddr := config.GetServerConfig().Host + ":" + config.GetServerConfig().Port

	// Create HTTP server với proper configuration
	server := &http.Server{
		Addr:    serverAddr,
		Handler: mux,
		// Timeout configurations for production
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Log server startup info
	logger.Logger.Infof("🚀 Starting VN Stock Prediction Server...")
	logger.Logger.Infof("📡 Server Address: %s", serverAddr)
	logger.Logger.Infof("🌐 Dashboard URL: http://%s", serverAddr)
	logger.Logger.Infof("🔌 API Base URL: http://%s/api", serverAddr)
	logger.Logger.Infof("❤️ Health Check: http://%s/health", serverAddr)

	// Start server
	logger.Logger.Infof("✅ HTTP Server listening on %s", serverAddr)
	err := server.ListenAndServe()
	if err != nil {
		logger.Logger.Fatalf("❌ Server failed to start: %v", err)
		panic(err)
	}
}
