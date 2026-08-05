// Command cli-svc is an interactive SSH server: users SSH in with their
// dashboard credentials and get a shell exposing four verbs (get/set/update/
// delete) over the platform API, with per-command RBAC and table output.
package main

import (
	"log"

	"go-stock-prediction/cli-svc/internal/server"
	"go-stock-prediction/cli-svc/internal/telemetry"
)

// metricsAddr is the auxiliary HTTP port that serves GET /metrics for Prometheus
// scraping. cli-svc's main listener is SSH, so metrics live on their own port.
const metricsAddr = ":9464"

func main() {
	// OpenTelemetry tracing (OTLP/gRPC → collector). No-op when
	// OTEL_EXPORTER_OTLP_ENDPOINT is unset (e.g. Docker Compose).
	telemetry.InitTracing("cli-svc")
	defer telemetry.ShutdownTracing()

	// Auxiliary Prometheus metrics endpoint (SSH service → separate HTTP port).
	telemetry.StartMetricsServer(metricsAddr)

	cfg := server.LoadConfig()
	srv := server.New(cfg)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("cli-svc server error: %v", err)
	}
}
