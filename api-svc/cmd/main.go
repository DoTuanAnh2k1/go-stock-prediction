package main

import (
	"context"
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
	"go-stock-prediction/pkg/version"

	regclient "go-stock-prediction/service-mgt/client"

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
//	@description				JWT token from POST /api/x/grant. Format: Bearer {token}

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

	// Stamp version info (reads GIT_SHA/BUILD_TIME/GIT_DIRTY env vars, logs, writes /versions/api-svc.json)
	version.Init()

	// Initialize the database connection (read queries)
	repository.Init()

	// Service registry client — registers this service and resolves peer
	// endpoints. No-op when SERVICE_MGT_ENABLED=false (static targets used).
	reg := regclient.New(regclient.Options{
		Enabled:        config.GetServiceMgtEnabled(),
		RegistryTarget: config.GetRegistryTarget(),
		ServiceName:    "api-svc",
		Address:        "api-svc",
		Port:           8118,
	})
	if err := reg.Start(context.Background()); err != nil {
		logger.Logger.Warnf("service registry start failed, using static targets: %v", err)
	}

	// Resolve gRPC targets via the registry, falling back to static env targets.
	authTarget := reg.Resolve("auth-svc", config.GetAuthGRPCConfig())
	predTarget := reg.Resolve("prediction-svc", config.GetGRPCConfig().ClientTarget)

	// Initialize gRPC client for Java Auth Service
	authclient.Init(authTarget)

	// Initialize gRPC client pointing at the prediction service
	grpcclient.Init(predTarget)

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
	reg.Stop()
	authclient.Close()
	grpcclient.Close()
	logger.Logger.Info("API service stopped")
}
