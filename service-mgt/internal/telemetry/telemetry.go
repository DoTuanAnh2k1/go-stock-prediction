// Package telemetry wires OpenTelemetry tracing and a Prometheus metrics
// endpoint for service-mgt. Tracing exports spans over OTLP/gRPC to the
// OpenTelemetry Collector, configured from the standard OTEL_* environment
// variables. When OTEL_EXPORTER_OTLP_ENDPOINT is unset (Docker Compose without a
// collector) tracing is disabled and everything runs unchanged.
package telemetry

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"google.golang.org/grpc"
)

var tracerProvider *sdktrace.TracerProvider

// InitTracing configures the global tracer provider and W3C propagators from the
// OTEL_* environment variables. serviceName is a fallback when OTEL_SERVICE_NAME
// is unset.
func InitTracing(serviceName string) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		log.Info().Msg("tracing: OTEL_EXPORTER_OTLP_ENDPOINT not set, tracing disabled")
		return
	}
	if v := os.Getenv("OTEL_SERVICE_NAME"); v != "" {
		serviceName = v
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure())
	if err != nil {
		log.Error().Err(err).Msg("tracing: failed to create OTLP exporter")
		return
	}

	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(semconv.ServiceName(serviceName)),
	)
	if err != nil {
		log.Error().Err(err).Msg("tracing: failed to build resource")
		res = resource.Default()
	}

	tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(samplerFromEnv()),
	)
	otel.SetTracerProvider(tracerProvider)
	log.Info().Str("endpoint", endpoint).Str("service", serviceName).Msg("tracing: OTLP/gRPC exporter initialised")
}

// ShutdownTracing flushes and stops the tracer provider. Safe when tracing was
// never initialised.
func ShutdownTracing() {
	if tracerProvider == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := tracerProvider.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("tracing: shutdown error")
	}
}

// GRPCServerOption returns the otelgrpc stats-handler server option. It extracts
// the incoming W3C trace context from gRPC metadata and starts a server span for
// every RPC (register/heartbeat/discover).
func GRPCServerOption() grpc.ServerOption {
	return grpc.StatsHandler(otelgrpc.NewServerHandler())
}

// StartMetricsServer serves the Prometheus scrape endpoint (default runtime
// collectors) on addr, e.g. ":9464". Runs in its own goroutine; blocking errors
// are logged, not fatal.
func StartMetricsServer(addr string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Info().Str("addr", addr).Msg("metrics: Prometheus endpoint listening on /metrics")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("metrics: server error")
		}
	}()
}

func samplerFromEnv() sdktrace.Sampler {
	switch strings.TrimSpace(strings.ToLower(os.Getenv("OTEL_TRACES_SAMPLER"))) {
	case "always_off":
		return sdktrace.NeverSample()
	case "always_on":
		return sdktrace.AlwaysSample()
	case "parentbased_always_off":
		return sdktrace.ParentBased(sdktrace.NeverSample())
	default:
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	}
}
