use axum::{middleware, routing::get, Router};
use std::sync::Arc;

use crate::config::AppConfig;
use crate::middleware::{logging::logging_middleware, request_id::request_id_middleware};
use crate::proxy::ProxyClient;
use crate::router::PathRouter;
use crate::routes::{
    health::{health_check, readiness_check},
    proxy::AppState,
};

pub mod health;
pub mod proxy;

pub fn create_routes(
    proxy_client: Arc<ProxyClient>,
    router: Arc<PathRouter>,
    _config: Arc<AppConfig>,
) -> Router {
    let state = Arc::new(AppState {
        proxy_client,
        router,
    });

    // /healthz + /readyz serve the gateway's own status (not proxied to any backend)
    let health_routes = Router::new()
        .route("/healthz", get(health_check))
        .route("/readyz", get(readiness_check));

    // Everything else is routed by PathRouter
    let proxy_routes = Router::new()
        .fallback(proxy::proxy_handler)
        .with_state(state);

    Router::new()
        .merge(health_routes)
        .merge(proxy_routes)
        .layer(middleware::from_fn(logging_middleware))
        .layer(middleware::from_fn(request_id_middleware))
}
