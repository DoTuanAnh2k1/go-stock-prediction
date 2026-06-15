# Design: Move Rate Limiting to Gateway

**Date:** 2026-06-15
**Status:** Approved

## Summary

Move rate limiting from the Go API Backend middleware chain to the Rust Gateway service. Replace the two separate limiters (global + login-specific) with a single global per-IP limiter using the `governor` crate (GCRA algorithm) via `tower_governor`.

## Architecture

**Before:**
```
Client → Gateway (Rust, no rate limit) → API (Go, RateLimit + LoginRateLimit → JWT → mux)
```

**After:**
```
Client → Gateway (Rust, GovernorLayer per-IP) → API (Go, JWT → mux)
```

Rate limiting happens at the edge, before traffic enters the backend. The gateway is the only public entry point, so all traffic — including direct `/api/*` and frontend `/` routes — passes through the limiter.

## Gateway Changes

### 1. Dependencies (`gateway-svc/Cargo.toml`)

Add:
```toml
governor = "0.6"
tower_governor = "0.4"
```

### 2. Config (`gateway-svc/config.yaml`)

Update `middleware.rate_limit` block:
```yaml
middleware:
  rate_limit:
    enabled: true
    max_requests: 60   # requests per window
    window_secs: 1     # window size in seconds → 60 req/s per IP
```

This matches the Go API's current global limit (60 req/s, burst handled by GCRA smoothing).

### 3. New middleware (`gateway-svc/src/middleware/rate_limit.rs`)

Build a `GovernorLayer` from `RateLimitConfig`. Use `tower_governor::PeerIpKeyExtractor` — the gateway is the edge so `RemoteAddr` is the real client IP, no `X-Forwarded-For` parsing needed.

`PeerIpKeyExtractor` requires `ConnectInfo<SocketAddr>` to be injected into request extensions. This is done by switching `into_make_service()` → `into_make_service_with_connect_info::<SocketAddr>()` in `main.rs` (both HTTP and HTTPS paths — see section 5 below).

```
pub fn build_rate_limit_layer(cfg: &RateLimitConfig) -> Option<GovernorLayer<...>>
```

Returns `None` when `enabled = false` so the layer is a no-op in development.

Error response on 429: `tower_governor`'s default `429 Too Many Requests` plain-text response. The frontend checks HTTP status code, not body — this is sufficient.

### 4. Route wiring (`gateway-svc/src/routes/mod.rs`)

Apply the layer in `create_routes()` when enabled:

```rust
if let Some(rl_layer) = build_rate_limit_layer(&config.middleware.rate_limit) {
    router = router.layer(rl_layer);
}
```

The layer wraps all routes including `/healthz` and `/readyz`. This is acceptable — health checks are internal Docker probes and hit limits only if something is misconfigured.

### 5. Export (`gateway-svc/src/middleware/mod.rs`)

Export `rate_limit` module alongside existing `logging`, `request_id`, `security_headers`.

### 6. `main.rs` — ConnectInfo wiring

Both serve paths must switch to `into_make_service_with_connect_info::<SocketAddr>()`:

```rust
// TLS path (axum-server):
axum_server::bind(http_addr)
    .serve(app.clone().into_make_service_with_connect_info::<SocketAddr>())

axum_server::bind_rustls(https_addr, tls_config)
    .serve(app.into_make_service_with_connect_info::<SocketAddr>())

// HTTP-only path (axum::serve):
axum::serve(listener, app.into_make_service_with_connect_info::<SocketAddr>())
```

This is safe even when rate limiting is disabled — `ConnectInfo` is a zero-cost extension and does not affect routing.

## API Changes

### 1. Delete `api/pkg/server/middleware_ratelimit.go`

The entire file is removed — both `RateLimitMiddleware` and `LoginRateLimitMiddleware` and their associated `ipRateLimiter` type.

### 2. Update `api/pkg/server/router.go`

- Remove `RateLimitMiddleware` from the global middleware chain
- Remove `LoginRateLimitMiddleware` wrapper from the login route

The login route no longer has a separate 5 req/min limit. The global 60 req/s gateway limit is the only protection. This is acceptable: brute-force login attempts at 60/s would require 60 credential pairs per second which is still impractical, and the Java Auth Service's bcrypt hashing naturally throttles server-side.

## What Does NOT Change

- JWT middleware in API — unchanged
- AdminRequired / MarketRequired — unchanged
- Gateway proxy logic, TLS, security headers — unchanged
- Frontend — unchanged (same 429 JSON format)
- Login endpoint path/behavior — unchanged (just loses its own sub-limiter)

## Testing

- Build gateway: `docker-compose build gateway`
- Verify 429 fires: `for i in $(seq 1 200); do curl -s -o /dev/null -w "%{http_code}\n" http://localhost/api/health; done`
- Verify API still works normally under limit: single requests return 200
- Verify `enabled: false` in config skips the layer entirely
