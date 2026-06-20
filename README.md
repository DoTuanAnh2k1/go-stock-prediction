# go-stock-prediction

Financial asset price prediction system with RBAC — crawls Gold SJC/XAU, NASDAQ, Crypto BTC/ETH/SOL, and S&P 500, runs 11 ML algorithms, and displays results in a web dashboard with role-based market access control.

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
│  pgAdmin 127.0.0.1:8081                                                  │
└─────────────────────────────────────────────────────────────────────────┘
```

## Services

| Service (dir) | Tech | Port | Role |
|---------------|------|------|------|
| `gateway-svc/` | Rust, Axum 0.8, rustls | `:80` / `:443` (public) | TLS termination; longest-prefix routing: `/swagger` → block, `/api` → `api-svc:8118`, `/health` → `api-svc:8118`, `/` → `web-svc:3000`; gateway-local `/healthz` and `/readyz` |
| `api-svc/` | Go 1.25+, net/http, gRPC, GORM v2, ZeroLog | `:8118` (internal) | HTTP API; thin auth proxy to `auth-svc` via gRPC; direct DB reads for market data; triggers `prediction-svc` via gRPC; backup scheduler |
| `auth-svc/` | Java 21, Spring Boot 3, gRPC, Flyway, bcrypt | `:8120` (internal) | Owns all RBAC: login, JWT generation (HMAC256, 24 h), user CRUD, market groups; Flyway V1: auth + RBAC tables; V2: `full_name`/`email`/`phone` profile fields; seeds `chon/super_admin` on startup |
| `prediction-svc/` | Python 3.12, PyTorch, statsmodels, LightGBM, XGBoost, scikit-learn, APScheduler, SQLAlchemy | `:8119` (internal) | gRPC service: crawling, 11 ML algorithms, training, cron scheduler |
| `web-svc/` | React, TypeScript, Vite, nginx | `:3000` (internal) | Static SPA served by nginx; accessed only through `gateway-svc` |
| `db` | TimescaleDB (PostgreSQL 16) | `:5432` (internal) | Shared DB for all services; hypertables for price/prediction time-series; schema auto-init from `database.sql` |
| `pgadmin` | pgAdmin 4 | `127.0.0.1:8081` | PostgreSQL web administration UI |

Internal DNS (between containers): `api-svc:8118`, `prediction-svc:8119`, `auth-svc:8120`, `web-svc:3000`, `db:5432`.

## Features

- **RBAC with market groups:** Three roles — `super_admin` (full access), `admin` (manage users and market groups), `user` (access only assigned markets). JWT includes `accessible_markets` claim; sidebar hides inaccessible market tabs.
- **Data collection:** Gold SJC/XAU/USD every hour; NASDAQ and S&P 500 every hour weekdays (skips NYSE holidays); Crypto BTC/ETH/SOL every 2 hours.
- **11 ML algorithms:** Moving Average, EMA/MACD, LSTM (PyTorch), GRU (PyTorch), ARIMA-GARCH, EGARCH, SARIMA, LightGBM (Optuna tuned), XGBoost (Optuna tuned), Random Forest, Ensemble.
- **Walk-forward backtest:** Historical backtesting via `POST /api/trigger/historical-backtest`.
- **Automated training:** Per-market weekly training jobs (Sunday 3–7 AM).
- **Daily reconcile:** 6 AM updates `direction_correct` for past predictions.
- **Dynamic cron schedules:** DB-backed, editable live via Settings page — no restart needed.
- **Pipeline monitoring:** `/api/monitoring/overview` — crawl freshness, per-algo prediction counts, bot win/loss stats (JWT required, 30 s cache).
- **Trading simulation:** Bot leaderboard, per-bot trades and portfolio snapshots, live-step and backtest modes.
- **Pipeline reports:** Per-pipeline run records with step-level status, stored 7 days (`GET /api/pipeline-reports`).
- **Auto backup:** Daily `pg_dump` at 3 AM into `BACKUP_DIR`; owned by `api-svc` (`backup_scheduler.go`).

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
├── deploy/                # All Dockerfiles and Compose files (flat layout)
│   ├── docker-compose.yaml
│   ├── docker-compose.test.yml
│   ├── api-svc.Dockerfile
│   ├── auth-svc.Dockerfile
│   ├── prediction-svc.Dockerfile
│   ├── web-svc.Dockerfile
│   └── gateway-svc.Dockerfile
├── database.sql           # Full PostgreSQL/TimescaleDB schema (authoritative)
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

The `db` container auto-initialises the schema from `database.sql` on first startup via `/docker-entrypoint-initdb.d/01-schema.sql`. No manual import is needed.

### Access

| URL | Service |
|-----|---------|
| `http://localhost` or `https://localhost` | React dashboard (via Gateway) |
| `http://localhost/api/...` | REST API (via Gateway) |
| `http://localhost:8081` | pgAdmin (127.0.0.1 only) |
| `http://localhost:8118/swagger/` | Swagger UI (direct to `api-svc`, bypasses Gateway) |

