use axum::{
    body::Body,
    extract::State,
    http::{HeaderMap, Method, StatusCode, Uri},
    response::{IntoResponse, Response},
};
use std::sync::Arc;
use std::time::Instant;
use tracing::{info, info_span, Instrument};

use crate::bluegreen::state::{is_failure, BlueGreenState};
use crate::middleware::add_security_headers;
use crate::proxy::client::ProxyClient;
use crate::router::{PathRouter, RouteAction};
use crate::telemetry::metrics::record_request;

pub struct AppState {
    pub proxy_client: Arc<ProxyClient>,
    pub router: Arc<PathRouter>,
    pub bg: Option<Arc<BlueGreenState>>,
}

/// Catch-all handler: routes by path prefix, applies security headers when configured.
///
/// EDGE span: a root `http.request` span is opened here so the gateway mints the
/// trace and the outbound call (proxy/client.rs) can inject `traceparent`. The
/// forwarding runs `.instrument(span)` so `Span::current()` is set for it.
pub async fn proxy_handler(
    State(state): State<Arc<AppState>>,
    method: Method,
    uri: Uri,
    headers: HeaderMap,
    body: Body,
) -> Response {
    let span = info_span!(
        "http.request",
        otel.name = %format!("{} {}", method, uri.path()),
        http.request.method = %method,
        url.path = %uri.path(),
        http.response.status_code = tracing::field::Empty,
    );
    let started = Instant::now();
    let method_label = method.as_str().to_string();

    let (route_label, response) = route_and_forward(state, method, uri, headers, body)
        .instrument(span.clone())
        .await;

    let status = response.status().as_u16();
    span.record("http.response.status_code", status);
    record_request(&route_label, &method_label, status, started.elapsed());
    response
}

/// Inner routing + forwarding, kept separate so `proxy_handler` can wrap it in
/// the edge span and record metrics around it. Returns `(route_label, response)`.
async fn route_and_forward(
    state: Arc<AppState>,
    method: Method,
    uri: Uri,
    headers: HeaderMap,
    body: Body,
) -> (String, Response) {
    let path = uri.path();
    let query = uri.query().unwrap_or("");
    let full_path = if query.is_empty() {
        path.to_string()
    } else {
        format!("{}?{}", path, query)
    };

    info!(method = %method, path = %full_path, "Routing request");

    let action = state.router.route(path);
    let route_label = route_label_for(&action);

    let response = match action {
        None | Some(RouteAction::Block) => {
            info!(path = %path, "Blocked request, returning 404");
            StatusCode::NOT_FOUND.into_response()
        }
        Some(RouteAction::Proxy {
            backend,
            security_headers,
        }) => {
            match state
                .proxy_client
                .forward_request(method, &full_path, &headers, body, backend)
                .await
            {
                Ok(mut response) => {
                    if *security_headers {
                        add_security_headers(response.headers_mut());
                    }
                    response
                }
                Err(e) => e.into_response(),
            }
        }
        Some(RouteAction::BlueGreen { app_id }) => {
            let bg = match &state.bg {
                Some(b) => b,
                None => return (route_label, StatusCode::BAD_GATEWAY.into_response()),
            };
            let app_id = *app_id;
            let color = bg.active(app_id);
            let backend = bg.backend(app_id);
            // failure_status_from is fixed at 500 on the data path; the controller
            // reads the configurable value for its verdict.
            const FAIL_FROM: u16 = 500;
            match state
                .proxy_client
                .forward_request(method, &full_path, &headers, body, &backend)
                .await
            {
                Ok(response) => {
                    let status = response.status().as_u16();
                    bg.record(app_id, color, !is_failure(Some(status), false, FAIL_FROM));
                    response
                }
                Err(e) => {
                    bg.record(app_id, color, !is_failure(None, true, FAIL_FROM));
                    e.into_response()
                }
            }
        }
    };

    (route_label, response)
}

/// Stable, low-cardinality label for metrics keyed on the routing decision.
fn route_label_for(action: &Option<&RouteAction>) -> String {
    match action {
        None => "unmatched".to_string(),
        Some(RouteAction::Block) => "blocked".to_string(),
        Some(RouteAction::Proxy { backend, .. }) => backend.clone(),
        Some(RouteAction::BlueGreen { app_id }) => format!("bluegreen:{app_id}"),
    }
}
