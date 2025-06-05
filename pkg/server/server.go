package server

import (
	"go-stock-prediction/pkg/config"
	"go-stock-prediction/pkg/logger"
	"net/http"
)

func StartHTTPServer() {
	mux := addHandler()
	serverAddr := config.GetServerConfig().Host + ":" + config.GetServerConfig().Port
	server := &http.Server{
		Addr:    serverAddr,
		Handler: mux,
	}
	logger.Logger.Infof("Starting HTTP server on %s", serverAddr)
	err := server.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
