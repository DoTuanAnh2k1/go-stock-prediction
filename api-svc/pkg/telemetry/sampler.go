package telemetry

import (
	"os"
	"strings"

	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// serviceNameAttr returns the service.name resource attribute for the given
// name. Kept as a slice so callers can spread it into resource.WithAttributes.
func serviceNameAttr(name string) []attribute.KeyValue {
	return []attribute.KeyValue{semconv.ServiceName(name)}
}

// samplerFromEnv maps OTEL_TRACES_SAMPLER to an SDK sampler. Only the values the
// stack uses are handled explicitly; anything else falls back to
// parent-based-always-on (trace everything, respecting upstream decisions).
func samplerFromEnv() sdktrace.Sampler {
	switch strings.TrimSpace(strings.ToLower(os.Getenv("OTEL_TRACES_SAMPLER"))) {
	case "always_off":
		return sdktrace.NeverSample()
	case "always_on":
		return sdktrace.AlwaysSample()
	case "parentbased_always_off":
		return sdktrace.ParentBased(sdktrace.NeverSample())
	default: // parentbased_always_on and unknown
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	}
}