**Default credentials:** `chon` / `super_admin` — seeded by `auth-svc` on first startup.

### First-run data load

```bash
# Get a JWT token (super_admin seeded by auth-svc)
TOKEN=$(curl -s -X POST http://localhost/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"chon","password":"super_admin"}' | jq -r '.token')

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
ADMIN_PASSWORD=admin123               # legacy seed fallback

# Database (TimescaleDB / PostgreSQL 16)
DB_DRIVER=postgresql
POSTGRES_HOST=db                      # Docker internal; use localhost when running locally
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=123
POSTGRES_DB=go_stock_prediction
POSTGRES_DEBUG=false

# Logging
LOG_LEVEL=INFO
DB_LOG_LEVEL=WARN

# Backup (api-svc writes pg_dump output here)
BACKUP_DIR=/backups
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
| `POST` | `/api/auth/login` | Login — returns JWT with `accessible_markets` claim |
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
| `daily_backup` | `0 0 3 * * *` | `pg_dump` to `BACKUP_DIR` — run by **api-svc** (`backup_scheduler.go`), not Python |

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

1. Add DB model: Go GORM struct in `api-svc/pkg/models/models_db/` and Python ORM model in `prediction-svc/src/database/models.py`. Add the table definition to `database.sql` (create as a hypertable if it holds time-series data).
2. Add repository methods in `prediction-svc/src/database/repository.py` and, as needed, in `api-svc/pkg/store/repository/repository.go` + `api-svc/pkg/store/postgres/`.
3. Create a crawler in `prediction-svc/src/crawlers/<name>.py` implementing `BaseCrawler`.
4. Register a cron pipeline job in `prediction-svc/src/scheduler/jobs.py` and wire it in `prediction-svc/src/orchestrator/runner.py`.
5. Add trigger endpoints in `api-svc/pkg/server/api_trigger_<name>.go`, a new proto RPC in `api-svc/proto/prediction/prediction.proto`, and regenerate stubs (Go + Python).

## Notes and Gotchas

**ICT-at-rest timezone:** All `TIMESTAMP` columns store ICT (Asia/Ho_Chi_Minh, UTC+7) wallclock time — not UTC. Python: use `datetime.now()` only (container has `TZ=Asia/Ho_Chi_Minh`), never `datetime.utcnow()`. Go: `time.Local` is set to `Asia/Ho_Chi_Minh` in `api-svc/cmd/main.go`.

**AutoMigrate is disabled:** Schema is managed exclusively by `database.sql`. GORM `AutoMigrate` is off because TimescaleDB hypertable composite PKs conflict with it. Schema is auto-applied on first container startup; for subsequent changes, write a migration and apply manually.

**prediction-svc build context is the repo root:** The `prediction-svc` Dockerfile needs `api-svc/proto/` for proto stub generation. The other four services use `context: ../<svc-dir>` with `dockerfile: ../deploy/<svc>.Dockerfile`.

**Cron schedules source of truth:** `DEFAULT_SCHEDULES` in `prediction-svc/src/scheduler/manager.py` is the authoritative source for Python-managed jobs. On each `prediction-svc` startup, `upsert_cron_schedule()` runs a true upsert — it overwrites DB values that differ from the code defaults. The `daily_backup` job is the exception: it is seeded (insert-if-not-exists, never overwritten) and polled by `api-svc/pkg/server/backup_scheduler.go`.

**Go module name unchanged:** The Go module path remains `go-stock-prediction` (declared in `api-svc/go.mod`) despite the directory rename from `api/` to `api-svc/`.

**pgAdmin credentials:** Default `admin@local.dev` / `admin` — change for any shared environment.
