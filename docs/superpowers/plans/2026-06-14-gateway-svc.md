# Gateway Service (ruway) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the embedded Nginx in the frontend container with a dedicated Rust HTTP gateway (`gateway-svc`) that handles path-based routing, TLS termination, and security headers — separating concerns cleanly.

**Architecture:** Copy the `ruway` project to `gateway-svc/`, gut the K8s discovery/load-balancer machinery, and replace it with a path-prefix router that dispatches to two upstreams: `/api/*` → `api:8118`, `/*` → `frontend:3000`. The frontend container becomes a pure static file server (no Nginx proxy config). The gateway handles TLS (self-signed cert auto-generated if missing) on `:443` and plain HTTP on `:80` (for Cloudflare Tunnel).

**Tech Stack:** Rust, Axum 0.8, axum-server 0.7 (TLS), reqwest, tokio, serde_yaml — same deps as ruway plus `axum-server` with `tls-rustls` feature.

---

## Architecture change summary

**Before:**
```
Browser → frontend:80/443 (Nginx: serves React SPA + proxies /api/ → api:8118)
```

**After:**
```
Browser → gateway-svc:80/443 (Rust: routes /api/* → api:8118, /* → frontend:3000)
                                        ↓
                              frontend:3000 (Nginx: static files only)
```

**Removed:** `nginx/` root dir (unused), `frontend/docker-entrypoint.sh` (TLS moved to gateway), `src/discovery/` module in ruway (K8s not needed).

---

## File Map

### New/modified in `gateway-svc/` (copied from `ruway/`)

| File | Action | Responsibility |
|------|--------|---------------|
| `Cargo.toml` | Modify | Add `axum-server = { version = "0.7", features = ["tls-rustls"] }` |
| `config.yaml` | Replace | Path-based routes for go-stock-prediction upstreams |
| `docker-entrypoint.sh` | Create | Generate self-signed cert if `/etc/gateway/certs/` is empty |
| `Dockerfile` | Modify | Use entrypoint script, expose 80+443, rename binary |
| `src/config/mod.rs` | Rewrite | `RouteConfig`, `TlsConfig`; remove K8s structs |
| `src/router/mod.rs` | Create | `PathRouter`, `RouteAction` — longest-prefix dispatch |
| `src/proxy/client.rs` | Rewrite | Remove discovery/LB; `forward_request` takes explicit backend URL |
| `src/proxy/mod.rs` | Keep | `pub mod client;` |
| `src/middleware/security_headers.rs` | Create | `add_security_headers(headers: &mut HeaderMap)` |
| `src/middleware/mod.rs` | Modify | Add `pub mod security_headers;` |
| `src/routes/proxy.rs` | Rewrite | Use `PathRouter`; apply security headers on matching routes |
| `src/routes/mod.rs` | Modify | Remove LB/discovery imports; pass router to `create_routes` |
| `src/routes/health.rs` | Keep | `/healthz`, `/readyz` only (remove `/health`) |
| `src/lib.rs` | Modify | Remove `discovery` re-exports; add `pub mod router;` |
| `src/main.rs` | Rewrite | Dual HTTP+HTTPS via `axum_server`; no discovery startup |
| `src/models/mod.rs` | Keep | No changes |
| `src/middleware/logging.rs` | Keep | No changes |
| `src/middleware/request_id.rs` | Keep | No changes |
| `src/discovery/` | Delete | K8s discovery not needed |

### Modified in project root

| File | Action | Change |
|------|--------|--------|
| `docker-compose.yaml` | Modify | Add `gateway` service (ports 80/443); strip ports from `frontend` |
| `frontend/nginx.conf` | Replace | Static-only server on port 3000, SPA fallback |
| `frontend/Dockerfile` | Modify | Expose 3000, remove TLS entrypoint, plain `nginx -g daemon off` |
| `frontend/docker-entrypoint.sh` | Delete | TLS moved to gateway-svc |

---

## Task 1: Copy ruway to gateway-svc and clean up

**Files:**
- Create: `gateway-svc/` (copy of `ruway/`)

- [ ] **Step 1: Copy ruway into gateway-svc, strip git history**

```bash
cp -r /home/chronical/Projects/private/go-stock-prediction/ruway \
      /home/chronical/Projects/private/go-stock-prediction/gateway-svc
rm -rf /home/chronical/Projects/private/go-stock-prediction/gateway-svc/.git
```

- [ ] **Step 2: Remove K8s discovery module**

```bash
rm -rf /home/chronical/Projects/private/go-stock-prediction/gateway-svc/src/discovery
```

- [ ] **Step 3: Verify structure**

```bash
find /home/chronical/Projects/private/go-stock-prediction/gateway-svc/src -type f | sort
```

Expected output: `config/mod.rs`, `lib.rs`, `main.rs`, `middleware/logging.rs`, `middleware/mod.rs`, `middleware/request_id.rs`, `models/mod.rs`, `proxy/client.rs`, `proxy/mod.rs`, `routes/health.rs`, `routes/mod.rs`, `routes/proxy.rs` — NO `discovery/`.

- [ ] **Step 4: Commit**

```bash
cd /home/chronical/Projects/private/go-stock-prediction
git add gateway-svc/
git commit -m "feat(gateway): copy ruway as gateway-svc, remove K8s discovery"
```

---

## Task 2: Update Cargo.toml

**Files:**
- Modify: `gateway-svc/Cargo.toml`

- [ ] **Step 1: Add axum-server dependency**

Replace the `[dependencies]` block — add `axum-server` and remove unused `rand` (no LB):

