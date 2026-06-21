package config

import (
	"os"
	"testing"
)

func TestServiceMgtDefaults(t *testing.T) {
	os.Unsetenv("SERVICE_MGT_ENABLED")
	os.Unsetenv("REGISTRY_GRPC_TARGET")
	InitConfig()
	if GetServiceMgtEnabled() {
		t.Fatalf("SERVICE_MGT_ENABLED must default to false")
	}
	if GetRegistryTarget() != "localhost:8121" {
		t.Fatalf("registry target default = %q, want localhost:8121", GetRegistryTarget())
	}
}

func TestServiceMgtEnabledFromEnv(t *testing.T) {
	t.Setenv("SERVICE_MGT_ENABLED", "true")
	t.Setenv("REGISTRY_GRPC_TARGET", "service-mgt:8121")
	InitConfig()
	if !GetServiceMgtEnabled() {
		t.Fatalf("SERVICE_MGT_ENABLED=true must enable")
	}
	if GetRegistryTarget() != "service-mgt:8121" {
		t.Fatalf("registry target = %q, want service-mgt:8121", GetRegistryTarget())
	}
}
