# Rate Limiting Migration: API → Gateway Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move rate limiting from the Go API middleware chain to the Rust gateway using `governor`/`tower_governor`, replacing two separate limiters with one global per-IP limiter at the edge.

**Architecture:** A new `apply_rate_limit(router, cfg)` function in the gateway middleware wraps the Axum router with a `GovernorLayer<PeerIpKeyExtractor>` when `enabled = true`. The Go API's `server.go` and `router.go` are stripped of both rate limit references; `middleware_ratelimit.go` is deleted.

**Tech Stack:** Rust/Axum 0.8, `governor = "0.6"`, `tower_governor = "0.4"` (or latest axum-0.8-compatible), `golang.org/x/time/rate` removed from Go.

---

### Task 1: Add governor crates to gateway

**Files:**
- Modify: `gateway-svc/Cargo.toml`

- [ ] **Step 1: Add dependencies**

Open `gateway-svc/Cargo.toml`. In the `[dependencies]` section, add after the `uuid` line:

```toml
governor = "0.6"
tower_governor = "0.4"
```

- [ ] **Step 2: Verify crates resolve**

```bash
cd gateway-svc && cargo check 2>&1 | head -40
```

Expected: crates download and resolve without error. If you see a body-type incompatibility error mentioning `http_body`, replace `tower_governor = "0.4"` with the latest version on crates.io that supports axum 0.8 (check https://crates.io/crates/tower_governor/versions). Re-run `cargo check`.

- [ ] **Step 3: Commit**

```bash
git add gateway-svc/Cargo.toml gateway-svc/Cargo.lock
git commit -m "chore(gateway): add governor + tower_governor deps"
```

---

### Task 2: Implement rate_limit middleware

**Files:**
- Create: `gateway-svc/src/middleware/rate_limit.rs`

- [ ] **Step 1: Write the tests first**

Create `gateway-svc/src/middleware/rate_limit.rs` with tests only:

```rust
use axum::Router;
use std::sync::Arc;
use tower_governor::{
    governor::GovernorConfigBuilder,
    key_extractor::PeerIpKeyExtractor,
    GovernorLayer,
};

use crate::config::RateLimitConfig;

pub fn apply_rate_limit(router: Router, cfg: &RateLimitConfig) -> Router {
    todo!()
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::RateLimitConfig;

    fn disabled_cfg() -> RateLimitConfig {
        RateLimitConfig {
            enabled: false,
            max_requests: 60,
            window_secs: 1,
        }
    }

    fn enabled_cfg() -> RateLimitConfig {
        RateLimitConfig {
            enabled: true,
            max_requests: 60,
            window_secs: 1,
        }
    }

    #[test]
    fn disabled_config_builds_without_panic() {
        let router = Router::new();
        let _ = apply_rate_limit(router, &disabled_cfg());
    }

    #[test]
    fn enabled_config_builds_without_panic() {
        let router = Router::new();
        let _ = apply_rate_limit(router, &enabled_cfg());
    }

    #[test]
    fn zero_max_requests_does_not_panic() {
        let cfg = RateLimitConfig {
            enabled: true,
            max_requests: 0,
            window_secs: 1,
        };
        let router = Router::new();
        let _ = apply_rate_limit(router, &cfg);
    }
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd gateway-svc && cargo test middleware::rate_limit 2>&1 | tail -20
```

Expected: compile error (todo! or function not implemented).

- [ ] **Step 3: Implement apply_rate_limit**

Replace the `apply_rate_limit` function body in `gateway-svc/src/middleware/rate_limit.rs`:

```rust
use axum::Router;
use std::sync::Arc;
use tower_governor::{
    governor::GovernorConfigBuilder,
    key_extractor::PeerIpKeyExtractor,
    GovernorLayer,
};

use crate::config::RateLimitConfig;

/// Wraps `router` with a per-IP rate limiter when `cfg.enabled = true`.
/// `max_requests` is the burst and replenishment ceiling per second.
/// Returns the original router unchanged when disabled.
pub fn apply_rate_limit(router: Router, cfg: &RateLimitConfig) -> Router {
    if !cfg.enabled {
        return router;
    }
    let rps = cfg.max_requests.max(1);
    let config = Arc::new(
        GovernorConfigBuilder::default()
            .per_second(rps)
            .burst_size(rps)
            .key_extractor(PeerIpKeyExtractor)
            .finish()
            .expect("quota is set — per_second was called"),
    );
    router.layer(GovernorLayer { config })
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
        let router = Router::new();
        let _ = apply_rate_limit(router, &disabled_cfg());
    }

    #[test]
    fn enabled_config_builds_without_panic() {
        let router = Router::new();
        let _ = apply_rate_limit(router, &enabled_cfg());
    }

    #[test]
    fn zero_max_requests_does_not_panic() {
        let cfg = RateLimitConfig { enabled: true, max_requests: 0, window_secs: 1 };
        let router = Router::new();
        let _ = apply_rate_limit(router, &cfg);
    }
}
```

- [ ] **Step 4: Run tests**

```bash
cd gateway-svc && cargo test middleware::rate_limit 2>&1
```

Expected: `test middleware::rate_limit::tests::disabled_config_builds_without_panic ... ok`, and the other two tests also `ok`. Zero failures.

- [ ] **Step 5: Commit**

```bash
git add gateway-svc/src/middleware/rate_limit.rs
git commit -m "feat(gateway): implement per-IP rate limit middleware via governor"
```

---

### Task 3: Export module and wire into routes

**Files:**
- Modify: `gateway-svc/src/middleware/mod.rs`
- Modify: `gateway-svc/src/routes/mod.rs`

- [ ] **Step 1: Export rate_limit module**

Edit `gateway-svc/src/middleware/mod.rs`. Current content:

```rust
pub mod logging;
pub mod request_id;
pub mod security_headers;

pub use security_headers::add_security_headers;
```

Replace with:

```rust
pub mod logging;
pub mod rate_limit;
pub mod request_id;
pub mod security_headers;

pub use security_headers::add_security_headers;
```

- [ ] **Step 2: Apply layer in create_routes**

Edit `gateway-svc/src/routes/mod.rs`. Current content:

```rust
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

    let health_routes = Router::new()
        .route("/healthz", get(health_check))
        .route("/readyz", get(readiness_check));

    let proxy_routes = Router::new()
        .fallback(proxy::proxy_handler)
        .with_state(state);

    Router::new()
        .merge(health_routes)
        .merge(proxy_routes)
        .layer(middleware::from_fn(logging_middleware))
        .layer(middleware::from_fn(request_id_middleware))
}
```

Replace with:

```rust
use axum::{middleware, routing::get, Router};
use std::sync::Arc;

use crate::config::AppConfig;
use crate::middleware::{
    logging::logging_middleware,
    rate_limit::apply_rate_limit,
    request_id::request_id_middleware,
};
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
) -> Router {
    let state = Arc::new(AppState {
        proxy_client,
        router,
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
```

- [ ] **Step 3: Verify it compiles**

```bash
cd gateway-svc && cargo check 2>&1
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add gateway-svc/src/middleware/mod.rs gateway-svc/src/routes/mod.rs
git commit -m "feat(gateway): wire rate limit layer into route stack"
```

---

### Task 4: Switch to ConnectInfo in main.rs

**Files:**
- Modify: `gateway-svc/src/main.rs`

`PeerIpKeyExtractor` requires `ConnectInfo<SocketAddr>` injected into request extensions. This is done by replacing `into_make_service()` with `into_make_service_with_connect_info::<SocketAddr>()` in all serve calls.

- [ ] **Step 1: Update main.rs**

Edit `gateway-svc/src/main.rs`. Find the TLS branch (lines 84–88):

```rust
        let http_future =
            axum_server::bind(http_addr).serve(app.clone().into_make_service());
        let https_future =
            axum_server::bind_rustls(https_addr, tls_config).serve(app.into_make_service());
```

Replace with:

```rust
        let http_future = axum_server::bind(http_addr)
            .serve(app.clone().into_make_service_with_connect_info::<SocketAddr>());
        let https_future = axum_server::bind_rustls(https_addr, tls_config)
            .serve(app.into_make_service_with_connect_info::<SocketAddr>());
```

Then find the HTTP-only branch (line 105):

```rust
        if let Err(e) = axum::serve(listener, app).await {
```

Replace with:

```rust
        if let Err(e) = axum::serve(listener, app.into_make_service_with_connect_info::<SocketAddr>()).await {
```

- [ ] **Step 2: Verify compile**

```bash
cd gateway-svc && cargo check 2>&1
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add gateway-svc/src/main.rs
git commit -m "fix(gateway): use into_make_service_with_connect_info for PeerIpKeyExtractor"
```

---

### Task 5: Enable rate limiting in config

**Files:**
- Modify: `gateway-svc/config.yaml`

- [ ] **Step 1: Update rate_limit block**

Find the `middleware:` section in `gateway-svc/config.yaml`:

```yaml
middleware:
  rate_limit:
    enabled: false
    max_requests: 1000
    window_secs: 60
```

Replace with:

```yaml
middleware:
  rate_limit:
    enabled: true
    max_requests: 60    # replenishment rate: 60 req/s per IP
    window_secs: 1      # window unit (keep at 1 — max_requests is treated as per-second)
```

- [ ] **Step 2: Build the gateway binary**

```bash
cd gateway-svc && cargo build --release 2>&1 | tail -20
```

Expected: `Compiling gateway ...` → `Finished release [optimized]`. Zero errors.

- [ ] **Step 3: Commit**

```bash
git add gateway-svc/config.yaml
git commit -m "feat(gateway): enable rate limiting at 60 req/s per IP"
```

---

### Task 6: Remove rate limiting from Go API

**Files:**
- Modify: `api/pkg/server/server.go`
- Modify: `api/pkg/server/router.go`
- Delete: `api/pkg/server/middleware_ratelimit.go`

- [ ] **Step 1: Remove RateLimitMiddleware from server.go**

Edit `api/pkg/server/server.go`. Find line 17:

```go
	handler := CORSMiddleware(RateLimitMiddleware(JWTMiddleware(mux)))
```

Replace with:

```go
	handler := CORSMiddleware(JWTMiddleware(mux))
```

- [ ] **Step 2: Remove LoginRateLimitMiddleware from router.go**

Edit `api/pkg/server/router.go`. Find line 28:

```go
	mux.HandleFunc("POST /api/auth/login", LoginRateLimitMiddleware(LoginHandler))
```

Replace with:

```go
	mux.HandleFunc("POST /api/auth/login", LoginHandler)
```

- [ ] **Step 3: Delete the rate limit file**

```bash
rm api/pkg/server/middleware_ratelimit.go
```

- [ ] **Step 4: Verify Go build**

```bash
cd api && go build ./... 2>&1
```

Expected: no output (success). If you see "undefined: RateLimitMiddleware" or "undefined: LoginRateLimitMiddleware", you missed a reference — search with `grep -r "RateLimitMiddleware\|LoginRateLimitMiddleware" api/` and remove any remaining usages.

- [ ] **Step 5: Run Go tests**

```bash
cd api && go test ./... 2>&1
```

Expected: all tests pass. The middleware tests that referenced rate limiting (if any) will no longer exist since the file was deleted.

- [ ] **Step 6: Commit**

```bash
git add api/pkg/server/server.go api/pkg/server/router.go
git rm api/pkg/server/middleware_ratelimit.go
git commit -m "feat(api): remove rate limiting middleware (moved to gateway)"
```

---

### Task 7: Integration test

- [ ] **Step 1: Rebuild containers**

```bash
docker-compose build gateway api 2>&1 | tail -20
```

Expected: both images build successfully.

- [ ] **Step 2: Restart services**

```bash
docker-compose up -d gateway api
```

- [ ] **Step 3: Verify normal requests still work**

```bash
curl -s -o /dev/null -w "%{http_code}" http://localhost/health/simple
```

Expected: `200`

- [ ] **Step 4: Verify 429 fires after burst**

```bash
for i in $(seq 1 150); do curl -s -o /dev/null -w "%{http_code}\n" http://localhost/api/dashboard/stats; done | sort | uniq -c
```

Expected: mix of `200` and `429` — once the 60-token burst is exhausted at ~60 req/s, subsequent requests within the same second receive `429`.

- [ ] **Step 5: Verify 429 clears after 1 second**

```bash
# Hit the limit
for i in $(seq 1 100); do curl -s -o /dev/null -w "%{http_code}\n" http://localhost/api/dashboard/stats; done
# Wait 2 seconds, then confirm normal response
sleep 2 && curl -s -o /dev/null -w "%{http_code}" http://localhost/api/dashboard/stats
```

Expected: `200` after the wait.

- [ ] **Step 6: Final commit (if any config tweaks needed)**

```bash
git add -p  # stage any adjustments made during testing
git commit -m "chore: post-integration test tweaks (if any)"
```
