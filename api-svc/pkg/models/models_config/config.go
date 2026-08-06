package models_config

type Config struct {
	Db         DatabaseConfig
	Svr        ServerConfig
	Log        LogConfig
	GRPC       GRPCConfig
	ServiceMgt ServiceMgtConfig
}

// ServiceMgtConfig controls integration with the service-mgt registry.
// When Enabled is false the API backend uses static gRPC targets (current behavior).
type ServiceMgtConfig struct {
	Enabled        bool
	RegistryTarget string
}

type GRPCConfig struct {
	// ServerPort is the port the prediction gRPC server listens on
	ServerPort string
	// ClientTarget is the address the API backend uses to connect to the prediction gRPC server
	ClientTarget string
	// AuthClientTarget is the address of the Java Auth Service gRPC server
	AuthClientTarget string
}

type ServerConfig struct {
	ServerName     string
	Host           string
	Port           string
	APIKey         string
	JWTSecret      string
	InternalSecret string
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
