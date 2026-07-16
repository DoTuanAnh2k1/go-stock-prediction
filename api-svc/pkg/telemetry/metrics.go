package telemetry

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HTTP metrics. Default process/go collectors are registered automatically by
// the client_golang default registry; we add request-level series here.
var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests handled, by method, path and status.",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds, by method and path.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	httpRequestErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_request_errors_total",
			Help: "Total number of HTTP requests that returned a 5xx status.",
		},
		[]string{"method", "path"},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration, httpRequestErrors)
}

// MetricsHandler returns the Prometheus scrape handler for GET /metrics.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// ObserveHTTP records one HTTP request's count, latency and error status. The
// caller supplies the route pattern (not the raw path) so cardinality stays
// bounded.
func ObserveHTTP(method, route string, status int, dur time.Duration) {
	statusStr := strconv.Itoa(status)
	httpRequestsTotal.WithLabelValues(method, route, statusStr).Inc()
	httpRequestDuration.WithLabelValues(method, route).Observe(dur.Seconds())
	if status >= 500 {
		httpRequestErrors.WithLabelValues(method, route).Inc()
	}
}
