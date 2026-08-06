"""Prometheus metrics server for prediction-svc.

Exposes /metrics on :9464 (HTTP) using the prometheus_client library.
Default process & Python metrics are included automatically.

Metrics registered here:
  prediction_grpc_requests_total{method}           — counter
  prediction_grpc_latency_seconds{method}          — histogram
  prediction_grpc_errors_total{method}             — counter
  predictions_total{market,algorithm}              — counter
  crawl_total{market,status}                       — counter  (status: saved|reject)
  crawl_saved_rows_total{market}                   — counter
  reconcile_total{market,verdict}                  — counter  (verdict: correct|wrong|pending)
  direction_accuracy{market,algorithm}             — gauge    (0.0–100.0)
  bot_portfolio_value{market,bot}                  — gauge
  bot_trades_total{market,side}                    — counter  (side: buy|sell)
  training_runs_total{market,algorithm,status}     — counter
  training_duration_seconds{market,algorithm}      — histogram

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

# Business metrics — all None until init_metrics() succeeds
predictions_total = None
crawl_total = None
crawl_saved_rows_total = None
reconcile_total = None
direction_accuracy = None
bot_portfolio_value = None
bot_trades_total = None
training_runs_total = None
training_duration_seconds = None

_server_thread: threading.Thread | None = None


def init_metrics() -> None:
    """Start the Prometheus HTTP server on :9464.  Safe when lib absent."""
    global grpc_requests_total, grpc_latency_seconds, grpc_errors_total, _server_thread
    global predictions_total, crawl_total, crawl_saved_rows_total, reconcile_total
    global direction_accuracy, bot_portfolio_value, bot_trades_total
    global training_runs_total, training_duration_seconds

    try:
        from prometheus_client import (
            Counter,
            Gauge,
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

        # --- Business metrics ---
        predictions_total = Counter(
            "predictions_total",
            "Total prediction rows written per market and algorithm",
            ["market", "algorithm"],
        )
        crawl_total = Counter(
            "crawl_total",
            "Total crawl attempts per market (status: saved or reject)",
            ["market", "status"],
        )
        crawl_saved_rows_total = Counter(
            "crawl_saved_rows_total",
            "Total rows saved to DB by crawlers per market",
            ["market"],
        )
        reconcile_total = Counter(
            "reconcile_total",
            "Total prediction reconcile verdicts (correct|wrong|pending)",
            ["market", "verdict"],
        )
        direction_accuracy = Gauge(
            "direction_accuracy",
            "Rolling direction accuracy (0-100) per market and algorithm",
            ["market", "algorithm"],
        )
        bot_portfolio_value = Gauge(
            "bot_portfolio_value",
            "Current total portfolio value for a simulation bot",
            ["market", "bot"],
        )
        bot_trades_total = Counter(
            "bot_trades_total",
            "Total trades executed by simulation bots per market and side (buy|sell)",
            ["market", "side"],
        )
        training_runs_total = Counter(
            "training_runs_total",
            "Total training runs per market and algorithm (status: success|failed|skipped)",
            ["market", "algorithm", "status"],
        )
        training_duration_seconds = Histogram(
            "training_duration_seconds",
            "Training duration in seconds per market and algorithm",
            ["market", "algorithm"],
            buckets=[1.0, 5.0, 15.0, 30.0, 60.0, 120.0, 300.0, 600.0, 1800.0],
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


# ---------------------------------------------------------------------------
# Business metric helpers — all no-op-safe (None guard + try/except)
# ---------------------------------------------------------------------------

def record_prediction(market: str, algorithm: str, n: int = 1) -> None:
    """Count prediction rows written.  No-op when prometheus_client absent."""
    if predictions_total is not None:
        try:
            predictions_total.labels(market=market, algorithm=algorithm).inc(n)
        except Exception:
            pass


def record_crawl(market: str, status: str) -> None:
    """Count a crawl tick (status: 'saved' or 'reject').  No-op when absent."""
    if crawl_total is not None:
        try:
            crawl_total.labels(market=market, status=status).inc()
        except Exception:
            pass


def record_crawl_saved(market: str, rows: int) -> None:
    """Count rows saved by a crawler run.  No-op when prometheus_client absent."""
    if crawl_saved_rows_total is not None and rows > 0:
        try:
            crawl_saved_rows_total.labels(market=market).inc(rows)
        except Exception:
            pass


# Alias matching the task spec name
record_crawl_rows = record_crawl_saved


def record_reconcile(market: str, verdict: str, n: int = 1) -> None:
    """Count a reconcile verdict (correct|wrong|pending).  No-op when absent."""
    if reconcile_total is not None:
        try:
            reconcile_total.labels(market=market, verdict=verdict).inc(n)
        except Exception:
            pass


def set_direction_accuracy(market: str, algorithm: str, value_float: float) -> None:
    """Set the rolling direction accuracy gauge.  No-op when absent."""
    if direction_accuracy is not None:
        try:
            direction_accuracy.labels(market=market, algorithm=algorithm).set(value_float)
        except Exception:
            pass


def set_bot_portfolio(market: str, bot: str, value_float: float) -> None:
    """Set bot portfolio value gauge.  No-op when prometheus_client absent."""
    if bot_portfolio_value is not None:
        try:
            bot_portfolio_value.labels(market=market, bot=bot).set(value_float)
        except Exception:
            pass


def record_trade(market: str, side: str) -> None:
    """Count a bot trade (side: 'buy' or 'sell').  No-op when absent."""
    if bot_trades_total is not None:
        try:
            bot_trades_total.labels(market=market, side=side).inc()
        except Exception:
            pass


def record_training(market: str, algorithm: str, status: str) -> None:
    """Count a training run completion.  No-op when prometheus_client absent."""
    if training_runs_total is not None:
        try:
            training_runs_total.labels(market=market, algorithm=algorithm, status=status).inc()
        except Exception:
            pass


def observe_training_duration(market: str, algorithm: str, seconds: float) -> None:
    """Record training duration histogram.  No-op when prometheus_client absent."""
    if training_duration_seconds is not None:
        try:
            training_duration_seconds.labels(market=market, algorithm=algorithm).observe(seconds)
        except Exception:
            pass
