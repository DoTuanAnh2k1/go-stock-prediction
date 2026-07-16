"""Prometheus metrics server for prediction-svc.

Exposes /metrics on :9464 (HTTP) using the prometheus_client library.
Default process & Python metrics are included automatically.

Metrics registered here:
  prediction_grpc_requests_total{method}    — counter
  prediction_grpc_latency_seconds{method}   — histogram
  prediction_grpc_errors_total{method}      — counter

Usage:
    from src.telemetry.metrics import init_metrics, stop_metrics
    init_metrics()      # call once at startup
    stop_metrics()      # call at SIGTERM (stops HTTP server thread)
"""
from __future__ import annotations

import threading
from src.utils.logger import get_logger

log = get_logger("telemetry.metrics")

METRICS_PORT = 9464

# Module-level metric objects (None when prometheus_client is absent)
grpc_requests_total = None
grpc_latency_seconds = None
grpc_errors_total = None

_server_thread: threading.Thread | None = None


def init_metrics() -> None:
    """Start the Prometheus HTTP server on :9464.  Safe when lib absent."""
    global grpc_requests_total, grpc_latency_seconds, grpc_errors_total, _server_thread

    try:
        from prometheus_client import (
            Counter,
            Histogram,
            start_http_server,
            REGISTRY,
        )
    except ImportError:
        log.warning("telemetry.metrics.disabled", reason="prometheus_client not installed")
        return

    # ----------------------------------------------------------------
    # Define metrics
    # ----------------------------------------------------------------
    try:
        grpc_requests_total = Counter(
            "prediction_grpc_requests_total",
            "Total gRPC requests handled by prediction-svc",
            ["method"],
        )
        grpc_latency_seconds = Histogram(
            "prediction_grpc_latency_seconds",
            "gRPC request latency in seconds",
            ["method"],
            buckets=[0.01, 0.05, 0.1, 0.5, 1.0, 5.0, 30.0, 60.0],
        )
        grpc_errors_total = Counter(
            "prediction_grpc_errors_total",
            "Total gRPC errors returned by prediction-svc",
            ["method"],
        )
    except Exception as exc:
        # Metrics may already be registered (e.g. during test re-imports)
        log.warning("telemetry.metrics.register.error", error=str(exc))

    # ----------------------------------------------------------------
    # Start HTTP server in a daemon thread
    # ----------------------------------------------------------------
    try:
        start_http_server(METRICS_PORT)
        log.info("telemetry.metrics.ready", port=METRICS_PORT)
    except Exception as exc:
        log.warning("telemetry.metrics.server.error", error=str(exc))


def stop_metrics() -> None:
    """Placeholder — prometheus_client HTTP server is daemonised and needs no
    explicit stop (process exit shuts it down cleanly)."""
    log.info("telemetry.metrics.stopped")


def record_rpc(method: str, latency_s: float, error: bool = False) -> None:
    """Record one completed gRPC call.  No-op when prometheus_client absent."""
    if grpc_requests_total is not None:
        try:
            grpc_requests_total.labels(method=method).inc()
        except Exception:
            pass
    if grpc_latency_seconds is not None:
        try:
            grpc_latency_seconds.labels(method=method).observe(latency_s)
        except Exception:
            pass
    if error and grpc_errors_total is not None:
        try:
            grpc_errors_total.labels(method=method).inc()
        except Exception:
            pass
