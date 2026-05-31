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
	_ "go-stock-prediction/pkg/service/market/crypto"   // registers CRYPTO market
	_ "go-stock-prediction/pkg/service/market/fuel"     // registers FUEL market
	_ "go-stock-prediction/pkg/service/market/gold"     // registers GOLD market
	_ "go-stock-prediction/pkg/service/market/nasdaq100" // registers NASDAQ100 market
	_ "go-stock-prediction/pkg/service/market/vn30"     // registers VN30 market
	_ "go-stock-prediction/pkg/service/predict"      // registers stock asset type via init()
	"go-stock-prediction/pkg/service/predict/assettype"
	_ "go-stock-prediction/pkg/service/predict/gold" // registers gold asset type via init()
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

	// Initialize all registered asset prediction types (stock, gold, and future types).
	// To add a new prediction type: implement assettype.AssetPredictionType and import it here.
	for _, t := range assettype.All() {
		t := t
		go t.Init()
	}

	// Wait for shutdown signal
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)
	sig := <-signals
	logger.Logger.Infof("Received signal %v — shutting down prediction service", sig)
	grpcSrv.Stop()
	logger.Logger.Info("Prediction service stopped")
}