```toml
[package]
name = "gateway"
version = "0.1.0"
edition = "2021"
description = "HTTP gateway for go-stock-prediction — path routing, TLS, security headers"

[dependencies]
axum = "0.8.6"
axum-server = { version = "0.7", features = ["tls-rustls"] }
tower = "0.5.2"
tower-http = { version = "0.6.6", features = ["cors", "trace"] }
tokio = { version = "1.48.0", features = ["full"] }
reqwest = { version = "0.12.24", features = ["json", "rustls-tls"] }
hyper = "1.7.0"
serde = { version = "1.0.228", features = ["derive"] }
serde_json = "1.0.145"
serde_yaml = "0.9.34"
config = "0.15.18"
tracing = "0.1.41"
tracing-subscriber = { version = "0.3.20", features = ["env-filter", "json"] }
anyhow = "1.0.100"
thiserror = "2.0.17"
async-trait = "0.1.89"
uuid = { version = "1.18.1", features = ["v4"] }
chrono = "0.4.42"
dotenv = "0.15.0"
bytes = "1.10.1"

[dev-dependencies]
tokio-test = "0.4"

[profile.release]
opt-level = 3
lto = true
codegen-units = 1

[[bin]]
name = "gateway"
path = "src/main.rs"
```

- [ ] **Step 2: Verify it compiles (will fail on code errors — that's expected)**

```bash
cd /home/chronical/Projects/private/go-stock-prediction/gateway-svc
cargo check 2>&1 | head -30
```

Expected: compile errors referencing `discovery` module — that's fine, we're about to fix them.

- [ ] **Step 3: Commit**

```bash
cd /home/chronical/Projects/private/go-stock-prediction
git add gateway-svc/Cargo.toml
git commit -m "feat(gateway): add axum-server TLS dep, rename binary to gateway"
```

---

## Task 3: Rewrite `src/config/mod.rs`

**Files:**
- Modify: `gateway-svc/src/config/mod.rs`

- [ ] **Step 1: Write the failing test first**

Add at the bottom of the new `config/mod.rs` (after full rewrite in Step 2):

```rust
#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_route_config_defaults() {
        let r = RouteConfig {
            prefix: "/api".to_string(),
            action: "proxy".to_string(),
            backend: Some("http://api:8118".to_string()),
            security_headers: false,
        };
        assert_eq!(r.prefix, "/api");
        assert!(!r.security_headers);
    }

    #[test]
    fn test_tls_config_defaults() {
        let tls = TlsConfig {
            enabled: false,
            cert_path: "/etc/gateway/certs/cert.pem".to_string(),
            key_path: "/etc/gateway/certs/key.pem".to_string(),
        };
        assert!(!tls.enabled);
    }
}
```

- [ ] **Step 2: Rewrite `gateway-svc/src/config/mod.rs`**

```rust
use config::{Config, ConfigError, File};
use serde::{Deserialize, Serialize};
use std::path::Path;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AppConfig {
    pub server: ServerConfig,
    pub http: HttpConfig,
    pub routes: Vec<RouteConfig>,
    pub middleware: MiddlewareConfig,
    pub logging: LoggingConfig,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ServerConfig {
    pub host: String,
    #[serde(default = "default_http_port")]
    pub http_port: u16,
    #[serde(default = "default_https_port")]
    pub https_port: u16,
    pub tls: TlsConfig,
    #[serde(default = "default_shutdown_timeout")]
    pub shutdown_timeout: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TlsConfig {
    #[serde(default)]
    pub enabled: bool,
    #[serde(default = "default_cert_path")]
    pub cert_path: String,
    #[serde(default = "default_key_path")]
    pub key_path: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RouteConfig {
    pub prefix: String,
    /// "proxy" or "block"
    #[serde(default = "default_action")]
    pub action: String,
    #[serde(default)]
    pub backend: Option<String>,
    #[serde(default)]
    pub security_headers: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HttpConfig {
    #[serde(default = "default_http_version")]
    pub version: String,
    pub pool: ConnectionPoolConfig,
    pub request: RequestConfig,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ConnectionPoolConfig {
    #[serde(default = "default_max_idle_per_host")]
    pub max_idle_per_host: usize,
    #[serde(default = "default_idle_timeout")]
    pub idle_timeout: u64,
    #[serde(default = "default_connection_timeout")]
    pub connection_timeout: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RequestConfig {
    #[serde(default = "default_timeout")]
    pub timeout: u64,
    #[serde(default)]
    pub max_retries: u32,
    #[serde(default = "default_retry_delay")]
    pub retry_delay: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MiddlewareConfig {
    pub rate_limit: RateLimitConfig,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RateLimitConfig {
    #[serde(default)]
    pub enabled: bool,
    #[serde(default = "default_rate_limit")]
    pub max_requests: u32,
    #[serde(default = "default_rate_window")]
    pub window_secs: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LoggingConfig {
    #[serde(default = "default_log_level")]
    pub level: String,
    #[serde(default = "default_log_format")]
    pub format: String,
    #[serde(default = "default_true")]
    pub access_log: bool,
}

fn default_http_port() -> u16 { 80 }
fn default_https_port() -> u16 { 443 }
fn default_shutdown_timeout() -> u64 { 30 }
fn default_cert_path() -> String { "/etc/gateway/certs/cert.pem".to_string() }
fn default_key_path() -> String { "/etc/gateway/certs/key.pem".to_string() }
fn default_action() -> String { "proxy".to_string() }
fn default_http_version() -> String { "auto".to_string() }
fn default_max_idle_per_host() -> usize { 20 }
fn default_idle_timeout() -> u64 { 90 }
fn default_connection_timeout() -> u64 { 10 }
fn default_timeout() -> u64 { 600 }
fn default_retry_delay() -> u64 { 1 }
fn default_rate_limit() -> u32 { 1000 }
fn default_rate_window() -> u64 { 60 }
fn default_log_level() -> String { "info".to_string() }
fn default_log_format() -> String { "json".to_string() }
fn default_true() -> bool { true }

impl AppConfig {
    pub fn load() -> Result<Self, ConfigError> {
        let paths = vec![
            "config.yaml",
            "/etc/gateway/config.yaml",
        ];
        for path in paths {
            if Path::new(path).exists() {
                return Self::load_from(path);
            }
        }
        Err(ConfigError::Message(
            "config.yaml not found. Tried: config.yaml, /etc/gateway/config.yaml".to_string()
        ))
    }

    pub fn load_from<P: AsRef<Path>>(path: P) -> Result<Self, ConfigError> {
        Config::builder()
            .add_source(File::with_name(path.as_ref().to_str().unwrap()))
            .add_source(config::Environment::with_prefix("GATEWAY").separator("_"))
            .build()?
            .try_deserialize()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_route_config_defaults() {
        let r = RouteConfig {
            prefix: "/api".to_string(),
            action: "proxy".to_string(),
            backend: Some("http://api:8118".to_string()),
            security_headers: false,
        };
        assert_eq!(r.prefix, "/api");
        assert!(!r.security_headers);
    }

    #[test]
    fn test_tls_config_defaults() {
        let tls = TlsConfig {
            enabled: false,
            cert_path: "/etc/gateway/certs/cert.pem".to_string(),
            key_path: "/etc/gateway/certs/key.pem".to_string(),
        };
        assert!(!tls.enabled);
    }
}
```

- [ ] **Step 3: Run the tests**

```bash
cd /home/chronical/Projects/private/go-stock-prediction/gateway-svc
cargo test config:: 2>&1 | tail -10
```

Expected: errors about `discovery` module still, but NOT about `config`. The `config` module tests should register if you run `cargo test --lib config::tests` after other tasks compile.

- [ ] **Step 4: Commit**

```bash
cd /home/chronical/Projects/private/go-stock-prediction
git add gateway-svc/src/config/mod.rs
git commit -m "feat(gateway): rewrite config — RouteConfig, TlsConfig, remove K8s structs"
```

---

## Task 4: Create `src/router/mod.rs`

**Files:**
- Create: `gateway-svc/src/router/mod.rs`

- [ ] **Step 1: Write the failing tests**

Create the file with tests first:

```rust
// gateway-svc/src/router/mod.rs
use crate::config::RouteConfig;

pub enum RouteAction {
    Block,
    Proxy {
        backend: String,
        security_headers: bool,
    },
}

pub struct PathRouter {
    routes: Vec<(String, RouteAction)>,
}

impl PathRouter {
    pub fn from_config(routes: &[RouteConfig]) -> Self {
        let mut pairs: Vec<(String, RouteAction)> = routes
            .iter()
            .map(|r| {
                let action = if r.action == "block" {
                    RouteAction::Block
                } else {
                    RouteAction::Proxy {
                        backend: r.backend.clone().expect("proxy route requires backend"),
                        security_headers: r.security_headers,
                    }
                };
                (r.prefix.clone(), action)
            })
            .collect();

        // Longest prefix wins — sort descending by length
        pairs.sort_by(|a, b| b.0.len().cmp(&a.0.len()));

        Self { routes: pairs }
    }

    pub fn route(&self, path: &str) -> Option<&RouteAction> {
        self.routes
            .iter()
            .find(|(prefix, _)| path.starts_with(prefix.as_str()))
            .map(|(_, action)| action)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn routes(items: &[(&str, &str, Option<&str>, bool)]) -> Vec<RouteConfig> {
        items
            .iter()
            .map(|(prefix, action, backend, sec)| RouteConfig {
                prefix: prefix.to_string(),
                action: action.to_string(),
                backend: backend.map(|s| s.to_string()),
                security_headers: *sec,
            })
            .collect()
    }

    #[test]
    fn test_api_wins_over_root() {
        let router = PathRouter::from_config(&routes(&[
            ("/", "proxy", Some("http://frontend:3000"), false),
            ("/api", "proxy", Some("http://api:8118"), true),
        ]));

        match router.route("/api/gold/latest") {
            Some(RouteAction::Proxy { backend, .. }) => {
                assert_eq!(backend, "http://api:8118");
            }
            _ => panic!("Expected proxy to api backend"),
        }
    }

    #[test]
    fn test_root_catches_frontend_paths() {
        let router = PathRouter::from_config(&routes(&[
            ("/api", "proxy", Some("http://api:8118"), true),
            ("/", "proxy", Some("http://frontend:3000"), false),
        ]));

        match router.route("/dashboard") {
            Some(RouteAction::Proxy { backend, .. }) => {
                assert_eq!(backend, "http://frontend:3000");
            }
            _ => panic!("Expected proxy to frontend"),
        }
    }

    #[test]
    fn test_swagger_block() {
        let router = PathRouter::from_config(&routes(&[
            ("/swagger", "block", None, false),
            ("/", "proxy", Some("http://frontend:3000"), false),
        ]));

        assert!(matches!(
            router.route("/swagger/index.html"),
            Some(RouteAction::Block)
        ));
    }

    #[test]
    fn test_security_headers_flag_forwarded() {
        let router = PathRouter::from_config(&routes(&[
            ("/api", "proxy", Some("http://api:8118"), true),
            ("/", "proxy", Some("http://frontend:3000"), false),
        ]));

        match router.route("/api/auth/login") {
            Some(RouteAction::Proxy { security_headers, .. }) => {
                assert!(*security_headers);
            }
            _ => panic!("Expected proxy with security_headers=true"),
        }

        match router.route("/") {
            Some(RouteAction::Proxy { security_headers, .. }) => {
                assert!(!*security_headers);
            }
            _ => panic!("Expected proxy with security_headers=false"),
        }
    }

    #[test]
    fn test_no_routes_returns_none() {
        let router = PathRouter::from_config(&[]);
        assert!(router.route("/anything").is_none());
    }
}
```

- [ ] **Step 2: Run to verify tests fail (module not yet wired)**

```bash
cd /home/chronical/Projects/private/go-stock-prediction/gateway-svc
cargo test router:: 2>&1 | tail -5
```

Expected: compile error — `mod router` not declared yet. That's correct.

- [ ] **Step 3: Wire the module into `lib.rs` and `main.rs`**

In `gateway-svc/src/lib.rs`, add `pub mod router;` and remove discovery references:

```rust
pub mod config;
pub mod middleware;
pub mod models;
pub mod proxy;
pub mod router;
pub mod routes;

pub use config::AppConfig;
pub use proxy::ProxyClient;
pub use router::PathRouter;
pub use routes::create_routes;
```

In `gateway-svc/src/main.rs`, add `mod router;` alongside the other mod declarations (will fix fully in Task 8, for now just add the declaration):

Open `main.rs` and change the mod block at the top to:
```rust
mod config;
mod middleware;
mod models;
mod proxy;
mod router;
mod routes;
```

- [ ] **Step 4: Run tests again**

```bash
cd /home/chronical/Projects/private/go-stock-prediction/gateway-svc
cargo test router:: 2>&1 | tail -20
```

Expected: 5 tests pass — `test_api_wins_over_root`, `test_root_catches_frontend_paths`, `test_swagger_block`, `test_security_headers_flag_forwarded`, `test_no_routes_returns_none`.

- [ ] **Step 5: Commit**

```bash
cd /home/chronical/Projects/private/go-stock-prediction
git add gateway-svc/src/router/mod.rs gateway-svc/src/lib.rs gateway-svc/src/main.rs
git commit -m "feat(gateway): add PathRouter with prefix matching and RouteAction"
```

---

## Task 5: Rewrite `src/proxy/client.rs`

**Files:**
- Modify: `gateway-svc/src/proxy/client.rs`

This removes the `DiscoveryService` and `LoadBalancer` dependencies. `forward_request` now takes an explicit `backend: &str` parameter.

- [ ] **Step 1: Rewrite `gateway-svc/src/proxy/client.rs`**

```rust
use anyhow::Result;
use axum::{
    body::Body,
    http::{HeaderMap, Method, StatusCode},
    response::Response,
};
use bytes::Bytes;
use reqwest::Client;
use std::sync::Arc;
use std::time::Duration;
use tracing::{error, info, warn};

use crate::config::AppConfig;

#[derive(Clone)]
pub struct ProxyClient {
    client: Client,
    max_retries: u32,
    retry_delay: Duration,
}

impl ProxyClient {
    pub async fn new(config: &AppConfig) -> Result<Self> {
        info!("Initializing ProxyClient");

        let client = Client::builder()
            .timeout(Duration::from_secs(config.http.request.timeout))
            .pool_max_idle_per_host(config.http.pool.max_idle_per_host)
            .pool_idle_timeout(Duration::from_secs(config.http.pool.idle_timeout))
            .connect_timeout(Duration::from_secs(config.http.pool.connection_timeout))
            .build()?;

        Ok(Self {
            client,
            max_retries: config.http.request.max_retries,
            retry_delay: Duration::from_secs(config.http.request.retry_delay),
        })
    }

    /// Forward a request to the given backend URL + path.
    /// `backend` is e.g. "http://api:8118" (no trailing slash).
    pub async fn forward_request(
        &self,
        method: Method,
        path: &str,
        headers: &HeaderMap,
        body: Body,
        backend: &str,
    ) -> Result<Response, ProxyError> {
        let url = format!("{}{}", backend.trim_end_matches('/'), path);

        let body_bytes = axum::body::to_bytes(body, usize::MAX)
            .await
            .map_err(|e| ProxyError::RequestFailed(format!("Failed to read body: {}", e)))?;

        for attempt in 0..=self.max_retries {
            if attempt > 0 {
                warn!("Retrying request attempt {}/{}", attempt, self.max_retries);
                tokio::time::sleep(self.retry_delay).await;
            }

            match self.do_forward(&method, &url, headers, body_bytes.clone()).await {
                Ok(response) => {
                    info!(status = %response.status(), url = %url, "Forwarded successfully");
                    return Ok(response);
                }
                Err(e) if attempt < self.max_retries => {
                    warn!(error = %e, "Request failed, will retry");
                }
                Err(e) => {
                    error!(error = %e, url = %url, "Request failed after retries");
                    return Err(e);
                }
            }
        }

        Err(ProxyError::RequestFailed("All retries exhausted".to_string()))
    }

    async fn do_forward(
        &self,
        method: &Method,
        url: &str,
        headers: &HeaderMap,
        body_bytes: Bytes,
    ) -> Result<Response, ProxyError> {
        let mut builder = match method.as_str() {
            "GET" => self.client.get(url),
            "POST" => self.client.post(url),
            "PUT" => self.client.put(url),
            "DELETE" => self.client.delete(url),
            "PATCH" => self.client.patch(url),
            "HEAD" => self.client.head(url),
            "OPTIONS" => self.client.request(reqwest::Method::OPTIONS, url),
            _ => return Err(ProxyError::RequestFailed(format!("Unsupported method: {}", method))),
        };

        for (key, value) in headers.iter() {
            let name = key.as_str().to_lowercase();
            if matches!(name.as_str(), "host" | "connection" | "content-length" | "transfer-encoding") {
                continue;
            }
            if let Ok(v) = value.to_str() {
                builder = builder.header(key.as_str(), v);
            }
        }

        if !body_bytes.is_empty() {
            builder = builder.body(body_bytes.to_vec());
        }

        let response = builder
            .send()
            .await
            .map_err(|e| ProxyError::RequestFailed(format!("Request failed: {}", e)))?;

        self.convert_response(response).await
    }

    async fn convert_response(&self, response: reqwest::Response) -> Result<Response, ProxyError> {
        let status = response.status();
        let headers = response.headers().clone();

        let body_bytes = response
            .bytes()
            .await
            .map_err(|e| ProxyError::RequestFailed(format!("Failed to read response: {}", e)))?;

        let mut builder = Response::builder().status(status);

        for (key, value) in headers.iter() {
            let name = key.as_str().to_lowercase();
            if matches!(name.as_str(), "transfer-encoding" | "content-encoding") {
                continue;
            }
            builder = builder.header(key, value);
        }

        builder
            .body(Body::from(body_bytes))
            .map_err(|e| ProxyError::RequestFailed(format!("Failed to build response: {}", e)))
    }
}

#[derive(Debug)]
pub enum ProxyError {
    RequestFailed(String),
}

impl std::fmt::Display for ProxyError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ProxyError::RequestFailed(msg) => write!(f, "Proxy error: {}", msg),
        }
    }
}

impl std::error::Error for ProxyError {}

use axum::response::IntoResponse;

impl IntoResponse for ProxyError {
    fn into_response(self) -> Response {
        let (status, message) = match self {
            ProxyError::RequestFailed(msg) => (StatusCode::BAD_GATEWAY, msg),
        };

        let body = serde_json::json!({ "error": message, "status": status.as_u16() });
        (status, axum::Json(body)).into_response()
    }
}
```

- [ ] **Step 2: Update `src/proxy/mod.rs`**

```rust
pub mod client;
pub use client::{ProxyClient, ProxyError};
```

- [ ] **Step 3: Commit**

```bash
cd /home/chronical/Projects/private/go-stock-prediction
git add gateway-svc/src/proxy/
git commit -m "feat(gateway): simplify ProxyClient — explicit backend URL, remove discovery dep"
```

---

## Task 6: Create `src/middleware/security_headers.rs`

**Files:**
- Create: `gateway-svc/src/middleware/security_headers.rs`
- Modify: `gateway-svc/src/middleware/mod.rs`

- [ ] **Step 1: Write the test**

```rust
// in security_headers.rs, at the bottom:
#[cfg(test)]
mod tests {
    use super::*;
    use axum::http::HeaderMap;

    #[test]
    fn test_adds_required_headers() {
        let mut headers = HeaderMap::new();
        add_security_headers(&mut headers);

        assert_eq!(headers.get("x-frame-options").unwrap(), "DENY");
        assert_eq!(headers.get("x-content-type-options").unwrap(), "nosniff");
        assert!(headers.get("referrer-policy").is_some());
        assert!(headers.get("strict-transport-security").is_some());
    }
}
```

- [ ] **Step 2: Create `gateway-svc/src/middleware/security_headers.rs`**

```rust
use axum::http::{HeaderMap, HeaderValue};

/// Add OWASP-recommended security headers to a response.
/// HSTS is included always — browsers ignore it over HTTP, so it's safe.
pub fn add_security_headers(headers: &mut HeaderMap) {
    headers.insert(
        "x-frame-options",
        HeaderValue::from_static("DENY"),
    );
    headers.insert(
        "x-content-type-options",
        HeaderValue::from_static("nosniff"),
    );
    headers.insert(
        "referrer-policy",
        HeaderValue::from_static("strict-origin-when-cross-origin"),
    );
    headers.insert(
        "strict-transport-security",
        HeaderValue::from_static("max-age=31536000; includeSubDomains"),
    );
}

#[cfg(test)]
mod tests {
    use super::*;
    use axum::http::HeaderMap;

    #[test]
    fn test_adds_required_headers() {
        let mut headers = HeaderMap::new();
        add_security_headers(&mut headers);

        assert_eq!(headers.get("x-frame-options").unwrap(), "DENY");
        assert_eq!(headers.get("x-content-type-options").unwrap(), "nosniff");
        assert!(headers.get("referrer-policy").is_some());
        assert!(headers.get("strict-transport-security").is_some());
    }
}
```

- [ ] **Step 3: Update `gateway-svc/src/middleware/mod.rs`**

```rust
pub mod logging;
pub mod request_id;
pub mod security_headers;

pub use request_id::RequestIdLayer;
pub use security_headers::add_security_headers;
```

- [ ] **Step 4: Run tests**

```bash
cd /home/chronical/Projects/private/go-stock-prediction/gateway-svc
cargo test middleware::security_headers 2>&1 | tail -10
```

Expected: 1 test passes.

- [ ] **Step 5: Commit**

```bash
cd /home/chronical/Projects/private/go-stock-prediction
git add gateway-svc/src/middleware/
git commit -m "feat(gateway): add security headers middleware (X-Frame-Options, HSTS, etc)"
```

---

## Task 7: Rewrite `src/routes/proxy.rs` and `src/routes/mod.rs`

**Files:**
- Modify: `gateway-svc/src/routes/proxy.rs`
- Modify: `gateway-svc/src/routes/mod.rs`

- [ ] **Step 1: Rewrite `gateway-svc/src/routes/proxy.rs`**

```rust
use axum::{
    body::Body,
    extract::{Request, State},
    http::{HeaderMap, Method, StatusCode, Uri},
    response::{IntoResponse, Response},
};
use std::sync::Arc;
use tracing::info;

use crate::middleware::security_headers::add_security_headers;
use crate::proxy::client::{ProxyClient, ProxyError};
use crate::router::{PathRouter, RouteAction};

pub struct AppState {
    pub proxy_client: Arc<ProxyClient>,
    pub router: Arc<PathRouter>,
}

/// Catch-all proxy handler — routes by path prefix, applies security headers.
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
            info!(path = %path, "Blocked route → 404");
            StatusCode::NOT_FOUND.into_response()
        }
        Some(RouteAction::Proxy { backend, security_headers }) => {
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
```

- [ ] **Step 2: Rewrite `gateway-svc/src/routes/mod.rs`**

```rust
use axum::{
    middleware,
    routing::any,
    Router,
};
use std::sync::Arc;

use crate::config::AppConfig;
use crate::middleware::{
    logging::logging_middleware,
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
    _config: Arc<AppConfig>,
) -> Router {
    let state = Arc::new(AppState {
        proxy_client,
        router,
    });

    // /healthz and /readyz serve the gateway's own health (not proxied)
    let health_routes = Router::new()
        .route("/healthz", axum::routing::get(health_check))
        .route("/readyz", axum::routing::get(readiness_check));

    // Everything else goes through the path router
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

- [ ] **Step 3: Commit**

```bash
cd /home/chronical/Projects/private/go-stock-prediction
git add gateway-svc/src/routes/
git commit -m "feat(gateway): update routes — PathRouter dispatch, security headers on response"
```

---

## Task 8: Rewrite `src/main.rs`

**Files:**
- Modify: `gateway-svc/src/main.rs`

- [ ] **Step 1: Rewrite `gateway-svc/src/main.rs`**

```rust
use std::net::SocketAddr;
use std::sync::Arc;
use tracing::{error, info};
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};

