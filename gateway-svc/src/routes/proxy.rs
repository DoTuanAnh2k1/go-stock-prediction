use axum::{
    body::Body,
    extract::State,
    http::{HeaderMap, Method, StatusCode, Uri},
    response::{IntoResponse, Response},
};
use std::sync::Arc;
use tracing::info;

use crate::bluegreen::state::{is_failure, BlueGreenState};
use crate::middleware::add_security_headers;
use crate::proxy::client::ProxyClient;
use crate::router::{PathRouter, RouteAction};

pub struct AppState {
    pub proxy_client: Arc<ProxyClient>,
    pub router: Arc<PathRouter>,
    pub bg: Option<Arc<BlueGreenState>>,
}

/// Catch-all handler: routes by path prefix, applies security headers when configured.
pub async fn proxy_handler(
    State(state): State<Arc<AppState>>,
    method: Method,
    uri: Uri,
    headers: HeaderMap,
    body: Body,
) -> Response {
    let path = uri.path();
    let query = uri.query().unwrap_or("");
    let full_path = if query.is_empty() {
        path.to_string()
    } else {
        format!("{}?{}", path, query)
    };

    info!(method = %method, path = %full_path, "Routing request");

    match state.router.route(path) {
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
                None => return StatusCode::BAD_GATEWAY.into_response(),
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
    }
}
