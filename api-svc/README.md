# api-svc — Go HTTP API Backend

The HTTP edge of the Go Stock Prediction system. A thin Go backend that serves
**all `/api/*` endpoints** on port **8118**. It owns no auth/RBAC logic and runs
no ML — instead it:

- **Validates JWTs locally** using a shared `JWT_SECRET` (HMAC), without calling
  the auth service on every request.
- **Forwards** auth / user / market-group operations to the Java **auth-svc**
  over gRPC.
- **Calls** the Python **prediction-svc** over gRPC to trigger crawlers,
  training, predictions, reconciliation, backtests, and simulations.
- **Reads the database directly** (TimescaleDB / PostgreSQL) for all data-query
  endpoints (prices, predictions, training history, dashboard, monitoring,
  simulation leaderboard, etc.).

## Tech stack

- **Go** (built with `golang:1.25-alpine`), Go module path `go-stock-prediction`.
- Standard-library `net/http` with Go 1.22+ pattern-based routing
  (`mux.HandleFunc("GET /api/...")`).
- **GORM v2** + PostgreSQL driver for direct DB reads.
- **gRPC** clients for prediction-svc and auth-svc.
- **golang-jwt/jwt/v5** for local JWT validation.
- **zerolog**-based logger (`pkg/logger`), **godotenv** for `.env` loading,
  **swaggo** for Swagger generation.

## Service boundaries

| Concern | Owner |
|---------|-------|
| HTTP `/api/*` surface, JSON responses, routing, CORS | **api-svc** (this service) |
| JWT validation (local, shared secret) | **api-svc** |
| Login, JWT issuance, user CRUD, market groups, RBAC, bcrypt | delegated to **auth-svc** (Java, gRPC `:8120`) |
| Crawling, ML training, prediction, reconcile, backtest, simulation | delegated to **prediction-svc** (Python, gRPC `:8119`) |
| Time-series data persistence | **db** (`timescaledb`, PG16) — read directly by api-svc |
| Scheduled DB backup (`pg_dump`) | **api-svc** ([`backup_scheduler.go`](pkg/server/backup_scheduler.go)) |

## Directory structure

```
api-svc/
├── cmd/main.go                 # Entry point — startup sequence + graceful shutdown
├── docs/                       # Generated Swagger spec (swagger.json/.yaml, docs.go) — do not edit by hand
├── proto/                      # Generated gRPC stubs (do not edit by hand)
│   ├── prediction/             # prediction-svc service stubs (shared .proto)
│   └── auth/                   # auth-svc service stubs
├── go.mod / go.sum
└── pkg/
    ├── config/                 # Load .env + env vars into a Config struct
    │   ├── init.go             #   InitConfig() — builds Config from env
    │   └── config.go           #   Getters: GetServerConfig, GetGRPCConfig, GetAuthGRPCConfig, ...
    ├── logger/                 # zerolog setup
    ├── server/                 # HTTP layer — handlers, router, middleware
    │   ├── server.go           #   StartHTTPServer() — wires middleware chain, http.Server
    │   ├── router.go           #   addHandler() — registers every route + auth wrapper
    │   ├── middleware_cors.go  #   CORSMiddleware (allowed-origins + security headers)
    │   ├── middleware_jwt.go   #   JWTMiddleware + AuthRequired/AdminRequired/MarketRequired + claim helpers
    │   ├── middleware_auth.go  #   APIKeyMiddleware (X-API-Key — currently unused in chain)
    │   ├── helper.go           #   requireGRPCClient(), ResponseSuccess/ResponseError, etc.
    │   ├── cache.go            #   small in-process response cache (e.g. monitoring)
    │   ├── backup_scheduler.go #   Start/StopBackupScheduler — DB-backed daily pg_dump
    │   ├── api_auth.go         #   POST /api/x/grant (login, X-Token header), GET /api/auth/me, PUT /api/auth/password (→ auth-svc)
    │   ├── api_users.go        #   /api/users* (→ auth-svc)
    │   ├── api_market_groups.go#   /api/market-groups* (→ auth-svc)
    │   ├── api_gold*.go        #   Gold prices + predictions (direct DB read)
    │   ├── api_nasdaq*.go      #   NASDAQ prices + predictions
    │   ├── api_crypto*.go      #   Crypto prices + predictions
    │   ├── api_sp500*.go       #   S&P 500 prices + predictions
    │   ├── api_markets.go      #   /api/markets/{key}/{predictions,training,session-stats}
    │   ├── api_training_*.go   #   /api/training/{status,history,algorithms,metrics,{id}}
    │   ├── api_direction_accuracy.go # /api/predictions/direction-accuracy
    │   ├── api_dashboard_stats.go    # /api/dashboard/stats
    │   ├── api_monitoring.go   #   /api/monitoring/overview (cached)
    │   ├── api_pipeline_reports.go / api_pipeline_stream.go # pipeline reports + SSE stream
    │   ├── api_schedules.go    #   /api/schedules (cron config)
    │   ├── api_simulation.go / api_session_stats.go # bot simulation data
    │   ├── api_backup.go       #   /api/backups*, /api/trigger/backup
    │   └── api_trigger_*.go    #   POST /api/trigger/* — each forwards to prediction-svc via gRPC
    ├── grpc/
    │   ├── client/client.go        # Prediction-svc gRPC client singleton (Init/GetClient/Close)
    │   └── authclient/client.go    # Auth-svc gRPC client singleton (Init/GetClient/Close)
    ├── store/
    │   ├── repository/          # DatabaseStore interface(s) — the only DB contract handlers use
    │   ├── postgres/            # PostgreSQL/TimescaleDB implementation (GORM) — active driver
    │   └── mysql/               # Legacy MySQL implementation (kept for fallback)
    ├── models/
    │   ├── models_db/           # GORM structs (table mappings)
    │   ├── models_api/          # JSON response DTOs
    │   ├── models_config/       # Config / ServerConfig / GRPCConfig / DatabaseConfig structs
    │   └── models_svc/          # Service-layer types
    ├── service/predict/registry/ # Algorithm metadata registry (for /api/training/algorithms)
    │   ├── registry.go          #   AlgorithmDef + Register()/All()
    │   └── algorithms.go        #   The one file to edit when adding algorithm metadata
    ├── utils/                   # env, cron constants, helpers
    └── testutil/                # Test helpers (fixtures, http)
```

