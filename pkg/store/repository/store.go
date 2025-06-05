package repository

import (
	"go-stock-prediction/pkg/config"
	"go-stock-prediction/pkg/store/mysql"
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
	// case "postgresql":
	// 	store = postgres.GetInstance()
	//case "aerospike":
	//	store = aerospikes.GetInstance()
	default:
		panic("unsupported database type")
	}
	err := store.Init(cfg)
	if err != nil {
		panic("cant init store")
	}
}
