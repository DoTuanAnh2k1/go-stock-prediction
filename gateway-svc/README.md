# Gateway Service (`gateway-svc`)

The single public entry point for the **go-stock-prediction** stack. A Rust
[Axum](https://github.com/tokio-rs/axum) reverse proxy that terminates TLS and
routes incoming requests to the internal backends by URL path prefix. It is the
only service exposed to the outside world — every other service (API, Auth,
Prediction, Web) listens on the internal Docker network and is reached through
this gateway.

## 1. Overview

- Binds **`:80`** (HTTP) and **`:443`** (HTTPS) on `0.0.0.0`.
- Performs **TLS termination** on `:443` using a certificate loaded from disk
  (self-signed by default — see [TLS](#4-tls)).
- Routes by **longest-prefix path matching** to the renamed backend services,
  or returns `404` for blocked prefixes.
- Adds OWASP security headers, a request ID, structured access logs, and an
  optional per-IP rate limiter.

When TLS is enabled, the gateway runs HTTP and HTTPS servers concurrently
(`tokio::join!`) over the **same** Axum router; both ports serve identical
routes. When `tls.enabled = false`, only the HTTP listener is started.

### Tech stack

| Component | Crate |
|-----------|-------|
| Web framework | `axum` 0.8 |
| TLS server | `axum-server` 0.7 (feature `tls-rustls`) + `rustls` 0.23 (`ring` provider) |
| Upstream HTTP client | `reqwest` 0.12 (`rustls-tls`) |
| Async runtime | `tokio` 1.x (`full`) |
| Rate limiting | `governor` 0.6 (GCRA, keyed by IP) |
| Config | `config` + `serde_yaml` |
| Logging | `tracing` + `tracing-subscriber` (JSON or pretty) |
| Request IDs | `uuid` v4 |

The release profile uses `lto = true`, `codegen-units = 1`, `opt-level = 3`.

## 2. Routing

Routing is configured in [`config.yaml`](config.yaml) under `routes`. Each entry
is `{ prefix, action, backend?, security_headers }`. At startup `PathRouter::from_config`
builds the route table and **sorts prefixes longest-first**, so the most specific
prefix wins (e.g. `/api/...` matches the `/api` route, not the catch-all `/`).

A request path is matched against `starts_with(prefix)` and produces a
`RouteAction`:

- **`RouteAction::Block`** → respond `404 Not Found`. A path that matches **no**
  route also returns `404` (same branch).
- **`RouteAction::Proxy { backend, security_headers }`** → forward the full
  path + query string to `backend`, then optionally apply security headers to
  the upstream response.

### Current route table (longest-prefix wins)

| Prefix | Action | Backend | Security headers |
|--------|--------|---------|------------------|
| `/swagger` | block (`404`) | — | — |
| `/api` | proxy | `http://api-svc:8118` | **on** |
| `/health` | proxy | `http://api-svc:8118` | off |
| `/` | proxy | `http://web-svc:3000` | off |

> The backend hostnames are the post-restructure DNS names: the Go API backend
> is `api-svc` and the React SPA is `web-svc`. Both resolve over the internal
> Docker Compose network.

The catch-all `proxy_handler` (registered as the router `fallback`) reads the
`PathRouter` from `AppState`, dispatches by prefix, and on a proxy hit calls
`ProxyClient::forward_request`. Hop-by-hop request headers (`host`, `connection`,
`content-length`, `transfer-encoding`) are stripped before forwarding, and
`transfer-encoding` / `content-encoding` are stripped from the upstream
response. On upstream failure the client returns `502 Bad Gateway` with a JSON
body `{ "error": ..., "status": 502 }`.

## 3. Gateway-local endpoints (not proxied)

These are served by the gateway itself and never forwarded upstream:

| Method | Path | Response |
|--------|------|----------|
| `GET` | `/healthz` | `200` `{ "status": "ok", "service": "gateway" }` |
| `GET` | `/readyz` | `200` `{ "status": "ready", "service": "gateway" }` |

The Docker `HEALTHCHECK` curls `http://localhost:80/healthz`.

## 4. TLS

TLS is terminated at the gateway. Cert/key paths come from `server.tls` in
`config.yaml`:

```yaml
tls:
  enabled: true
  cert_path: "/etc/gateway/certs/cert.pem"
  key_path:  "/etc/gateway/certs/key.pem"
```

[`docker-entrypoint.sh`](docker-entrypoint.sh) runs first (as root) and, **if the
cert/key are absent**, generates a self-signed RSA-2048 cert valid for 10 years
(`CN=localhost`, `O=go-stock-prediction`) via `openssl`. It then `chown`s the
certs to the unprivileged `gateway` user (uid/gid `1001`), `chmod 600`s the key,
and drops privileges with `setpriv` before `exec`-ing the binary as PID 1.

The cert directory `/etc/gateway/certs` is **bind-mounted from `gateway-svc/certs/`**
on the host, so a generated (or replaced) cert persists across container
restarts. `gateway-svc/certs/.gitignore` keeps the actual `.pem` files out of
version control.

### Real certificates

The self-signed cert triggers browser warnings — fine for local/dev, not for
production. To install a real cert, drop a valid `cert.pem` + `key.pem` into
`gateway-svc/certs/` (they take precedence — the entrypoint only generates when
files are missing). Helper scripts live at the **repo root** under `scripts/`:

| Script | Use |
|--------|-----|
| `scripts/gen-self-signed.sh` | self-signed cert (same as the entrypoint fallback) |
| `scripts/gen-letsencrypt.sh` | Let's Encrypt cert |
| `scripts/gen-cloudflare-origin.sh` | Cloudflare Origin CA cert |
| `scripts/gen-tailscale-cert.sh` | Tailscale-issued cert |

## 5. Middleware

Layers are applied in `create_routes` and wrap **both** the gateway-local and
proxy routes:

- **Request ID** (`middleware/request_id.rs`) — reuses an incoming
  `X-Request-ID` header if present, otherwise mints a UUID v4. Stored in request
  extensions and echoed back on the response `X-Request-ID` header.
- **Logging** (`middleware/logging.rs`) — structured access log via `tracing`.
  Logs method, URI, client IP (from `X-Forwarded-For`), request ID, response
  status, and duration in ms. Successes/redirects log at `info`; everything else
  at `warn`.
- **Security headers** (`middleware/security_headers.rs`) — applied **only** on
  proxy routes flagged `security_headers: true` (currently just `/api`). Adds
  `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`,
  `Referrer-Policy: strict-origin-when-cross-origin`, and
  `Strict-Transport-Security: max-age=31536000; includeSubDomains`.
- **Rate limiting** (`middleware/rate_limit.rs`) — a `governor` GCRA limiter
  keyed by client IP, configured by `middleware.rate_limit` in `config.yaml`.
  When `enabled`, allows `max_requests` tokens per `window_secs` (with burst up
  to `max_requests`); over-limit requests get `429 Too Many Requests`. When
  disabled, the router is returned unchanged. (Requires the server to be started
  with connection info, which `main.rs` does via
  `into_make_service_with_connect_info::<SocketAddr>()`.)

## 6. `config.yaml` structure

```yaml
server:                       # bind + TLS
  host: "0.0.0.0"
  http_port: 80
  https_port: 443
  tls:
    enabled: true
    cert_path: "/etc/gateway/certs/cert.pem"
    key_path:  "/etc/gateway/certs/key.pem"
  shutdown_timeout: 30

http:                         # reqwest upstream client tuning
  version: "auto"
  pool:
    max_idle_per_host: 20
    idle_timeout: 90          # seconds
    connection_timeout: 10    # seconds
  request:
    timeout: 600              # seconds (per upstream request)
    max_retries: 0
    retry_delay: 1            # seconds between retries

routes:                       # longest-prefix routing (see §2)
  - { prefix: "/swagger", action: "block" }
  - { prefix: "/api",     action: "proxy", backend: "http://api-svc:8118", security_headers: true }
  - { prefix: "/health",  action: "proxy", backend: "http://api-svc:8118", security_headers: false }
  - { prefix: "/",        action: "proxy", backend: "http://web-svc:3000", security_headers: false }

middleware:
  rate_limit:
    enabled: true
    max_requests: 60
    window_secs: 1

logging:
  level: "info"
  format: "json"              # "json" or pretty
  access_log: true
```

Config is loaded from `config.yaml` (cwd) or `/etc/gateway/config.yaml`,
whichever exists first. Values can be overridden by environment variables with
the `GATEWAY_` prefix (`_` separator). Logging is also influenced at runtime by
the `RUST_LOG` and `LOG_FORMAT` env vars in `main.rs`.

## 7. Directory structure

```
gateway-svc/
├── Cargo.toml                  # crate manifest (binary name: gateway)
├── config.yaml                 # runtime config — server/tls/http/routes/middleware/logging
├── docker-entrypoint.sh        # cert provisioning + privilege drop, then exec binary
├── certs/                      # bind-mounted TLS cert/key (gitignored)
└── src/
    ├── main.rs                 # entry point — install rustls provider → load config →
    │                           #   build PathRouter + ProxyClient → create_routes →
    │                           #   serve HTTP (+HTTPS when TLS enabled)
    ├── lib.rs                  # module declarations / re-exports
    ├── config/mod.rs           # AppConfig + section structs; load()/load_from() with serde defaults
    ├── router/mod.rs           # PathRouter, RouteAction (Block | Proxy); longest-prefix match
    ├── routes/
    │   ├── mod.rs              # create_routes() — wires health + fallback proxy + middleware layers
    │   ├── health.rs           # /healthz, /readyz handlers (gateway-local)
    │   └── proxy.rs            # AppState + proxy_handler (catch-all fallback)
    ├── proxy/
    │   ├── mod.rs              # re-export
    │   └── client.rs           # ProxyClient (reqwest) — header filtering, retries, ProxyError → 502
    ├── middleware/
    │   ├── mod.rs              # module decls + add_security_headers re-export
    │   ├── request_id.rs       # X-Request-ID injection (UUID v4)
    │   ├── logging.rs          # structured access log
    │   ├── security_headers.rs # OWASP headers (proxy routes flagged security_headers: true)
    │   └── rate_limit.rs       # per-IP GCRA rate limiter (governor)
    └── models/                 # shared types
```

> The vendored `gateway-svc/.cargo/` directory is build tooling and not part of
> the service source.

## 8. Build & run

### Local

```bash
cd gateway-svc
cargo build --release      # binary at target/release/gateway
cargo test                 # unit tests in config / router / request_id / security_headers / rate_limit
cargo run                  # runs with ./config.yaml
```

Running locally with the committed `config.yaml` will attempt to bind `:80`/`:443`
and load certs from `/etc/gateway/certs/` — adjust `config.yaml` (`tls.enabled`,
ports, cert paths) or use `GATEWAY_`-prefixed env vars for a dev setup.

### Docker / Compose

The Dockerfile now lives in the flat `deploy/` directory at
[`deploy/gateway-svc.Dockerfile`](../deploy/gateway-svc.Dockerfile). It is a
two-stage build: `rust:1.86-slim` compiles the release binary (with a dependency
caching layer), and a `debian:bookworm-slim` runtime stage ships the binary,
`config.yaml`, the entrypoint, plus `ca-certificates`, `openssl`, and `curl`.
The compose build context is `../gateway-svc` with `dockerfile: ../deploy/gateway-svc.Dockerfile`.

Run the whole stack from the **repo root**:

```bash
docker compose --env-file .env -f deploy/docker-compose.yaml up -d
```

Compose service key, `container_name`, and directory name are all `gateway-svc`.
The service exposes `:80` and `:443`, bind-mounts `gateway-svc/certs/` to
`/etc/gateway/certs`, and depends on `api-svc` and `web-svc`.
```
