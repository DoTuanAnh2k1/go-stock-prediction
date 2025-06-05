package main

import (
	"fmt"
	"go-stock-prediction/pkg/config"
	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/server"
	"go-stock-prediction/pkg/store/repository"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Initialize the configuration
	config.InitConfig()

	// Initialize the logger
	logger.Init()

	// Initialize the database connection
	repository.Init()

	go server.StartHTTPServer()

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
