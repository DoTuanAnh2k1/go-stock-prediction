package config

import (
	"go-stock-prediction/pkg/models/models_config"
	"go-stock-prediction/pkg/utils/env"

	"github.com/joho/godotenv"
)

func InitConfig(filenames ...string) {
	err := godotenv.Load(filenames...)
	if err != nil {
		panic(err)
	}

	cfg := &models_config.Config{
		Svr: models_config.ServerConfig{
			ServerName: env.GetEnv("SERVER_NAME", "go-stock-prediction"),
			Host:       env.GetEnv("SERVER_HOST", "0.0.0.0"),
			Port:       env.GetEnv("SERVER_PORT", "31300"),
			APIKey:     env.GetEnv("API_KEY", ""),
		},
		Db: models_config.DatabaseConfig{
			DbType: env.GetEnv("DB_DRIVER", "mysql"),
			Mysql: models_config.MySqlConfig{
				Host:     env.GetEnv("MYSQL_HOST", "localhost"),
				Port:     env.GetEnv("MYSQL_PORT", "3306"),
				User:     env.GetEnv("MYSQL_USER", "root"),
				Password: env.GetEnv("MYSQL_PASSWORD", ""),
				Name:     env.GetEnv("MYSQL_DB_NAME", "go_stock_prediction"),
				Debug:    env.GetEnv("MYSQL_DEBUG", "false") == "true",
			},
		},
		Log: models_config.LogConfig{
			Level:   env.GetEnv("LOG_LEVEL", "DEBUG"),
			DbLevel: env.GetEnv("DB_LOG_LEVEL", "DEBUG"),
		},
	}

	Init(cfg)
}
