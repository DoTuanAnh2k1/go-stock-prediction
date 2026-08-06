package config

import (
	"go-stock-prediction/pkg/models/models_config"
	"go-stock-prediction/pkg/utils/env"

	"github.com/joho/godotenv"
)

func InitConfig(filenames ...string) {
	// Load .env file if present; in Docker, env vars are injected directly so missing .env is fine
	_ = godotenv.Load(filenames...)

	cfg := &models_config.Config{
		Svr: models_config.ServerConfig{
			ServerName: env.GetEnv("SERVER_NAME", "go-stock-prediction"),
			Host:       env.GetEnv("SERVER_HOST", "0.0.0.0"),
			Port:       env.GetEnv("SERVER_PORT", "8118"),
			APIKey:        env.GetEnv("API_KEY", ""),
			JWTSecret:     env.GetEnv("JWT_SECRET", "change-me-in-production"),
			InternalSecret: env.GetEnv("INTERNAL_SECRET", ""),
		},
		GRPC: models_config.GRPCConfig{
			ServerPort:       env.GetEnv("GRPC_SERVER_PORT", "8119"),
			ClientTarget:     env.GetEnv("GRPC_TARGET", "localhost:8119"),
			AuthClientTarget: env.GetEnv("AUTH_GRPC_TARGET", "localhost:8120"),
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
			Pgsql: models_config.PostgresConfig{
				Host:     env.GetEnv("POSTGRES_HOST", "localhost"),
				Port:     env.GetEnv("POSTGRES_PORT", "5432"),
				User:     env.GetEnv("POSTGRES_USER", "postgres"),
				Password: env.GetEnv("POSTGRES_PASSWORD", "123"),
				DbName:   env.GetEnv("POSTGRES_DB", "go_stock_prediction"),
				Debug:    env.GetEnv("POSTGRES_DEBUG", "false") == "true",
			},
		},
		Log: models_config.LogConfig{
			Level:   env.GetEnv("LOG_LEVEL", "DEBUG"),
			DbLevel: env.GetEnv("DB_LOG_LEVEL", "DEBUG"),
		},
		ServiceMgt: models_config.ServiceMgtConfig{
			Enabled:        env.GetEnv("SERVICE_MGT_ENABLED", "false") == "true",
			RegistryTarget: env.GetEnv("REGISTRY_GRPC_TARGET", "localhost:8121"),
		},
	}

	Init(cfg)
}
