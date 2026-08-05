// Package telemetry wires OpenTelemetry tracing and a Prometheus metrics
// endpoint for cli-svc. Tracing exports spans over OTLP/gRPC to the
// OpenTelemetry Collector, configured from the standard OTEL_* environment
// variables. When OTEL_EXPORTER_OTLP_ENDPOINT is unset (Docker Compose without a
// collector) tracing is disabled and everything runs unchanged.
package telemetry

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
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
		log.Printf("tracing: OTEL_EXPORTER_OTLP_ENDPOINT not set, tracing disabled")
		return
	}
	if v := os.Getenv("OTEL_SERVICE_NAME"); v != "" {
		serviceName = v
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure())
	if err != nil {
		log.Printf("tracing: failed to create OTLP exporter: %v", err)
		return
	}

	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(semconv.ServiceName(serviceName)),
	)
	if err != nil {
		log.Printf("tracing: failed to build resource: %v", err)
		res = resource.Default()
	}

	tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(samplerFromEnv()),
	)
	otel.SetTracerProvider(tracerProvider)
	log.Printf("tracing: OTLP/gRPC exporter initialised (endpoint=%s service=%s)", endpoint, serviceName)
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
		log.Printf("tracing: shutdown error: %v", err)
	}
}

// NewHTTPTransport wraps base with the otelhttp transport so outbound requests
// from cli-svc to the gateway/API carry the current trace context (traceparent
// header). Pass http.DefaultTransport when base is nil.
func NewHTTPTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return otelhttp.NewTransport(base)
}

// StartMetricsServer serves the Prometheus scrape endpoint (default runtime
// collectors) on addr, e.g. ":9464". Runs in its own goroutine.
func StartMetricsServer(addr string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Printf("metrics: Prometheus endpoint listening on %s/metrics", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("metrics: server error: %v", err)
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
