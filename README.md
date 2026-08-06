# go-stock-prediction

Financial asset price prediction system with RBAC — crawls Gold SJC/XAU, NASDAQ, Crypto BTC/ETH/SOL, and S&P 500, runs 11 ML algorithms, and displays results in a web dashboard with role-based market access control.

> ### 🎓 CKAD Capstone — start here / bắt đầu ở đây
> - **How to deploy & verify (EN + VI):** [VERIFY.md](VERIFY.md) — cluster up → `scripts/build.sh` → `scripts/deploy.sh` → `scripts/smoke-test.sh` → `scripts/run-labs.sh`
> - **§4 requirement → resource → verify command:** [docs/ckad-checklist.md](docs/ckad-checklist.md)
> - **The graded spec:** [deploy/k8s/ckad-labs/capstone-requirements.md](deploy/k8s/ckad-labs/capstone-requirements.md)
> - **Day 1–5 labs:** [deploy/k8s/ckad-labs/](deploy/k8s/ckad-labs/) (`day_N/run-dayN.sh` + `lab.md`, `DEMO.md`) — or run all: `./scripts/run-labs.sh`
> - **Deploy scripts:** [scripts/](scripts/) · **Architecture/impl:** [DESIGN.md](DESIGN.md) · [IMPLEMENTATION.md](IMPLEMENTATION.md)
>
> Verified live on **kind Kubernetes v1.35.0** — see the [Kubernetes / CKAD](#kubernetes--ckad-capstone) section below.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          Docker Compose                                  │
│                                                                          │
│  Browser ──► gateway-svc :80/:443 (Rust) ──► api-svc :8118 (Go)       │
│                      │                              │                    │
│                      │                              ├──gRPC──► auth-svc :8120
│                      │                              │       (Java Spring)│
│                      │                              │                    │
│                      │                              └──gRPC──► prediction-svc :8119
│                      │                                         (Python)  │
│                      │                                              │    │
│                      └──► web-svc :3000 (internal)                │    │
│                              (React + Vite, static nginx)          │    │
│                                                                    │    │
│              TimescaleDB :5432 ◄───────────────────────────────────┘    │
│              (PostgreSQL 16)                                             │
│                                                                          │
│  SSH ──► cli-svc :2345 (Go, wish+bubbletea) ──HTTP──► gateway-svc :80  │
│  pgAdmin 127.0.0.1:8081                                                  │
│                                                                          │
│  service-mgt :8121 (optional, gRPC) — registry/discovery for            │
│    api-svc, auth-svc, prediction-svc, gateway-svc when                  │
│    SERVICE_MGT_ENABLED=true (default: false, static endpoints used)      │
└─────────────────────────────────────────────────────────────────────────┘
```

## Services

| Service (dir) | Tech | Port | Role |
|---------------|------|------|------|
| `gateway-svc/` | Rust, Axum 0.8, rustls | `:80` / `:443` (public) | TLS termination; longest-prefix routing: `/swagger` → block, `/api` → `api-svc:8118`, `/health` → `api-svc:8118`, `/` → `web-svc:3000`; gateway-local `/healthz` and `/readyz` |
| `api-svc/` | Go 1.25+, net/http, gRPC, GORM v2, ZeroLog | `:8118` (internal) | HTTP API; thin auth proxy to `auth-svc` via gRPC; connects to `market_db` as read-only role `api_svc`; triggers `prediction-svc` via gRPC; backup scheduler |
| `auth-svc/` | Java 21, Spring Boot 3, gRPC, Flyway, bcrypt | `:8120` (internal) | Owns all RBAC: login, JWT generation (HMAC256, 24 h), user CRUD, market groups; Flyway V1: auth + RBAC tables in `auth_db`; V2: `full_name`/`email`/`phone`; V3: command RBAC; seeds super_admin from env `SUPER_ADMIN_USERNAME`/`SUPER_ADMIN_PASSWORD` (fail-fast if blank/weak) |
| `prediction-svc/` | Python 3.12, PyTorch, statsmodels, LightGBM, XGBoost, scikit-learn, APScheduler, SQLAlchemy | `:8119` (internal) | gRPC service: crawling, 13 ML algorithms, training, cron scheduler |
| `web-svc/` | React, TypeScript, Vite, nginx | `:3000` (internal) | Static SPA served by nginx; accessed only through `gateway-svc` |
| `cli-svc/` | Go 1.26, charmbracelet/wish + bubbletea, go-pretty | `:2345` (public, SSH) | Interactive SSH shell for headless servers; renders API data as tables; `get`/`set`/`update`/`delete` verbs; per-command RBAC enforced client-side; calls the API through `gateway-svc` at a static `API_BASE_URL` (no service discovery) |
| `service-mgt/` | Go 1.25, gRPC, GORM v2, ZeroLog | `:8121` (internal) | Central service registry/discovery: services register on boot, renew a lease via heartbeat (push/lease-TTL), and resolve peers via Discover; write-through cache (Postgres `service_instances` = source of truth, in-memory cache = read layer). Disabled by default (`SERVICE_MGT_ENABLED=false`). |
| `db` | TimescaleDB (PostgreSQL 16) | `:5432` (internal) | One instance, three isolated databases (`auth_db`, `market_db`, `registry_db`); each service uses a dedicated least-privilege role; schema init via `deploy/db-init/` scripts |
| `pgadmin` | pgAdmin 4 | `127.0.0.1:8081` | PostgreSQL web administration UI |

Internal DNS (between containers): `api-svc:8118`, `prediction-svc:8119`, `auth-svc:8120`, `service-mgt:8121`, `web-svc:3000`, `db:5432`. The `cli-svc` SSH port `2345` is exposed directly (the gateway speaks HTTP only); `cli-svc` reaches the API at `http://gateway-svc/api`.

## Features

- **RBAC with market groups:** Three roles — `super_admin` (full access), `admin` (manage users and market groups), `user` (access only assigned markets). JWT includes `accessible_markets` claim; sidebar hides inaccessible market tabs.
- **Data collection:** Gold SJC/XAU/USD every hour; NASDAQ and S&P 500 every hour weekdays (skips NYSE holidays); Crypto BTC/ETH/SOL every 2 hours.
- **13 ML algorithms:** Moving Average, EMA/MACD, LSTM (PyTorch), GRU (PyTorch), ARIMA-GARCH, EGARCH, SARIMA, LightGBM (Optuna tuned), XGBoost (Optuna tuned), Random Forest, Ensemble, RL DQN (Dueling Double-DQN), Transformer (PatchTST-lite).
- **Observability (15-factor #14):** OpenTelemetry distributed tracing across all 6 services (gateway→api-svc→{auth-svc, prediction-svc}) → OTLP → Tempo; Prometheus metrics (`/metrics` per service + domain metrics like `predictions_total`, `crawl_total`, `direction_accuracy`); Grafana + Tempo dashboards in a separate `observability` namespace.
- **Request correlation (trace-log):** every request carries an `X-Request-ID` propagated through all 6 services (HTTP header + gRPC metadata) and stamped into each service's structured logs for end-to-end log correlation.
- **Walk-forward backtest:** Historical backtesting via `POST /api/trigger/historical-backtest`.
- **Automated training:** Per-market weekly training jobs (Sunday 3–7 AM).
- **Daily reconcile:** 6 AM updates `direction_correct` for past predictions.
- **Dynamic cron schedules:** DB-backed, editable live via Settings page — no restart needed.
- **Pipeline monitoring:** `/api/monitoring/overview` — crawl freshness, per-algo prediction counts, bot win/loss stats (JWT required, 30 s cache).
- **Trading simulation:** Bot leaderboard, per-bot trades and portfolio snapshots, live-step and backtest modes.
- **Pipeline reports:** Per-pipeline run records with step-level status, stored 7 days (`GET /api/pipeline-reports`).
- **Auto backup:** Daily `pg_dumpall` (superuser) at 3 AM — captures all three databases plus roles in one archive. In compose: run by `ofelia` on the `db` container. In k8s: dedicated CronJob (`cronjob-backup.yaml`). `api-svc` is not involved when `BACKUP_SCHEDULER_ENABLED=false`.
- **Interactive CLI over SSH (`cli-svc`):** `ssh <user>@<host> -p 2345` (dashboard credentials) opens a shell that renders API data as tables. Four verbs `get`/`set`/`update`/`delete`, tab-completion, multi-session.
- **Command RBAC:** Admins declare commands (a cli handler + fixed args), group them, and assign users to groups — a user may execute only the commands in their groups (`super_admin` runs all). Managed from the web dashboard (`/admin/commands`, `/admin/command-groups`) or the CLI. RBAC lives in `auth-svc` (Flyway V3); `cli-svc` enforces it client-side.
- **Service discovery (optional, `SERVICE_MGT_ENABLED`):** When enabled, `api-svc`, `auth-svc`, `prediction-svc`, and `gateway-svc` register with `service-mgt` and resolve peers dynamically (client-side discovery, push lease-TTL heartbeat). Disabled by default — services use static endpoints. Always falls back to static targets if the registry is unreachable. `cli-svc` (HTTP via gateway) and `web-svc` (static nginx) do not register.
- **Request correlation ID (`X-Request-ID`):** Every log line carries a `request_id` (UUID v4) so a single transaction can be traced across all six services with `grep request_id=<id>`. The gateway mints/forwards `x-request-id`; each service reads it (HTTP header or gRPC metadata), binds it to its logger (Go `logger.Ctx(ctx)`, Java SLF4J MDC → `%X{requestId}`, Python `structlog.contextvars`), and re-injects it on outbound gRPC/HTTP calls. Background cron jobs mint their own id per run. Independent of OpenTelemetry tracing (`trace_id` still chains to Tempo separately).
- **Database-per-service:** One TimescaleDB instance, three isolated databases, each owned by a dedicated least-privilege login role. `auth_db` (role `auth_svc`) holds all RBAC tables — managed exclusively by Flyway inside `auth-svc`. `market_db` (role `prediction_svc` R/W; role `api_svc` read-limited) holds all price, prediction, simulation, pipeline, and cron data. `registry_db` (role `service_mgt`) holds only `service_instances`. `api-svc` connects to `market_db` as the `api_svc` role: full SELECT plus INSERT/UPDATE/DELETE only on `cron_schedules` and `sim_bots`. Inter-service auth/RBAC queries remain via gRPC as before. Init: `deploy/db-init/00-init-databases.sh` creates roles and databases; `deploy/db-init/schemas/10-market_db.sql` and `20-registry_db.sql` apply the DDL. Backup uses `pg_dumpall` (superuser) to capture all three databases plus roles in one archive.

## Repository Layout

```
go-stock-prediction/
├── api-svc/               # Go HTTP API backend (module: go-stock-prediction)
│   ├── cmd/               # Entry point (main.go)
│   ├── pkg/               # Handlers, store, models, config, gRPC clients, utils
│   ├── proto/             # .proto definitions + generated Go stubs (shared with prediction-svc)
│   └── docs/              # Generated Swagger spec (do not edit manually)
├── auth-svc/              # Java Spring Boot 3 gRPC auth/RBAC service
│   └── src/main/          # gRPC servicer, entities, Flyway migrations
├── prediction-svc/        # Python gRPC ML/crawlers/scheduler service
│   └── src/               # algorithms/, crawlers/, scheduler/, orchestrator/, database/, grpc_server/
├── web-svc/               # React + Vite + TypeScript SPA
│   └── src/               # pages/, components/, context/, i18n
├── gateway-svc/           # Rust Axum HTTP/HTTPS gateway
│   └── src/               # router/, routes/, proxy/, middleware/
├── cli-svc/               # Go SSH shell service (module: go-stock-prediction/cli-svc)
│   ├── internal/          # server/ (wish), shell/ (bubbletea), handlers/, client/, render/
│   └── keys/              # SSH host key (generated on first boot; gitignored)
├── service-mgt/           # Go gRPC service registry/discovery (module: go-stock-prediction/service-mgt)
│   ├── main.go            # Entry point — config → logger → DB → gRPC server → signal wait
│   ├── proto/registry/    # registry.proto + generated Go stubs
│   ├── client/            # Go client SDK — imported by api-svc (replace sibling module)
│   └── internal/          # config/, store/ (service_instances table), registry/ (lease/reaper), grpcserver/
├── deploy/                # All Dockerfiles and Compose files
│   ├── docker-compose.yaml
│   ├── docker-compose.test.yml      # uses database.sql (single-DB test harness only)
│   ├── db-init/                     # Database-per-service init
│   │   ├── 00-init-databases.sh     # Creates 3 DBs + 4 least-privilege roles + grants
│   │   └── schemas/
│   │       ├── 10-market_db.sql     # market_db DDL (hypertables, sim, pipeline, ...)
│   │       └── 20-registry_db.sql   # registry_db DDL (service_instances)
│   ├── api-svc.Dockerfile           # build context = repo root (imports service-mgt/)
│   ├── auth-svc.Dockerfile
│   ├── prediction-svc.Dockerfile    # build context = repo root (needs api-svc/proto/)
│   ├── web-svc.Dockerfile
│   ├── gateway-svc.Dockerfile
│   ├── cli-svc.Dockerfile           # build context = ../cli-svc (standalone, no service-mgt import)
│   └── service-mgt.Dockerfile       # build context = ../service-mgt
├── database.sql           # Legacy monolithic schema (kept for integration test harness only)
├── Makefile               # Root convenience targets (see below)
└── .env                   # Environment variables — stays at repo root
```

Each service directory contains its own `README.md` with service-specific documentation.

## Quick Start

### Prerequisites

- Docker and Docker Compose v2
- A `.env` file at the repo root (see [Configuration](#configuration) below)

### Deploy

All Dockerfiles and the Compose file live in `deploy/`. The `.env` file stays at the repo root and is passed via `--env-file`:

```bash
# Start all services (builds images on first run)
docker compose --env-file .env -f deploy/docker-compose.yaml up -d

# Equivalent shortcut via the root Makefile
make up

# Rebuild all images and restart
make reset
```

The `db` container auto-initialises on first startup via `deploy/db-init/00-init-databases.sh` (creates three databases and four least-privilege roles) plus the SQL files under `deploy/db-init/schemas/`. No manual import is needed. See [Database-per-service](#database-per-service) below.

### Access

| URL | Service |
|-----|---------|
| `http://localhost` or `https://localhost` | React dashboard (via Gateway) |
| `http://localhost/api/...` | REST API (via Gateway) |
| `http://localhost:8081` | pgAdmin (127.0.0.1 only) |
| `http://localhost:8118/swagger/` | Swagger UI (direct to `api-svc`, bypasses Gateway) |

**Default credentials:** Set via env `SUPER_ADMIN_USERNAME` / `SUPER_ADMIN_PASSWORD` — seeded by `auth-svc` on first startup (fail-fast if blank or weak; no hardcoded default).

### First-run data load

```bash
# Get a JWT token (use credentials from SUPER_ADMIN_USERNAME/SUPER_ADMIN_PASSWORD in .env)
TOKEN=$(curl -s -X POST http://localhost/api/x/grant \
  -H "Content-Type: application/json" \
  -H "X-Token: $(printf '%s' "${SUPER_ADMIN_USERNAME}:${SUPER_ADMIN_PASSWORD}" | base64)" \
  -d '{"request":""}' | jq -r '.token')

# Crawl initial data for all markets
curl -X POST http://localhost/api/trigger/gold-crawler   -H "Authorization: Bearer $TOKEN"
curl -X POST http://localhost/api/trigger/nasdaq-crawler -H "Authorization: Bearer $TOKEN"
curl -X POST http://localhost/api/trigger/crypto-crawler -H "Authorization: Bearer $TOKEN"
curl -X POST http://localhost/api/trigger/sp500-crawler  -H "Authorization: Bearer $TOKEN"

# Train models
curl -X POST http://localhost/api/trigger/train -H "Authorization: Bearer $TOKEN"

# Run walk-forward backtest (generates chart data)
curl -X POST "http://localhost/api/trigger/historical-backtest?train_window=30&step_size=6&market_key=ALL" \
  -H "Authorization: Bearer $TOKEN"
```

## Kubernetes / CKAD Capstone

Besides Docker Compose, the whole stack deploys to Kubernetes via **per-service independent Helm
charts** (`deploy/helm/<svc>/`, one chart per service — install/upgrade/rollback each on its own).
Shared partials live in the `common` library chart (backend charts pull it via `file://../common`);
namespace-wide governance (quota, pod-reader RBAC, PDBs, default-deny + multi-target NetworkPolicies)
lives in the `bootstrap` chart; batch jobs live in the `cronjobs` chart. This section is the CKAD
deliverable runbook. Item-by-item mapping of every mandatory requirement → resource → file → verify
command lives in [`docs/ckad-checklist.md`](docs/ckad-checklist.md).

### Prerequisites

- A cluster (kind `ckad` used here) — **v1.35+**, policy-capable CNI (kindnet) for NetworkPolicy
- `kubectl`, `helm` v3, `kustomize` (or `kubectl apply -k`)
- **ingress-nginx** installed (for Ingress N2/N3) and **metrics-server** (for HPA P4)
- A default StorageClass (dynamic PVC for db / MinIO / backups)

### Deploy (scripted)

```bash
./scripts/build.sh                 # build 7 images (imageTag from git SHA) + kind load into cluster "ckad"
./scripts/deploy.sh                # ns stock + db-init/db-schemas ConfigMaps + helm install (demo toggles ON)
./scripts/smoke-test.sh            # E2E: login → /api/version → monitoring (via gateway/ingress)
./scripts/run-labs.sh              # run all CKAD day 1–5 labs (handles NetworkPolicy toggle)
```

Full deploy + verify walkthrough (English + Vietnamese): **[VERIFY.md](VERIFY.md)**.

### Deploy (manual)

```bash
kubectl create namespace stock
# db-init and db-schemas ConfigMaps are NOT Helm-managed — deploy.sh creates them automatically:
#   kubectl create configmap db-init    -n stock --from-file=deploy/db-init/00-init-databases.sh
#   kubectl create configmap db-schemas -n stock --from-file=deploy/db-init/schemas/

# scripts/deploy.sh installs every chart in dependency order:
#   bootstrap → db → minio → service-mgt → prediction-svc → auth-svc → api-svc
#   → gateway-svc → web-svc → cli-svc → pgadmin → cronjobs
# (backend charts need `helm dependency build` first to vendor the `common` library)
./scripts/deploy.sh

# ...or install a single chart manually, e.g. api-svc:
helm dependency build deploy/helm/api-svc          # vendor `common` (backend charts only)
helm install api-svc deploy/helm/api-svc -n stock

# Production secrets (never committed): override each chart's placeholder secrets with -f.
# Each chart carries only the secrets it needs (imageTag/secrets are self-contained per chart).
helm upgrade api-svc deploy/helm/api-svc -n stock -f api-svc-secret.yaml
```

Toggles `quota`, `rbac`, `pdb` (on `bootstrap`) and `bluegreen` (on `api-svc`/`web-svc`) are **on by
default**; pgAdmin is now install-or-not (just skip `helm install pgadmin`). HPA, Ingress and
NetworkPolicy are **demo-gated off** (they need ingress-nginx / metrics-server or would cut idle
metrics) — enable them per chart for the graded cluster state:

```bash
helm upgrade api-svc     deploy/helm/api-svc     -n stock --set hpa.enabled=true
helm upgrade gateway-svc deploy/helm/gateway-svc -n stock --set hpa.enabled=true --set ingress.enabled=true
# the netpol graph is split — enable on BOTH bootstrap (default-deny + multi-target) and db (allow-db)
helm upgrade bootstrap   deploy/helm/bootstrap   -n stock --set networkPolicy.enabled=true
helm upgrade db          deploy/helm/db          -n stock --set networkPolicy.enabled=true
```

### Verify the CKAD mandatory items

```bash
kubectl get pods,svc,endpoints -n stock          # D1/N1/N5 — all Ready, no orphan Endpoints
kubectl get cronjob,deploy -n stock              # D2/P1  — Deployments + CronJobs
kubectl get pod <pod> -n stock \
  -o jsonpath='{.spec.initContainers[*].name} | {.spec.containers[*].name}'   # D3 init + sidecar
kubectl get pvc -n stock                         # D5 — persistent volumes
kubectl get hpa -n stock                         # P4
kubectl get resourcequota,limitrange -n stock    # C5
kubectl get netpol,ingress -n stock              # N3/N4
kubectl auth can-i list pods \
  --as=system:serviceaccount:stock:pod-reader -n stock   # C4 — expect "yes"
helm history api-svc -n stock                    # P6 — per-chart upgrade/rollback trail
```

### Debug runbook (O4)

Backend pods run the **ambassador pattern** (≥4 containers) so `kubectl logs` needs `-c`:

```bash
kubectl logs <pod> -n stock -c log-sidecar        # tail app stdout via sidecar (emptyDir)
kubectl logs <pod> -n stock -c api-svc            # the app container directly
kubectl logs <pod> -n stock -c wait-db            # init container (DB readiness)
kubectl describe pod <pod> -n stock               # events, probe status, mounts, QoS
kubectl get events -n stock --sort-by=.lastTimestamp | tail -20
kubectl top pod -n stock                          # CPU/mem (needs metrics-server)
kubectl exec <pod> -n stock -c api-svc -- env | grep -E 'GRPC_TARGET|JWT'   # ConfigMap/Secret injection
```

Container names per backend: init `wait-db` → app (`<svc>`) → `nginx` ambassador → `log-sidecar`.

### Rollout, blue/green, rollback (P2/P3/P6)

```bash
kubectl set image deploy/api-svc-green api-svc=api-svc:v2 -n stock && kubectl rollout status deploy/api-svc-green -n stock
kubectl patch svc api-svc -n stock -p '{"spec":{"selector":{"color":"blue"}}}'   # blue/green flip
helm rollback api-svc <REV> -n stock                                             # Helm rollback (per chart)
```

### Kustomize overlay (P5)

```bash
kubectl kustomize deploy/k8s/kustomize/overlays/prod       # render (image v2, replicas 3)
kubectl apply -k deploy/k8s/kustomize/overlays/dev         # apply into ns stock (needs Helm stack)
```

### Known limitations

- `bluegreen.enabled` (on the `api-svc` chart) must stay **true** — gateway-svc hard-routes
  `/api → api-svc-<color>`.
- Each chart's `values.yaml` secrets have **empty defaults** (Helm `required` guards prevent deploy without real values). Provide credentials via a gitignored `values-secret.yaml` overridden with `-f` at `helm upgrade`. The `db` chart's four role passwords (`authDbPassword`, `marketDbPassword`, `apiDbPassword`, `registryDbPassword`) must match the corresponding `secrets.postgresPassword` in each service chart.
- prediction-svc uses a `startupProbe` (~150s for torch import); gRPC health is a `tcpSocket` probe.

## Configuration

Create a `.env` file at the **repo root** (passed to Compose via `--env-file .env`):

```env
# HTTP server (api-svc)
SERVER_PORT=8118

# gRPC targets (internal Docker DNS)
GRPC_SERVER_PORT=8119
GRPC_TARGET=prediction-svc:8119       # use localhost:8119 when running api-svc locally
AUTH_GRPC_TARGET=auth-svc:8120        # use localhost:8120 when running api-svc locally

# Auth
JWT_SECRET=change-me-in-production    # required — shared between api-svc and auth-svc
SUPER_ADMIN_USERNAME=                 # seeded by auth-svc on first start (fail-fast if blank)
SUPER_ADMIN_PASSWORD=                 # no hardcoded default — set a strong value

# Database — database-per-service (one TimescaleDB instance, three databases)
POSTGRES_HOST=db                      # Docker internal; use localhost when running locally
POSTGRES_PORT=5432

# Superuser (used only by the db container init and backup pg_dumpall)
POSTGRES_USER=postgres
POSTGRES_PASSWORD=                    # superuser password

# auth-svc (auth_db, role auth_svc)
AUTH_DB_PASSWORD=

# prediction-svc / jobs_cli (market_db, role prediction_svc)
MARKET_DB_PASSWORD=
POSTGRES_DEBUG=false                  # true = SQLAlchemy echo SQL

# api-svc (market_db, role api_svc — read-only + cron_schedules/sim_bots writes)
API_DB_PASSWORD=

# service-mgt (registry_db, role service_mgt)
REGISTRY_DB_PASSWORD=

# Logging
LOG_LEVEL=INFO
DB_LOG_LEVEL=WARN

# Backup (api-svc writes pg_dump output here)
BACKUP_DIR=/backups

# Service registry (service-mgt) — optional
SERVICE_MGT_ENABLED=false             # set true to enable register/discover; false = static endpoints (default)
REGISTRY_GRPC_TARGET=service-mgt:8121 # address used by services to reach service-mgt (Docker internal DNS)
```

`JWT_SECRET` is injected into both `api-svc` (for local JWT validation) and `auth-svc` (for token signing) via the Compose file.

## Development

All Makefile targets run from the repo root.

```bash
make up       # docker compose up -d
make reset    # docker compose down && up --build
make build    # cd api-svc && go build -o api-server ./cmd
make vet      # cd api-svc && go vet ./...
make swagger  # regenerate api-svc/docs/ via swag (needs swag CLI)
```

### Per-service build and test

**api-svc (Go)**
```bash
cd api-svc && go build -o api-server ./cmd
cd api-svc && go test ./... -v -count=1          # all tests
cd api-svc && go test ./... -v -count=1 -short   # unit tests only
cd api-svc && go test ./... -coverprofile=coverage.out && go tool cover -html=coverage.out -o coverage.html
```

Or via the root Makefile: `make test`, `make test-unit`, `make test-coverage`.

**prediction-svc (Python)**
```bash
cd prediction-svc && make install      # install deps
cd prediction-svc && make test-unit    # unit tests (no Docker needed)
cd prediction-svc && make test-phase5  # full regression (needs running stack)
# or run inside the container:
docker exec prediction-svc python -m pytest tests/ -v
```

Or via root Makefile: `make test-phase5`.

**auth-svc (Java)**
```bash
cd auth-svc && mvn verify
```

**gateway-svc (Rust)**
```bash
cd gateway-svc && cargo build
cd gateway-svc && cargo test
```

**web-svc (TypeScript + Vite)**
```bash
cd web-svc && npm install
cd web-svc && npm run build
```

### Proto regeneration

When `api-svc/proto/prediction/prediction.proto` or `api-svc/proto/auth/auth.proto` changes:

```bash
# Go stubs (run from api-svc/)
cd api-svc && protoc --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  proto/prediction/prediction.proto

# Python stubs (run inside Docker or with grpcio-tools installed)
cd prediction-svc && make proto
```

Do not edit generated `*.pb.go` or `*_pb2*.py` files by hand.

## Key API Endpoints

### Auth & Users

| Method | Path | Notes |
|--------|------|-------|
| `POST` | `/api/x/grant` | Login — `X-Token: base64("user:pass")` header, body `{"request":""}` (ignored); returns JWT with `accessible_markets` claim |
| `GET` | `/api/auth/me` | Verify token |
| `PUT` | `/api/auth/password` | Change own password (JWT required) |
| `GET` | `/api/users` | List users (admin) |
| `POST` | `/api/users` | Create user (admin) — optional `full_name`, `email`, `phone` |
| `PUT` | `/api/users/{id}` | Update user profile/role (admin) |
| `DELETE` | `/api/users/{id}` | Delete user (admin) |
| `POST` | `/api/users/{id}/reset-password` | Reset user password (admin) — min 6 chars |

### Market Groups (admin only)

| Method | Path | Notes |
|--------|------|-------|
| `GET` | `/api/market-groups` | List all groups |
| `POST` | `/api/market-groups` | Create group |
| `PUT` | `/api/market-groups/{id}/markets` | Assign market keys to group |
| `POST` | `/api/market-groups/{id}/users` | Add user to group |
| `DELETE` | `/api/market-groups/{id}/users/{uid}` | Remove user from group |

### Predictions & Monitoring

| Method | Path | Notes |
|--------|------|-------|
| `GET` | `/api/predictions/direction-accuracy` | Per-algo direction accuracy — `?market=GOLD\|NASDAQ\|SP500\|CRYPTO` |
| `GET` | `/api/monitoring/overview` | Pipeline health — crawl freshness, algo counts, bot stats (JWT, 30 s cache) |
| `GET` | `/api/pipeline-reports` | Pipeline run reports — `?pipeline=<key>&limit=<n>` (default 50, max 200) |
| `GET` | `/api/schedules` | Cron schedule list (JWT) |
| `PUT` | `/api/schedules/{key}` | Update cron schedule live (JWT) |

### Triggers (admin / super_admin only)

All `POST /api/trigger/*` require admin or super_admin JWT.

| Method | Path | Notes |
|--------|------|-------|
| `POST` | `/api/trigger/train` | Train all or one algorithm |
| `POST` | `/api/trigger/gold-crawler` | Crawl Gold prices |
| `POST` | `/api/trigger/nasdaq-crawler` | Crawl NASDAQ |
| `POST` | `/api/trigger/crypto-crawler` | Crawl Crypto |
| `POST` | `/api/trigger/sp500-crawler` | Crawl S&P 500 |
| `POST` | `/api/trigger/reconcile` | Reconcile predictions with actual prices |
| `POST` | `/api/trigger/historical-backtest` | Walk-forward backtest — `?train_window=30&step_size=6&market_key=ALL` |
| `POST` | `/api/trigger/backup` | Manual DB backup |

### Backups

| Method | Path | Notes |
|--------|------|-------|
| `GET` | `/api/backups` | List backup files (JWT) |
| `GET` | `/api/backups/{filename}` | Download backup file (JWT) |
| `DELETE` | `/api/backups/{filename}` | Delete backup file (admin JWT) |

## Cron Schedules (default)

Stored in the `cron_schedules` DB table. Edit live via `PUT /api/schedules/{key}` or the Settings page — no restart required. `prediction-svc` polls DB every 60 s and reschedules automatically.

| Job Key | Default Schedule | Task |
|---------|-----------------|------|
| `crawler_gold` | `0 0 * * * *` | Gold pipeline: crawl → train every 10 runs → predict |
| `crawler_nasdaq` | `0 15 * * * 1-5` | NASDAQ pipeline (weekdays, minute 15) |
| `crawler_sp500` | `0 0,30 * * * 1-5` | S&P 500 pipeline (weekdays, minutes 0 and 30) |
| `crawler_crypto` | `0 0 */2 * * *` | Crypto pipeline (every 2 hours) |
| `train_gold` | `0 0 3 * * 0` | Retrain Gold models (Sunday 3 AM) |
| `train_nasdaq` | `0 0 4 * * 0` | Retrain NASDAQ models (Sunday 4 AM) |
| `train_crypto` | `0 0 5 * * 0` | Retrain Crypto models (Sunday 5 AM) |
| `train_sp500` | `0 0 7 * * 0` | Retrain S&P 500 models (Sunday 7 AM) |
| `daily_reconcile` | `0 0 6 * * *` | Reconcile predictions with actual prices |
| `simulation_daily` | `0 0 20 * * *` | Bot trading step (8 PM) |
| `daily_backup` | `0 0 3 * * *` | `pg_dumpall` (superuser) — all 3 databases + roles in one archive; compose: ofelia on `db` container; k8s: `cronjob-backup.yaml` → PVC `backup-data` |

## ML Algorithms (11)

| Key | Name | Notes |
|-----|------|-------|
| `moving_average` | Moving Average | VWMA slope + RSI momentum + StochRSI overlay |
| `ema` | EMA/MACD | EMA slope + MACD boost + Bollinger %B mean-reversion |
| `lstm_nn` | LSTM | PyTorch, 2 layers, hidden=64, seq=60 |
| `gru_nn` | GRU | PyTorch, 2 layers, hidden=64, seq=60 |
| `arima_garch` | ARIMA-GARCH | statsmodels ARIMA(2,1,2) + arch GARCH(1,1) |
| `egarch` | EGARCH | arch EGARCH(1,1,1) with HARX mean |
| `sarima` | SARIMA | statsmodels SARIMA(1,1,1)(1,0,1,5), seasonal period 5 |
| `lightgbm` | LightGBM | ~30 features; Optuna (30 trials, 120 s timeout, ≥200 data points) |
| `xgboost` | XGBoost | ~30 features; Optuna (30 trials, 120 s timeout, ≥200 data points) |
| `random_forest` | Random Forest | n_estimators=200, max_depth=8; no Optuna |
| `ensemble` | Ensemble | Equal-weight average of the 10 base models above |

Market-aware price-change clamp: GOLD/SP500 ±15%, NASDAQ100 ±20%, CRYPTO ±50%.

## Extending the System

### Add a new ML algorithm

1. Create `prediction-svc/src/algorithms/<name>.py` implementing `PredictionAlgorithm` from `base.py`. Apply `get_max_change_pct(self._market_key)` clamp before returning `PredictionResult`.
2. Register the class in `prediction-svc/src/algorithms/registry.py` inside `build_algorithms()`. If it should participate in the Ensemble, add the instance to `ensemble.py`.
3. Add metadata to `api-svc/pkg/service/predict/registry/algorithms.go` inside `init()`:
   ```go
   Register(AlgorithmDef{Key: "my_algo", DisplayName: "My Algorithm", Config: map[string]interface{}{}})
   ```
4. The algorithm automatically appears in `GET /api/training/algorithms` and all prediction workflows.

For tree-based models, reuse `build_enhanced_features()` from `prediction-svc/src/algorithms/features.py` (~30 features, with numpy-only fallback if `pandas-ta` is absent).

### Add a new market

1. Add DB model: Go GORM struct in `api-svc/pkg/models/models_db/` and Python ORM model in `prediction-svc/src/database/models.py`. Add the table DDL to `deploy/db-init/schemas/10-market_db.sql` (price/prediction tables are hypertables in `market_db`).
2. Add repository methods in `prediction-svc/src/database/repository.py` and, as needed, in `api-svc/pkg/store/repository/repository.go` + `api-svc/pkg/store/postgres/`.
3. Create a crawler in `prediction-svc/src/crawlers/<name>.py` implementing `BaseCrawler`.
4. Register a cron pipeline job in `prediction-svc/src/scheduler/jobs.py` and wire it in `prediction-svc/src/orchestrator/runner.py`.
5. Add trigger endpoints in `api-svc/pkg/server/api_trigger_<name>.go`, a new proto RPC in `api-svc/proto/prediction/prediction.proto`, and regenerate stubs (Go + Python).

## Notes and Gotchas

**ICT-at-rest timezone:** All `TIMESTAMP` columns store ICT (Asia/Ho_Chi_Minh, UTC+7) wallclock time — not UTC. Python: use `datetime.now()` only (container has `TZ=Asia/Ho_Chi_Minh`), never `datetime.utcnow()`. Go: `time.Local` is set to `Asia/Ho_Chi_Minh` in `api-svc/cmd/main.go`.

**AutoMigrate is disabled:** GORM `AutoMigrate` is off because TimescaleDB hypertable composite PKs conflict with it. `market_db` schema is managed by `deploy/db-init/schemas/10-market_db.sql`; `auth_db` schema is managed by Flyway inside `auth-svc`. For subsequent changes, update the relevant file and apply manually (or extend Flyway for auth tables).

**prediction-svc build context is the repo root:** The `prediction-svc` Dockerfile needs `api-svc/proto/` for proto stub generation. The other four services use `context: ../<svc-dir>` with `dockerfile: ../deploy/<svc>.Dockerfile`.

**Cron schedules source of truth:** `DEFAULT_SCHEDULES` in `prediction-svc/src/scheduler/manager.py` is the authoritative source for Python-managed jobs. On each `prediction-svc` startup, `upsert_cron_schedule()` runs a true upsert — it overwrites DB values that differ from the code defaults. The `daily_backup` job entry in `cron_schedules` is reference-only; actual execution is by `ofelia` (compose) or `cronjob-backup.yaml` (k8s), not by `api-svc`.

**Go module name unchanged:** The Go module path remains `go-stock-prediction` (declared in `api-svc/go.mod`) despite the directory rename from `api/` to `api-svc/`.

**pgAdmin credentials:** Default `admin@local.dev` / `admin` — change for any shared environment.
