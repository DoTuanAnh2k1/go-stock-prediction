package main

import (
	"fmt"
	"go-stock-prediction/pkg/config"
	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/server"
	"go-stock-prediction/pkg/service/crawler"
	"go-stock-prediction/pkg/service/predict"
	"go-stock-prediction/pkg/store/repository"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"
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

	go server.StartHTTPServer()

	go crawler.Init()

	go func() {
		// Wait for DB and services to be ready
		time.Sleep(5 * time.Second)
		logger.Logger.Info("Triggering startup data sync...")
		if err := crawler.CronjobCrawler(); err != nil {
			logger.Logger.Errorf("Startup crawler failed: %v", err)
		}
	}()

	go predict.Init()

	stopOrKillServer()
}

func stopOrKillServer() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)
	sig := <-signals
	fmt.Println("Receive Signal from OS - Release resource")
	fmt.Println(sig)
	os.Exit(1)
}