### Conventions

- **Repository pattern.** Handlers never touch GORM directly — they go through
  the `DatabaseStore` interface in
  [`pkg/store/repository/`](pkg/store/repository/). `repository.GetSingleton()`
  returns the initialized global store. The active implementation
  (`postgres` vs legacy `mysql`) is chosen by `DB_DRIVER` in
  [`store.go`](pkg/store/repository/store.go).
- **One file per endpoint group.** Each `api_<topic>.go` in `pkg/server/` holds a
  related set of handlers, all registered in
  [`router.go`](pkg/server/router.go).
- **Generated code is not edited by hand** — `proto/*` (run `protoc`) and
  `docs/*` (run `swag init`).

## Startup flow

See [`cmd/main.go`](cmd/main.go). On boot the service runs, in order:

1. `config.InitConfig()` — loads `.env` (if present) and reads env vars into a
   `Config`. In Docker, env vars are injected directly, so a missing `.env` is
   fine.
2. **Timezone** — sets `time.Local = Asia/Ho_Chi_Minh` (ICT). All `time.Now()`
   and time comparisons use ICT (matches the "ICT-at-rest" DB convention).
3. `logger.Init()` — initializes the zerolog logger.
4. `repository.Init()` — connects to the database (PostgreSQL/TimescaleDB) for
   read queries.
5. `authclient.Init(config.GetAuthGRPCConfig())` — opens the gRPC client to
   **auth-svc** (`auth-svc:8120`).
6. `grpcclient.Init(config.GetGRPCConfig().ClientTarget)` — opens the gRPC client
   to **prediction-svc** (`prediction-svc:8119`).
7. `server.StartBackupScheduler(...)` — starts the DB-backed daily backup
   scheduler (seeds/polls the `daily_backup` cron row, runs `pg_dump`).
8. `go server.StartHTTPServer()` — starts the HTTP server on `:8118` in a
   goroutine.
9. Blocks on `SIGTERM`/`SIGINT`/`Interrupt`, then performs graceful shutdown:
   `StopBackupScheduler()` → `authclient.Close()` → `grpcclient.Close()`.

## Request flow & auth

### Middleware chain

The HTTP server wraps the route mux as (`pkg/server/server.go`):

