package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	"go-stock-prediction/pkg/config"
	authclient "go-stock-prediction/pkg/grpc/authclient"
	grpcclient "go-stock-prediction/pkg/grpc/client"
	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/server"
	"go-stock-prediction/pkg/store/repository"

	_ "go-stock-prediction/docs"
)

//	@title			Go Stock Prediction API
//	@version		1.0
//	@description	Financial asset prediction system — ML algorithms (VWMA, EMA, LSTM, ARIMA-GARCH, Ensemble) for gold, NASDAQ, crypto, and S&P 500.
//	@host			localhost:8118
//	@BasePath		/
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				JWT token from POST /api/auth/login. Format: Bearer {token}

func main() {
	// Initialize the configuration
	config.InitConfig()

	// Set timezone to Asia/Ho_Chi_Minh
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		panic(fmt.Sprintf("Failed to load timezone: %v", err))
	}
	time.Local = loc

	// Initialize the logger
	logger.Init()

	// Initialize the database connection (read queries)
	repository.Init()

	// Initialize gRPC client for Java Auth Service
	authclient.Init(config.GetAuthGRPCConfig())

	// Initialize gRPC client pointing at the prediction service
	grpcclient.Init(config.GetGRPCConfig().ClientTarget)

	// Start the scheduled database backup (DB-backed schedule, editable via Settings)
	server.StartBackupScheduler(repository.GetSingleton())

	// Start the HTTP API server
	go server.StartHTTPServer()

	// Wait for shutdown signal
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)
	sig := <-signals
	logger.Logger.Infof("Received signal %v — shutting down API service", sig)
	server.StopBackupScheduler()
	authclient.Close()
	grpcclient.Close()
	logger.Logger.Info("API service stopped")
}
