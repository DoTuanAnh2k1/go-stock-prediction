package models_config

type Config struct {
	Db  DatabaseConfig
	Svr ServerConfig
	Log LogConfig
}

type ServerConfig struct {
	ServerName string
	Host       string
	Port       string
	APIKey     string
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
