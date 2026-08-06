package server

import (
	"go-stock-prediction/pkg/config"
	"go-stock-prediction/pkg/logger"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func StartHTTPServer() {
	// Setup all routes (web + API)
	mux := SetupAllRoutes()

	// Get server config
	serverAddr := config.GetServerConfig().Host + ":" + config.GetServerConfig().Port

	// otelhttp wraps the whole chain: it extracts the incoming W3C trace context
	// (traceparent header, e.g. from the gateway) and starts a server span for
	// every request so the trace continues into the outbound gRPC calls.
	handler := otelhttp.NewHandler(
		RequestIDMiddleware(CORSMiddleware(JWTMiddleware(AccessLogMiddleware(mux)))),
		"api-svc",
	)

	// Create HTTP server with proper configuration
	server := &http.Server{
		Addr:    serverAddr,
		Handler: handler,
		// Timeout configurations for production
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Log server startup info
	logger.Logger.Infof("starting api-svc HTTP server")
	logger.Logger.Infof("server address: %s", serverAddr)
	logger.Logger.Infof("api base url: http://%s/api", serverAddr)
	logger.Logger.Infof("health check: http://%s/health", serverAddr)

	// Start server
	logger.Logger.Infof("HTTP server listening on %s", serverAddr)
	err := server.ListenAndServe()
	if err != nil {
		logger.Logger.Fatalf("server failed to start: %v", err)
		panic(err)
	}
}
