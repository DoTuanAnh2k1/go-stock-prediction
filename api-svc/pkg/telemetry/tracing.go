// Package telemetry wires OpenTelemetry tracing and Prometheus metrics for the
// api-svc. Tracing exports spans over OTLP/gRPC to the OpenTelemetry Collector;
// configuration is read from the standard OTEL_* environment variables so the
// same binary behaves correctly in Docker Compose (no collector) and in k8s.
package telemetry

import (
	"context"
	"os"
	"time"

	"go-stock-prediction/pkg/logger"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"
)

// tracerProvider is the process-global provider, kept so it can be flushed and
// shut down cleanly on SIGTERM.
var tracerProvider *sdktrace.TracerProvider

// InitTracing configures the global tracer provider and W3C propagators from the
// standard OTEL_* environment variables. When OTEL_EXPORTER_OTLP_ENDPOINT is
// empty (e.g. Docker Compose without a collector) tracing is left disabled and a
// no-op provider is used, so the service runs unchanged.
//
// serviceName is used only as a fallback when OTEL_SERVICE_NAME is not set.
func InitTracing(serviceName string) {
	// Always install W3C propagators so incoming traceparent headers are honoured
	// even when we are not exporting (harmless no-op otherwise).
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		logger.Logger.Infof("tracing: OTEL_EXPORTER_OTLP_ENDPOINT not set, tracing disabled")
		return
	}

	if v := os.Getenv("OTEL_SERVICE_NAME"); v != "" {
		serviceName = v
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// otlptracegrpc reads OTEL_EXPORTER_OTLP_ENDPOINT / _PROTOCOL itself; the
	// endpoint here (insecure) matches the in-cluster collector over plaintext.
	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure())
	if err != nil {
		logger.Logger.Errorf("tracing: failed to create OTLP exporter: %v", err)
		return
	}

	// The resource picks up OTEL_RESOURCE_ATTRIBUTES automatically via
	// resource.Default(); we merge in the service name as an explicit fallback.
	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(serviceNameAttr(serviceName)...),
	)
	if err != nil {
		logger.Logger.Errorf("tracing: failed to build resource: %v", err)
		res = resource.Default()
	}

	tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		// Sampler is driven by OTEL_TRACES_SAMPLER (parentbased_always_on for the
		// education stack). ParentBased(AlwaysSample) is a safe default.
		sdktrace.WithSampler(samplerFromEnv()),
	)
	otel.SetTracerProvider(tracerProvider)

	logger.Logger.Infof("tracing: OTLP/gRPC exporter initialised (endpoint=%s service=%s)", endpoint, serviceName)
}

// ShutdownTracing flushes and stops the tracer provider. Safe to call when
// tracing was never initialised.
func ShutdownTracing() {
	if tracerProvider == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := tracerProvider.Shutdown(ctx); err != nil {
		logger.Logger.Errorf("tracing: shutdown error: %v", err)
	}
}

// GRPCClientDialOption returns the otelgrpc stats handler that injects the
// current trace context into outbound gRPC calls (auth-svc, prediction-svc), so
// spans chain across services.
func GRPCClientDialOption() grpc.DialOption {
	return grpc.WithStatsHandler(otelgrpc.NewClientHandler())
}