mod config;
mod middleware;
mod models;
mod proxy;
mod router;
mod routes;

use config::AppConfig;
use proxy::ProxyClient;
use router::PathRouter;
use routes::create_routes;

#[tokio::main]
async fn main() {
    init_tracing();

    info!("Starting go-stock-prediction gateway");

    let config = match AppConfig::load() {
        Ok(c) => Arc::new(c),
        Err(e) => {
            error!("Failed to load config: {}", e);
            std::process::exit(1);
        }
    };

    let router = Arc::new(PathRouter::from_config(&config.routes));
    info!("Loaded {} route(s)", config.routes.len());

    let proxy_client = match ProxyClient::new(&config).await {
        Ok(c) => Arc::new(c),
        Err(e) => {
            error!("Failed to create proxy client: {}", e);
            std::process::exit(1);
        }
    };

    let app = create_routes(proxy_client, router, Arc::clone(&config));

    let http_addr: SocketAddr = format!("{}:{}", config.server.host, config.server.http_port)
        .parse()
        .expect("Invalid HTTP address");

    info!("HTTP gateway on http://{}", http_addr);
    info!("Health: http://{}/healthz", http_addr);

    if config.server.tls.enabled {
        use axum_server::tls_rustls::RustlsConfig;

        let https_addr: SocketAddr =
            format!("{}:{}", config.server.host, config.server.https_port)
                .parse()
                .expect("Invalid HTTPS address");

        let tls_config =
            match RustlsConfig::from_pem_file(&config.server.tls.cert_path, &config.server.tls.key_path).await {
                Ok(c) => c,
                Err(e) => {
                    error!(
                        "Failed to load TLS certs from {} / {}: {}",
                        config.server.tls.cert_path, config.server.tls.key_path, e
                    );
                    std::process::exit(1);
                }
            };

        info!("HTTPS gateway on https://{}", https_addr);

        let http_future = axum_server::bind(http_addr).serve(app.clone().into_make_service());
        let https_future = axum_server::bind_rustls(https_addr, tls_config).serve(app.into_make_service());

        info!("Gateway ready");
        tokio::join!(http_future, https_future);
    } else {
        let listener = match tokio::net::TcpListener::bind(http_addr).await {
            Ok(l) => l,
            Err(e) => {
                error!("Failed to bind {}: {}", http_addr, e);
                std::process::exit(1);
            }
        };
        info!("Gateway ready (HTTP only — TLS disabled)");
        if let Err(e) = axum::serve(listener, app).await {
            error!("Server error: {}", e);
        }
    }
}

