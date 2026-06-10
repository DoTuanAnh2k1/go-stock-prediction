package models_config

type Config struct {
	Db   DatabaseConfig
	Svr  ServerConfig
	Log  LogConfig
	GRPC GRPCConfig
}

type GRPCConfig struct {
	// ServerPort is the port the prediction gRPC server listens on
	ServerPort string
	// ClientTarget is the address the API backend uses to connect to the prediction gRPC server
	ClientTarget string
}

type ServerConfig struct {
	ServerName    string
	Host          string
	Port          string
	APIKey        string
	AdminUsername string
	AdminPassword string
	JWTSecret     string
}

type LogConfig struct {
	Level   string
	DbLevel string
}

type DatabaseConfig struct {
	DbType string
	Mysql  MySqlConfig
	Pgsql  PostgresConfig
}

var DatabaseConfigInit Config
