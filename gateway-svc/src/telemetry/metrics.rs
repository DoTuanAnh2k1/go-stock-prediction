//! Prometheus metrics for the gateway (factor 14).
//!
//! A process-global Prometheus recorder is installed at boot. The rendered
//! text is served by an axum handler on the **admin port :9100** (`/metrics`),
//! which is a separate listener from the public proxy — it is never routed
//! through the proxy table and never reachable from :80/:443.

use std::sync::OnceLock;
use std::time::Duration;

use axum::http::StatusCode;
use axum::response::IntoResponse;
use metrics_exporter_prometheus::{PrometheusBuilder, PrometheusHandle};

/// Handle used to render the current metric snapshot as Prometheus text.
static PROM_HANDLE: OnceLock<PrometheusHandle> = OnceLock::new();

const M_REQUESTS_TOTAL: &str = "gateway_requests_total";
const M_REQUEST_DURATION: &str = "gateway_request_duration_seconds";
const M_ERRORS_TOTAL: &str = "gateway_request_errors_total";

/// Install the global Prometheus recorder. Idempotent-ish: a second install
/// fails inside the metrics crate and is logged, not fatal.
pub fn init() {
    let builder = PrometheusBuilder::new().set_buckets_for_metric(
        metrics_exporter_prometheus::Matcher::Full(M_REQUEST_DURATION.to_string()),
        &[
            0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0,
        ],
    );

    let builder = match builder {
        Ok(b) => b,
        Err(e) => {
            tracing::warn!(error = %e, "metrics: bucket config failed");
            PrometheusBuilder::new()
        }
    };

    let recorder = builder.build_recorder();
    let handle = recorder.handle();
    if metrics::set_global_recorder(recorder).is_err() {
        tracing::warn!("metrics: global recorder already set");
    }
    let _ = PROM_HANDLE.set(handle);
    tracing::info!("metrics: Prometheus recorder installed");
}

/// Record one proxied request's outcome. `route` is the destination label
/// (backend / route action), `status` the numeric HTTP status.
pub fn record_request(route: &str, method: &str, status: u16, elapsed: Duration) {
    let status_class = format!("{}xx", status / 100);
    metrics::counter!(
        M_REQUESTS_TOTAL,
        "route" => route.to_string(),
        "method" => method.to_string(),
        "status" => status.to_string(),
        "status_class" => status_class.clone(),
    )
    .increment(1);

    metrics::histogram!(
        M_REQUEST_DURATION,
        "route" => route.to_string(),
        "method" => method.to_string(),
    )
    .record(elapsed.as_secs_f64());

    if status >= 500 {
        metrics::counter!(
            M_ERRORS_TOTAL,
            "route" => route.to_string(),
            "method" => method.to_string(),
            "status" => status.to_string(),
        )
        .increment(1);
    }
}

/// axum handler for `GET /metrics` (admin port). Returns Prometheus text.
pub async fn metrics_handler() -> impl IntoResponse {
    match PROM_HANDLE.get() {
        Some(h) => (
            StatusCode::OK,
            [("content-type", "text/plain; version=0.0.4")],
            h.render(),
        )
            .into_response(),
        None => (
            StatusCode::SERVICE_UNAVAILABLE,
            "metrics recorder not initialized",
        )
            .into_response(),
    }
}