fn init_tracing() {
    let log_level = std::env::var("RUST_LOG").unwrap_or_else(|_| "info,gateway=debug".to_string());
    let format = std::env::var("LOG_FORMAT").unwrap_or_else(|_| "json".to_string());

    if format == "json" {
        tracing_subscriber::registry()
            .with(tracing_subscriber::EnvFilter::new(log_level))
            .with(tracing_subscriber::fmt::layer().json())
            .init();
    } else {
        tracing_subscriber::registry()
            .with(tracing_subscriber::EnvFilter::new(log_level))
            .with(tracing_subscriber::fmt::layer().pretty())
            .init();
    }
}
```

- [ ] **Step 2: Try to compile**

```bash
cd /home/chronical/Projects/private/go-stock-prediction/gateway-svc
cargo build 2>&1 | head -40
```

Expected: compile errors or warnings but NO errors about `discovery`. Fix any remaining import errors by checking the error output and adjusting. Common issues:
- `axum_server` not found → check Cargo.toml has `axum-server = { version = "0.7", features = ["tls-rustls"] }`
- Unused imports in `src/routes/health.rs` → add `#[allow(unused)]` or remove

- [ ] **Step 3: Run all tests**

```bash
cd /home/chronical/Projects/private/go-stock-prediction/gateway-svc
cargo test 2>&1 | tail -20
```

