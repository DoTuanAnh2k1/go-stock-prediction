use axum::{
    body::Body,
    extract::State,
    http::{HeaderMap, Method, StatusCode, Uri},
    response::{IntoResponse, Response},
};
use std::sync::Arc;
use tracing::info;

use crate::middleware::add_security_headers;
use crate::proxy::client::ProxyClient;
use crate::router::{PathRouter, RouteAction};

pub struct AppState {
    pub proxy_client: Arc<ProxyClient>,
    pub router: Arc<PathRouter>,
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
    }
}
