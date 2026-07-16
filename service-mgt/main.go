// Command service-mgt is the central service registry: services register on
// boot, renew a lease via Heartbeat, and resolve peers via Discover. Postgres
// is the source of truth; an in-memory cache is the read layer (write-through).
package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"go-stock-prediction/service-mgt/internal/config"
	"go-stock-prediction/service-mgt/internal/grpcserver"
	"go-stock-prediction/service-mgt/internal/registry"
	"go-stock-prediction/service-mgt/internal/store"
	"go-stock-prediction/service-mgt/internal/telemetry"
	registrypb "go-stock-prediction/service-mgt/proto/registry"
)

// metricsAddr is the auxiliary HTTP port that serves GET /metrics for Prometheus
// scraping. service-mgt is gRPC-only, so metrics live on their own port.
const metricsAddr = ":9464"

func main() {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		panic(fmt.Sprintf("load tz: %v", err))
	}
	time.Local = loc

	zerolog.TimeFieldFormat = time.RFC3339
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, NoColor: false}).With().Timestamp().Logger()
	log.Logger = logger

	cfg := config.Load()

	// OpenTelemetry tracing (OTLP/gRPC → collector). No-op when
	// OTEL_EXPORTER_OTLP_ENDPOINT is unset (e.g. Docker Compose).
	telemetry.InitTracing("service-mgt")

	// Auxiliary Prometheus metrics endpoint (gRPC-only service → separate port).
	telemetry.StartMetricsServer(metricsAddr)

	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{})
	if err != nil {
		logger.Fatal().Err(err).Msg("service-mgt: db connect failed")
	}
	st := store.NewGorm(db)

	core := registry.New(st, registry.Config{
		DefaultTTL:    time.Duration(cfg.DefaultTTLSeconds) * time.Second,
		EvictGrace:    time.Duration(cfg.EvictGraceSeconds) * time.Second,
		FlushInterval: time.Duration(cfg.FlushIntervalSeconds) * time.Second,
		ReaperTick:    time.Second,
	})
	if err := core.WarmStart(); err != nil {
		logger.Warn().Err(err).Msg("service-mgt: warm-start failed")
	}
	stop := make(chan struct{})
	core.StartBackground(stop)

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		logger.Fatal().Err(err).Msgf("service-mgt: listen :%s failed", cfg.GRPCPort)
	}
	// otelgrpc server handler extracts the incoming trace context from RPC
	// metadata and starts a server span per RPC.
	grpcSrv := grpc.NewServer(telemetry.GRPCServerOption())
	registrypb.RegisterRegistryServer(grpcSrv, grpcserver.New(core))

	// Standard gRPC health service so the k8s native `grpc:` probe works.
	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(grpcSrv, healthSrv)
	go func() {
		logger.Info().Msgf("service-mgt: gRPC registry listening on :%s", cfg.GRPCPort)
		if err := grpcSrv.Serve(lis); err != nil {
			logger.Fatal().Err(err).Msg("service-mgt: serve failed")
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)
	s := <-sig
	logger.Info().Msgf("service-mgt: received %v, shutting down", s)
	close(stop)
	grpcSrv.GracefulStop()
	telemetry.ShutdownTracing()
	logger.Info().Msg("service-mgt: stopped")
}