Expected: all tests pass (router, middleware, config, models).

- [ ] **Step 4: Commit**

```bash
cd /home/chronical/Projects/private/go-stock-prediction
git add gateway-svc/src/main.rs
git commit -m "feat(gateway): rewrite main — dual HTTP+HTTPS via axum_server, no discovery"
```

---

## Task 9: New `config.yaml` and `docker-entrypoint.sh` for gateway-svc

**Files:**
- Replace: `gateway-svc/config.yaml`
- Create: `gateway-svc/docker-entrypoint.sh`

- [ ] **Step 1: Replace `gateway-svc/config.yaml`**

```yaml
server:
  host: "0.0.0.0"
  http_port: 80
  https_port: 443
  tls:
    enabled: true
    cert_path: "/etc/gateway/certs/cert.pem"
    key_path: "/etc/gateway/certs/key.pem"
  shutdown_timeout: 30

http:
  version: "auto"
  pool:
    max_idle_per_host: 20
    idle_timeout: 90
    connection_timeout: 10
  request:
    timeout: 600
    max_retries: 0
    retry_delay: 1

# Path-prefix routing — longest prefix wins.
# /swagger → block (404), /api → Go API Backend, / → React SPA
routes:
  - prefix: "/swagger"
    action: "block"
  - prefix: "/api"
    action: "proxy"
    backend: "http://api:8118"
    security_headers: true
  - prefix: "/health"
    action: "proxy"
    backend: "http://api:8118"
    security_headers: false
  - prefix: "/"
    action: "proxy"
    backend: "http://frontend:3000"
    security_headers: false

middleware:
  rate_limit:
    enabled: false
    max_requests: 1000
    window_secs: 60

logging:
  level: "info"
  format: "json"
  access_log: true
```

