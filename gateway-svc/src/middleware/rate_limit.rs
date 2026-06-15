use axum::{
    body::Body,
    extract::ConnectInfo,
    http::{Request, StatusCode},
    middleware::Next,
    response::Response,
};
use governor::{DefaultKeyedRateLimiter, Quota, RateLimiter};
use std::{net::SocketAddr, num::NonZeroU32, sync::Arc};

use crate::config::RateLimitConfig;

/// Shared keyed rate limiter, indexed by client IP.
type IpRateLimiter = Arc<DefaultKeyedRateLimiter<std::net::IpAddr>>;

/// Build a per-IP rate limiter from config and return an axum layer.
/// When `cfg.enabled` is false the router is returned unchanged.
pub fn apply_rate_limit(
    router: axum::Router,
    cfg: &RateLimitConfig,
) -> axum::Router {
    if !cfg.enabled {
        return router;
    }
    let rps = NonZeroU32::new(cfg.max_requests.max(1))
        .expect("max(1) ensures non-zero");
    let quota = Quota::per_second(rps).allow_burst(rps);
    let limiter: IpRateLimiter = Arc::new(RateLimiter::keyed(quota));

    router.layer(axum::middleware::from_fn_with_state(
        limiter,
        rate_limit_middleware,
    ))
}

async fn rate_limit_middleware(
    axum::extract::State(limiter): axum::extract::State<IpRateLimiter>,
    req: Request<Body>,
    next: Next,
) -> Response {
    // Extract peer IP from ConnectInfo extension; fall back to allowing if absent.
    let ip = req
        .extensions()
        .get::<ConnectInfo<SocketAddr>>()
        .map(|ci| ci.0.ip());

    if let Some(addr) = ip {
        if limiter.check_key(&addr).is_err() {
            return Response::builder()
                .status(StatusCode::TOO_MANY_REQUESTS)
                .body(Body::empty())
                .unwrap();
        }
    }

    next.run(req).await
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::RateLimitConfig;

    fn disabled_cfg() -> RateLimitConfig {
        RateLimitConfig { enabled: false, max_requests: 60, window_secs: 1 }
    }

    fn enabled_cfg() -> RateLimitConfig {
        RateLimitConfig { enabled: true, max_requests: 60, window_secs: 1 }
    }

    #[test]
    fn disabled_config_builds_without_panic() {
        let router = axum::Router::new();
        let _ = apply_rate_limit(router, &disabled_cfg());
    }

    #[test]
    fn enabled_config_builds_without_panic() {
        let router = axum::Router::new();
        let _ = apply_rate_limit(router, &enabled_cfg());
    }

    #[test]
    fn zero_max_requests_does_not_panic() {
        let cfg = RateLimitConfig { enabled: true, max_requests: 0, window_secs: 1 };
        let router = axum::Router::new();
        let _ = apply_rate_limit(router, &cfg);
    }
}
