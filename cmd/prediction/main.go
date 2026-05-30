package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	"go-stock-prediction/pkg/config"
	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/service/crawler"
	"go-stock-prediction/pkg/service/predict"
	goldpredict "go-stock-prediction/pkg/service/predict/gold"
	"go-stock-prediction/pkg/store/repository"
	grpcserver "go-stock-prediction/pkg/grpc/server"
)

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

	// Initialize the database connection
	repository.Init()

	// Start the gRPC prediction server
	grpcSrv := grpcserver.New()
	if err := grpcSrv.Start(config.GetGRPCConfig().ServerPort); err != nil {
		logger.Logger.Fatalf("Failed to start gRPC server: %v", err)
	}

	// Initialize crawlers and register cron jobs
	go crawler.Init()

	// Trigger a startup data sync after services are ready
	go func() {
		time.Sleep(5 * time.Second)
		logger.Logger.Info("Triggering startup data sync...")
		if err := crawler.CronjobCrawler(); err != nil {
			logger.Logger.Errorf("Startup crawler failed: %v", err)
		}
	}()

	// Initialize prediction services and register cron jobs
	go predict.Init()
	go goldpredict.Init()

	// Wait for shutdown signal
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)
	sig := <-signals
	logger.Logger.Infof("Received signal %v — shutting down prediction service", sig)
	grpcSrv.Stop()
	logger.Logger.Info("Prediction service stopped")
}