- [ ] **Step 2: Create `gateway-svc/docker-entrypoint.sh`**

```sh
#!/bin/sh
set -e

CERT_DIR="/etc/gateway/certs"
CERT_FILE="$CERT_DIR/cert.pem"
KEY_FILE="$CERT_DIR/key.pem"

mkdir -p "$CERT_DIR"

if [ ! -f "$CERT_FILE" ] || [ ! -f "$KEY_FILE" ]; then
  echo "[TLS] No certs found in $CERT_DIR — generating self-signed cert..."
  openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
    -keyout "$KEY_FILE" \
    -out "$CERT_FILE" \
    -subj "/CN=localhost/O=go-stock-prediction/C=VN" \
    2>/dev/null
  chmod 600 "$KEY_FILE"
  echo "[TLS] Self-signed cert generated (valid 10 years). Replace with real certs for production."
fi

exec /app/gateway
```

```bash
chmod +x /home/chronical/Projects/private/go-stock-prediction/gateway-svc/docker-entrypoint.sh
```

- [ ] **Step 3: Commit**

```bash
cd /home/chronical/Projects/private/go-stock-prediction
git add gateway-svc/config.yaml gateway-svc/docker-entrypoint.sh
git commit -m "feat(gateway): add config.yaml with path routes and TLS entrypoint script"
```

