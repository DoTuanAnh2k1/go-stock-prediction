package server

import "os"

// Config holds cli-svc runtime configuration loaded from env.
type Config struct {
	APIBaseURL     string // e.g. http://gateway-svc/api
	SSHListenAddr  string // e.g. :2345
	InternalSecret string // shared secret for the handler-catalog upsert
	HostKeyPath    string // path to the SSH host key (created if missing)

	// Service registry (service-mgt) — gated by SERVICE_MGT_ENABLED. When off,
	// cli-svc uses the static APIBaseURL (current behavior).
	ServiceMgtEnabled bool
	RegistryTarget    string // e.g. service-mgt:8121
}

// LoadConfig reads configuration from the environment with sensible defaults.
func LoadConfig() Config {
	return Config{
		APIBaseURL:        getenv("API_BASE_URL", "http://gateway-svc/api"),
		SSHListenAddr:     getenv("SSH_LISTEN_ADDR", ":2345"),
		InternalSecret:    os.Getenv("INTERNAL_SECRET"),
		HostKeyPath:       getenv("SSH_HOST_KEY_PATH", "/etc/cli-svc/keys/host_key"),
		ServiceMgtEnabled: getenv("SERVICE_MGT_ENABLED", "false") == "true",
		RegistryTarget:    getenv("REGISTRY_GRPC_TARGET", "service-mgt:8121"),
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
