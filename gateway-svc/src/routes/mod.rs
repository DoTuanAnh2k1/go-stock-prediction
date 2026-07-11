use axum::{middleware, routing::get, Router};
use std::sync::Arc;

use crate::config::AppConfig;
use crate::middleware::{
    logging::logging_middleware,
    rate_limit::apply_rate_limit,
    request_id::request_id_middleware,
};
use crate::bluegreen::state::BlueGreenState;
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
    config: Arc<AppConfig>,
    bg: Option<Arc<BlueGreenState>>,
) -> Router {
    let state = Arc::new(AppState {
        proxy_client,
        router,
        bg,
    });

    let health_routes = Router::new()
        .route("/healthz", get(health_check))
        .route("/readyz", get(readiness_check));

    let proxy_routes = Router::new()
        .fallback(proxy::proxy_handler)
        .with_state(state);

    let router = Router::new()
        .merge(health_routes)
        .merge(proxy_routes)
        .layer(middleware::from_fn(logging_middleware))
        .layer(middleware::from_fn(request_id_middleware));

    apply_rate_limit(router, &config.middleware.rate_limit)
}