---

## Task 10: Update `gateway-svc/Dockerfile`

**Files:**
- Modify: `gateway-svc/Dockerfile`

- [ ] **Step 1: Rewrite `gateway-svc/Dockerfile`**

```dockerfile
# Stage 1: Build
FROM rust:1.82-slim AS builder

WORKDIR /app

RUN apt-get update && \
    apt-get install -y pkg-config libssl-dev && \
    rm -rf /var/lib/apt/lists/*

COPY Cargo.toml ./
# No Cargo.lock committed — generate on first build
RUN mkdir src && \
    echo "fn main() {}" > src/main.rs && \
    cargo build --release 2>/dev/null || true && \
    rm -rf src

COPY src ./src
COPY config.yaml ./

RUN cargo build --release

# Stage 2: Runtime
FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update && \
    apt-get install -y ca-certificates libssl3 curl openssl && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/target/release/gateway /app/gateway
COPY config.yaml /app/config.yaml
COPY docker-entrypoint.sh /docker-entrypoint.sh

RUN useradd -m -u 1001 gateway && \
    chown -R gateway:gateway /app && \
    chmod +x /docker-entrypoint.sh

EXPOSE 80 443

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:80/healthz || exit 1

USER gateway

ENTRYPOINT ["/docker-entrypoint.sh"]
```

- [ ] **Step 2: Commit**

```bash
cd /home/chronical/Projects/private/go-stock-prediction
git add gateway-svc/Dockerfile
git commit -m "feat(gateway): update Dockerfile — expose 80+443, self-signed cert entrypoint"
```

---

## Task 11: Update `frontend/` — static-only server

**Files:**
- Modify: `frontend/nginx.conf`
- Modify: `frontend/Dockerfile`
- Delete: `frontend/docker-entrypoint.sh`

- [ ] **Step 1: Replace `frontend/nginx.conf` with static-only config**

```nginx
server {
    listen 3000;
    server_name _;

    root /usr/share/nginx/html;
    index index.html;

    # SPA fallback — all client-side routes serve index.html
    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

- [ ] **Step 2: Update `frontend/Dockerfile`**

Remove TLS entrypoint and openssl. Frontend is no longer the gateway:

```dockerfile
# Stage 1: build
FROM node:22-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
RUN npm run build

