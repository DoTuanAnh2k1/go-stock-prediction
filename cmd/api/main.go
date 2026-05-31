package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	"go-stock-prediction/pkg/config"
	grpcclient "go-stock-prediction/pkg/grpc/client"
	"go-stock-prediction/pkg/logger"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"go-stock-prediction/pkg/server"
	"go-stock-prediction/pkg/store/repository"
	"golang.org/x/crypto/bcrypt"

	_ "go-stock-prediction/docs"
)

//	@title			Go Stock Prediction API
//	@version		1.0
//	@description	Vietnamese stock market prediction system — ML algorithms (VWMA, EMA, LSTM, ARIMA-GARCH, Ensemble) for VN30 stocks, gold, NASDAQ, crypto, and fuel prices.
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

	// Seed admin user if no admin exists
	seedAdminUser()

	// Initialize gRPC client pointing at the prediction service
	grpcclient.Init(config.GetGRPCConfig().ClientTarget)

	// Start the HTTP API server
	go server.StartHTTPServer()

	// Wait for shutdown signal
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)
	sig := <-signals
	logger.Logger.Infof("Received signal %v — shutting down API service", sig)
	grpcclient.Close()
	logger.Logger.Info("API service stopped")
}

func seedAdminUser() {
	store := repository.GetSingleton()
	exists, err := store.AdminExists()
	if err != nil {
		logger.Logger.Errorf("Failed to check admin existence: %v", err)
		return
	}
	if exists {
		return
	}
	cfg := config.GetServerConfig()
	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		logger.Logger.Errorf("Failed to hash admin password: %v", err)
		return
	}
	if err := store.CreateUser(&modelsdb.User{
		Username:     cfg.AdminUsername,
		PasswordHash: string(hash),
		Role:         "admin",
	}); err != nil {
		logger.Logger.Errorf("Failed to seed admin user: %v", err)
		return
	}
	logger.Logger.Infof("Admin user '%s' seeded successfully", cfg.AdminUsername)
}
