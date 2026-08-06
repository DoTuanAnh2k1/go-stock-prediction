package config

import (
	"fmt"
	"os"
)

type Config struct {
	GRPCPort             string
	DBHost, DBPort       string
	DBUser, DBPassword   string
	DBName               string
	DefaultTTLSeconds    int
	EvictGraceSeconds    int
	FlushIntervalSeconds int
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func Load() Config {
	return Config{
		GRPCPort:             env("GRPC_PORT", "8121"),
		DBHost:               env("POSTGRES_HOST", "localhost"),
		DBPort:               env("POSTGRES_PORT", "5432"),
		DBUser:               env("POSTGRES_USER", "service_mgt"),   // database-per-service: registry_db owner
		DBPassword:           env("POSTGRES_PASSWORD", ""),
		DBName:               env("POSTGRES_DB", "registry_db"),
		DefaultTTLSeconds:    30,
		EvictGraceSeconds:    60,
		FlushIntervalSeconds: 30,
	}
}

func (c Config) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Ho_Chi_Minh",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName)
}