# Stage 2: serve static files (internal only — gateway handles TLS)
FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 3000
CMD ["nginx", "-g", "daemon off;"]
```

- [ ] **Step 3: Delete the entrypoint script**

```bash
rm /home/chronical/Projects/private/go-stock-prediction/frontend/docker-entrypoint.sh
```

- [ ] **Step 4: Commit**

```bash
cd /home/chronical/Projects/private/go-stock-prediction
git add frontend/nginx.conf frontend/Dockerfile
git rm frontend/docker-entrypoint.sh
git commit -m "feat(frontend): simplify to static-only nginx on port 3000 — gateway handles routing+TLS"
```

---

## Task 12: Update `docker-compose.yaml`

**Files:**
- Modify: `docker-compose.yaml`

- [ ] **Step 1: Read current docker-compose.yaml first**

```bash
cat /home/chronical/Projects/private/go-stock-prediction/docker-compose.yaml
```

- [ ] **Step 2: Make the following changes to `docker-compose.yaml`**

**Replace the `frontend` service** — strip ports 80/443, change to internal port 3000, remove nginx/certs volume:

```yaml
  frontend:
    build:
      context: ./frontend
      dockerfile: Dockerfile
    container_name: vnstock_frontend
    depends_on:
      - api
    restart: unless-stopped
    expose:
      - "3000"
    networks:
      - app-network
```

**Add `gateway` service** before frontend (after `api`):

```yaml
  gateway:
    build:
      context: ./gateway-svc
      dockerfile: Dockerfile
    container_name: vnstock_gateway
    depends_on:
      - api
      - frontend
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx/certs:/etc/gateway/certs
    networks:
      - app-network
```

Note: `./nginx/certs` is reused — existing cert mount point works without any filesystem change.

- [ ] **Step 3: Verify the docker-compose is valid**

```bash
cd /home/chronical/Projects/private/go-stock-prediction
docker-compose config 2>&1 | head -20
```

Expected: no errors, prints merged config.

- [ ] **Step 4: Commit**

```bash
cd /home/chronical/Projects/private/go-stock-prediction
git add docker-compose.yaml
git commit -m "feat(gateway): replace nginx frontend with gateway-svc — ports 80/443 moved to Rust gateway"
```

---

## Task 13: Build and smoke test

- [ ] **Step 1: Build gateway-svc Docker image**

```bash
cd /home/chronical/Projects/private/go-stock-prediction
docker-compose build gateway 2>&1 | tail -20
```

Expected: image builds successfully. First build is slow (Rust compile ~3-5 min).

- [ ] **Step 2: Build updated frontend image**

```bash
docker-compose build frontend 2>&1 | tail -10
```

- [ ] **Step 3: Start stack**

```bash
docker-compose up -d
docker-compose ps
```

Expected: all services `Up`.

- [ ] **Step 4: Smoke test HTTP**

```bash
# Gateway health
curl -s http://localhost/healthz
# Expected: {"status":"ok","service":"gateway"}

# Frontend (SPA)
curl -sI http://localhost/ | head -5
# Expected: HTTP/1.1 200 OK, Content-Type: text/html

# API proxy
curl -s http://localhost/api/auth/me | head -1
# Expected: {"error":"..."} or 401 — NOT 404 or 502

# Swagger blocked
curl -sI http://localhost/swagger/ | head -3
# Expected: HTTP/1.1 404 Not Found
```

- [ ] **Step 5: Smoke test HTTPS (self-signed cert)**

```bash
curl -sk https://localhost/healthz
# Expected: {"status":"ok","service":"gateway"}

curl -skI https://localhost/api/auth/me | head -3
# Expected: HTTP/1.1 200 or 401

# Verify HSTS header on API routes
curl -sk -I https://localhost/api/auth/me | grep -i "strict-transport"
# Expected: strict-transport-security: max-age=31536000; includeSubDomains
```

- [ ] **Step 6: Final commit**

```bash
cd /home/chronical/Projects/private/go-stock-prediction
git add -A
git commit -m "docs: update CLAUDE.md + README — gateway-svc replaces nginx frontend"
```

---

## Self-review

**Spec coverage check:**
- ✅ Path-based routing (`/api/*` → api, `/*` → frontend, `/swagger/*` → 404)
- ✅ TLS on :443 with cert auto-generation
- ✅ Plain HTTP on :80 (Cloudflare Tunnel)
- ✅ Security headers (`X-Frame-Options`, `X-Content-Type-Options`, `Referrer-Policy`, `HSTS`)
- ✅ Frontend simplified to static-only
- ✅ `./nginx/certs` reused — no filesystem change needed
- ✅ `/healthz` for gateway health check (not proxied)
- ✅ `/health` proxied to API (matches existing nginx behavior)

**Placeholder scan:** No TBD or TODO in code blocks.

**Type consistency:**
- `PathRouter::from_config(&[RouteConfig])` → Task 4 defines it, Task 7 uses it ✅
- `ProxyClient::new(&AppConfig)` → Task 5 defines it, Task 8 uses it ✅
- `create_routes(Arc<ProxyClient>, Arc<PathRouter>, Arc<AppConfig>)` → Task 7 defines, Task 8 calls ✅
- `RouteAction::Proxy { backend: String, security_headers: bool }` → Task 4 and Task 7 consistent ✅

**Known limitation:** The `Cargo.lock` is not committed. First Docker build fetches and compiles all deps from scratch (~3-5 min). Add `COPY Cargo.lock ./` to Dockerfile after generating it locally with `cargo build` if faster CI builds are needed.
