package config

import "go-stock-prediction/pkg/models/models_config"

var config *models_config.Config

func Init(cfg *models_config.Config) {
	config = cfg
}

func Get() *models_config.Config {
	return config
}

func GetServerConfig() models_config.ServerConfig {
	return config.Svr
}

func GetDatabaseConfig() models_config.DatabaseConfig {
	return config.Db
}

func GetLogConfig() models_config.LogConfig {
	if config == nil {
		return models_config.LogConfig{
			Level:   "DEBUG",
			DbLevel: "DEBUG",
		}
	}
	return config.Log
}

func GetGRPCConfig() models_config.GRPCConfig {
	if config == nil {
		return models_config.GRPCConfig{
			ServerPort:   "8119",
			ClientTarget: "localhost:8119",
		}
	}
	return config.GRPC
}
