use axum::{extract::Request, middleware::Next, response::Response};
use std::time::Instant;
use tracing::{info, warn};

use crate::middleware::request_id::RequestIdExt;

pub async fn logging_middleware(request: Request, next: Next) -> Response {
    let start = Instant::now();
    let method = request.method().clone();
    let uri = request.uri().clone();
    let request_id = request
        .request_id()
        .unwrap_or_else(|| "unknown".to_string());

    let client_ip = request
        .headers()
        .get("x-forwarded-for")
        .and_then(|v| v.to_str().ok())
        .map(|s| s.split(',').next().unwrap_or("unknown").trim())
        .unwrap_or("unknown");

    info!(
        request_id = %request_id,
        method = %method,
        uri = %uri,
        client_ip = %client_ip,
        "Incoming request"
    );

    let response = next.run(request).await;
    let duration = start.elapsed();
    let status = response.status();

    if status.is_success() || status.is_redirection() {
        info!(
            request_id = %request_id,
            status = %status,
            duration_ms = %duration.as_millis(),
            "Request completed"
        );
    } else {
        warn!(
            request_id = %request_id,
            status = %status,
            duration_ms = %duration.as_millis(),
            "Request completed with error"
        );
    }

    response
}
