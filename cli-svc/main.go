// Command cli-svc is an interactive SSH server: users SSH in with their
// dashboard credentials and get a shell exposing four verbs (get/set/update/
// delete) over the platform API, with per-command RBAC and table output.
package main

import (
	"context"
	"log"

	"go-stock-prediction/cli-svc/internal/server"
	regclient "go-stock-prediction/service-mgt/client"
)

func main() {
	cfg := server.LoadConfig()

	// Service registry — register cli-svc and resolve the gateway endpoint.
	// No-op when SERVICE_MGT_ENABLED=false (static API_BASE_URL is used).
	reg := regclient.New(regclient.Options{
		Enabled:        cfg.ServiceMgtEnabled,
		RegistryTarget: cfg.RegistryTarget,
		ServiceName:    "cli-svc",
		Address:        "cli-svc",
		Port:           2345,
	})
	if err := reg.Start(context.Background()); err != nil {
		log.Printf("cli-svc service registry start failed, using static API base: %v", err)
	}
	defer reg.Stop()

	// Resolve the gateway via the registry; fall back to the static base URL.
	if gw := reg.Resolve("gateway-svc", ""); gw != "" {
		cfg.APIBaseURL = "http://" + gw + "/api"
		log.Printf("cli-svc resolved gateway via registry: API=%s", cfg.APIBaseURL)
	}

	srv := server.New(cfg)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("cli-svc server error: %v", err)
	}
}
