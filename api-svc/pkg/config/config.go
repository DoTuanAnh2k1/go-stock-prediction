package config

import (
	"os"
	"strings"

	"go-stock-prediction/pkg/models/models_config"
)

var config *models_config.Config

func Init(cfg *models_config.Config) {
	config = cfg
}

func Get() *models_config.Config {
	return config
}

func GetServerConfig() models_config.ServerConfig {
	if config == nil {
		return models_config.ServerConfig{}
	}
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

func GetAuthGRPCConfig() string {
	if config == nil {
		return "localhost:8120"
	}
	return config.GRPC.AuthClientTarget
}

func GetServiceMgtEnabled() bool {
	if config == nil {
		return false
	}
	return config.ServiceMgt.Enabled
}

func GetRegistryTarget() string {
	if config == nil {
		return "localhost:8121"
	}
	return config.ServiceMgt.RegistryTarget
}

// GetBackupSchedulerEnabled reports whether the in-app database backup scheduler
// should run. Defaults to true (Docker Compose behaviour) and is set to false in
// k8s, where daily_backup runs as a CronJob instead of in-process. Read directly
// from the environment so it works before the config struct is fully wired.
func GetBackupSchedulerEnabled() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("BACKUP_SCHEDULER_ENABLED")))
	if v == "" {
		return true // default on for compose parity
	}
	return v != "false" && v != "0" && v != "no"
}
