use axum::{
    body::Body,
    extract::ConnectInfo,
    http::{Request, StatusCode},
    middleware::Next,
    response::Response,
};
use governor::{DefaultKeyedRateLimiter, Quota, RateLimiter};
use std::{net::SocketAddr, num::NonZeroU32, sync::Arc, time::Duration};
use tracing::warn;

use crate::config::RateLimitConfig;

/// Shared keyed rate limiter, indexed by client IP.
/// Note: governor's DefaultKeyedRateLimiter does not evict stale keys. At the traffic
/// scale of this project this is an accepted trade-off; add moka or retain_recent()
/// if unique-IP churn becomes a concern.
type IpRateLimiter = Arc<DefaultKeyedRateLimiter<std::net::IpAddr>>;

/// Wraps `router` with a per-IP GCRA rate limiter when `cfg.enabled = true`.
/// `max_requests` tokens are allowed per `window_secs` seconds; burst is capped at
/// `max_requests` (clients that were idle can fire up to that many requests at once).
/// Returns the original router unchanged when disabled.
pub fn apply_rate_limit(
    router: axum::Router,
    cfg: &RateLimitConfig,
) -> axum::Router {
    if !cfg.enabled {
        return router;
    }
    let max = cfg.max_requests.max(1);
    let burst = NonZeroU32::new(max).expect("max(1) ensures non-zero");
    // Replenishment period: window_secs / max_requests (time per token).
    let period_ms = (cfg.window_secs * 1000 / max as u64).max(1);
    let quota = Quota::with_period(Duration::from_millis(period_ms))
        .expect("period is non-zero")
        .allow_burst(burst);
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
    let ip = req
        .extensions()
        .get::<ConnectInfo<SocketAddr>>()
        .map(|ci| ci.0.ip());

    match ip {
        None => {
            // ConnectInfo absent — server was not started with into_make_service_with_connect_info.
            // Allow the request but warn so the misconfiguration is detectable.
            warn!("rate_limit: ConnectInfo absent, skipping rate check — ensure into_make_service_with_connect_info is used");
            next.run(req).await
        }
        Some(addr) => {
            if limiter.check_key(&addr).is_err() {
                return Response::builder()
                    .status(StatusCode::TOO_MANY_REQUESTS)
                    .body(Body::empty())
                    .unwrap();
            }
            next.run(req).await
        }
    }
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
