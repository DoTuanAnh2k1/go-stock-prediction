package repository

import (
	"go-stock-prediction/pkg/config"
	"go-stock-prediction/pkg/store/grpcstore"
	"go-stock-prediction/pkg/store/mysql"
	"go-stock-prediction/pkg/store/postgres"
)

var (
	store DatabaseStore
)

func GetSingleton() DatabaseStore {
	return store
}

func Init() {
	cfg := config.GetDatabaseConfig()
	switch cfg.DbType {
	case "mysql":
		store = mysql.GetInstance()
	case "postgresql":
		store = postgres.GetInstance()
	case "grpc":
		// Phase 2 — DB-less api-svc: read facade over prediction-svc (market_db owner).
		store = grpcstore.New(config.GetGRPCConfig().ClientTarget)
	default:
		panic("unsupported database type: " + cfg.DbType)
	}
	err := store.Init(cfg)
	if err != nil {
		panic("cant init store: " + err.Error())
	}
}