```
CORSMiddleware( JWTMiddleware( mux ) )
```

- **CORSMiddleware** — echoes `Access-Control-Allow-Origin` only for origins in
  the `ALLOWED_ORIGINS` env list, sets allow-methods/headers, adds security
  headers (`X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`,
  `Referrer-Policy`), and short-circuits `OPTIONS` with `204`.
- **JWTMiddleware** — **non-blocking.** If an `Authorization: Bearer <token>`
  header is present and the token validates against `JWT_SECRET` (HMAC), the
  claims are injected into the request context. Requests without a token (or with
  an invalid one) still proceed — as *unauthenticated*. Enforcement happens per
  route via the wrappers below.

> Note: an `APIKeyMiddleware` exists in
> [`middleware_auth.go`](pkg/server/middleware_auth.go) but is **not** wired into
> the active chain; trigger protection is done via JWT/role wrappers.

### Per-route auth wrappers

Defined in [`middleware_jwt.go`](pkg/server/middleware_jwt.go):

- `AuthRequired(handler)` — 401 unless valid JWT claims are present.
- `AdminRequired(handler)` — 401 if unauthenticated; 403 unless `role` is
  `admin` or `super_admin`. Wraps **all** `/api/trigger/*` and admin-only
  endpoints.
- `MarketRequired("KEY")(handler)` — checks the JWT `accessible_markets` claim
  (e.g. `["GOLD","NASDAQ"]`) and returns 403 if the market `KEY` isn't present.
  `super_admin` always bypasses the check. Used to gate per-market data endpoints
  (`/api/gold/*`, `/api/nasdaq/*`, `/api/crypto/*`, `/api/sp500/*`).

Internal helpers: `getClaims(r)`, `requireAuth(w,r)`, `requireAdmin(w,r)`,
`getAccessibleMarkets(claims)`, `isSuperAdmin(claims)`, and `callerFromClaims`
(packs `user_id`/`role` into a `CallerMeta` proto forwarded to auth-svc for
permission checks).

### RBAC roles

JWTs are issued by **auth-svc** and carry `sub`, `role`, `user_id`, and
`accessible_markets`. Three roles:

- **super_admin** — full access to all markets; not manageable by admins.
- **admin** — manages users and market groups; market access per their groups.
- **user** — read-only access to assigned markets only.

### Public vs protected routes

- **Public** (no auth): `/api/dashboard/stats`, `/api/training/*`,
  `/api/predictions/direction-accuracy`, simulation read endpoints, monitoring
  is `AuthRequired`. (See `router.go` for the authoritative list.)
- **Auth-only**: backups list/download, schedules, pipeline reports, monitoring,
  per-market paginated `/api/markets/{key}/*`.
- **Market-gated**: per-market price/prediction endpoints (`AuthRequired` +
  `MarketRequired`).
- **Admin-only**: every `/api/trigger/*`, user management, market groups, backup
  trigger/delete.

## Build & run

### Local

From the repo root (the Makefile wraps the `cd api-svc`):

```bash
make build          # -> cd api-svc && go build -o api-server ./cmd
# or directly:
cd api-svc && go build -o api-server ./cmd
./api-server
```

Running locally needs a reachable DB, auth-svc, and prediction-svc (point
`GRPC_TARGET` / `AUTH_GRPC_TARGET` / `POSTGRES_HOST` at them via env or `.env`).

### Docker

Compose lives in `deploy/` and the `.env` stays at the repo root. Run from the
**repo root**:

```bash
# Whole stack
docker compose --env-file .env -f deploy/docker-compose.yaml up -d
# or: make up

# Just this service (and its deps)
docker compose --env-file .env -f deploy/docker-compose.yaml up -d api-svc
```

Inside the compose network the service is reachable as `api-svc:8118`; it is not
published directly — the Rust `gateway-svc` proxies `/api` and `/health` to it.
The image (`deploy/api-svc.Dockerfile`) is a multi-stage build that also installs
`postgresql-client` (for `pg_dump` backups) and mounts the `backup_data` volume
at `/backups`. Healthcheck: `GET /health/simple`.

### Swagger

Annotations live on each handler (`@Summary`, `@Tags`, `@Param`, `@Success`,
`@Router`). After editing them, regenerate:

```bash
cd api-svc && swag init -g cmd/main.go -o docs/
# or: make swagger
```

Swagger UI is served at `http://localhost:8118/swagger/` (via gateway in Docker).

## Environment variables

Read in [`pkg/config/init.go`](pkg/config/init.go); values below match the
`api-svc` service in `deploy/docker-compose.yaml`.

| Variable | Default | Purpose |
|----------|---------|---------|
| `SERVER_PORT` | `8118` | HTTP listen port |
| `SERVER_HOST` | `0.0.0.0` | HTTP bind host |
| `SERVER_NAME` | `go-stock-prediction` | Service name (logging) |
| `GRPC_TARGET` | `localhost:8119` | prediction-svc gRPC address (Docker: `prediction-svc:8119`) |
| `AUTH_GRPC_TARGET` | `localhost:8120` | auth-svc gRPC address (Docker: `auth-svc:8120`) |
| `GRPC_SERVER_PORT` | `8119` | (config field; informational) |
| `JWT_SECRET` | `change-me-in-production` | **Shared** HMAC secret for local JWT validation — must match auth-svc |
| `ADMIN_USERNAME` | `admin` | Dashboard admin username (informational; auth owned by auth-svc) |
| `ADMIN_PASSWORD` | `admin123` | Dashboard admin password (informational) |
| `API_KEY` | `""` | Optional X-API-Key value (middleware not active in chain) |
| `ALLOWED_ORIGINS` | _(empty)_ | Comma-separated CORS allow-list origins |
| `DB_DRIVER` | `mysql` (set `postgresql` in compose) | Selects DB implementation |
| `POSTGRES_HOST` | `localhost` (Docker: `db`) | DB host |
| `POSTGRES_PORT` | `5432` | DB port |
| `POSTGRES_USER` | `postgres` | DB user |
| `POSTGRES_PASSWORD` | `123` | DB password |
| `POSTGRES_DB` | `go_stock_prediction` | DB name |
| `POSTGRES_DEBUG` | `false` | GORM debug logging |
| `LOG_LEVEL` | `DEBUG` | App log level |
| `DB_LOG_LEVEL` | `DEBUG` | DB log level |
| `BACKUP_DIR` | `/backups` | Directory for `pg_dump` backup files |

## Tests

Run from the repo root (wraps `cd api-svc`):

```bash
make test            # cd api-svc && go test ./... -v -count=1
# or directly:
cd api-svc && go test ./...

make test-unit       # adds -short to skip integration tests
make test-coverage   # writes coverage.html
make vet             # go vet ./...
```

Conventions:

- Test files live in the same package: `foo.go` → `foo_test.go`.
- Handler tests use a **recover / mock pattern** rather than a live DB — see
  `pkg/server/*_test.go` (e.g. `api_training_algorithms_test.go`,
  `api_monitoring_test.go`, `helper_test.go`).
- Integration-style tests guard with `if testing.Short() { t.Skip() }`.

## Adding a new API endpoint

1. **Create the handler.** Add `pkg/server/api_<topic>.go` with a handler
   `func XxxHandler(w http.ResponseWriter, r *http.Request)`. For DB reads, go
   through `repository.GetSingleton()` (a `DatabaseStore` method) — never raw
   GORM. For triggers, call `requireGRPCClient(w)` then the prediction-svc gRPC
   client; for auth/user operations, call the `authclient` gRPC client.
2. **Register the route** in [`pkg/server/router.go`](pkg/server/router.go),
   choosing the right wrapper:
   - public → bare `mux.HandleFunc(...)`
   - logged-in → `AuthRequired(Handler)`
   - market-scoped → `AuthRequired(MarketRequired("KEY")(Handler))`
   - admin/trigger → `AdminRequired(Handler)`
   Add a matching line to `logRegisteredRoutes()` if you want it logged at boot.
3. **Add Swagger annotations** (`@Summary`, `@Tags`, `@Param`, `@Success`,
   `@Router`, and `@Security BearerAuth` for protected routes) on the handler.
4. **Regenerate Swagger**: `cd api-svc && swag init -g cmd/main.go -o docs/`
   (or `make swagger`). Do not hand-edit `docs/`.
5. **(If a new gRPC call is needed)** add the RPC to the relevant `.proto`, then
   regenerate both Go and Python stubs (see repo-root `CLAUDE.md`).
6. Add a `*_test.go` following the recover/mock pattern.
