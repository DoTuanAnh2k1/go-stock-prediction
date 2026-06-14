# go-stock-prediction

Financial asset price prediction system with RBAC — crawls Gold SJC/XAU, NASDAQ, Crypto BTC/ETH/SOL, and S&P 500, runs 11 ML algorithms, and displays results in a web dashboard with role-based market access control.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Docker Compose                                │
│                                                                      │
│  Browser ──► Nginx :80 ──► API Backend :8118 (Go)                  │
│                                    │                                 │
│              Frontend :36018        ├──gRPC──► Auth Service :8120   │
│              (React)               │          (Java Spring Boot)     │
│                                    │                                 │
│                                    └──gRPC──► Prediction :8119      │
│                                               (Python)               │
│                                                    │                 │
│              MySQL :3306 ◄─────────────────────────┘                │
│              (shared DB)                                             │
│                                                                      │
│  phpMyAdmin :8081                                                    │
└─────────────────────────────────────────────────────────────────────┘
```

**Services:**
- **Nginx** `:80` — reverse proxy, single entry point
- **API Backend** (`api/`) — Go HTTP `:8118`; thin auth proxy to Java Auth Service via gRPC, reads DB directly for market data, triggers Python Prediction Service via gRPC
- **Auth Service** (`auth-service/`) — Java Spring Boot 3, gRPC `:8120` (internal); owns all RBAC: login, JWT generation, user CRUD, market groups, Flyway migrations
- **Prediction Service** (`prediction/`) — Python gRPC `:8119` (internal); crawling, 11 ML algorithms, training, APScheduler cron jobs
- **MySQL** `:3306` — shared database for all services
- **Frontend** (`frontend/`) — React + TypeScript SPA `:36018`

## Features

- **RBAC with market groups:** Three roles — `super_admin` (full access), `admin` (manage users and market groups), `user` (access only assigned markets). JWT includes `accessible_markets` claim; sidebar hides tabs for inaccessible markets.
- **Data collection:** Gold SJC/XAU/USD every hour; NASDAQ and S&P 500 every hour (Mon-Fri, skips NYSE holidays); Crypto BTC/ETH/SOL every 2 hours
- **11 ML algorithms:** Moving Average, EMA/MACD, LSTM (PyTorch), GRU (PyTorch), ARIMA-GARCH, EGARCH, SARIMA, LightGBM (Optuna tuned), XGBoost (Optuna tuned), Random Forest, Ensemble
- **Walk-forward backtest:** Historical backtesting via `POST /api/trigger/historical-backtest`
- **Automated training:** Per-market weekly training jobs (Sunday 3-7 AM)
- **Daily reconcile:** 6 AM updates `actual_price` and `direction_correct` for past predictions
- **Dynamic cron schedules:** DB-backed, editable live via Settings page — no restart needed
- **Pipeline monitoring:** `/api/monitoring/overview` — crawl freshness, per-algo prediction counts, bot win/loss stats (JWT, 30s cache)
- **Auto backup:** Daily mysqldump at 3 AM into `BACKUP_DIR`

## Quick Start

```bash
# Start all 7 services
docker-compose up -d
```

| URL | Service |
|-----|---------|
| `http://localhost:36018` | React dashboard |
| `http://localhost:80` | API (via Nginx) |
| `http://localhost:8081` | phpMyAdmin |
| `http://localhost:8118/swagger/` | Swagger UI |

Default credentials: `chon` / `super_admin` (seeded by Java Auth Service on first startup).

## Environment Variables

Create a `.env` file in the project root:

```env
# HTTP server
SERVER_PORT=8118

# gRPC
GRPC_SERVER_PORT=8119
GRPC_TARGET=prediction:8119       # Docker internal; use localhost:8119 when running locally
AUTH_GRPC_TARGET=auth:8120        # Docker internal; use localhost:8120 when running locally

# Auth
JWT_SECRET=change-me-in-production
ADMIN_USERNAME=admin              # Legacy — only used as fallback seed; Java Auth Service seeds chon/super_admin
ADMIN_PASSWORD=admin123

# Database
DB_DRIVER=mysql
MYSQL_HOST=db                     # Docker internal; use localhost when running locally
MYSQL_PORT=3306
MYSQL_USER=root
MYSQL_PASSWORD=123
MYSQL_DB_NAME=go_stock_prediction
MYSQL_DEBUG=false

# Logging
LOG_LEVEL=DEBUG
DB_LOG_LEVEL=DEBUG

# Backup
BACKUP_DIR=/backups
```

## First-time Setup

After `docker-compose up -d`, the DB is empty. Run in order:

```bash
# 1. Get a JWT token (super_admin seeded by Java Auth Service)
TOKEN=$(curl -s -X POST http://localhost/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"chon","password":"super_admin"}' | jq -r '.token')

# 2. Crawl initial data
curl -X POST http://localhost/api/trigger/gold-crawler   -H "Authorization: Bearer $TOKEN"
curl -X POST http://localhost/api/trigger/nasdaq-crawler -H "Authorization: Bearer $TOKEN"
curl -X POST http://localhost/api/trigger/crypto-crawler -H "Authorization: Bearer $TOKEN"
curl -X POST http://localhost/api/trigger/sp500-crawler  -H "Authorization: Bearer $TOKEN"

# 3. Train models
curl -X POST http://localhost/api/trigger/train -H "Authorization: Bearer $TOKEN"

# 4. Run walk-forward backtest (generates chart data)
curl -X POST "http://localhost/api/trigger/historical-backtest?train_window=30&step_size=6&market_key=ALL" \
  -H "Authorization: Bearer $TOKEN"
```

## Ports

| Service | Port | Notes |
|---------|------|-------|
| Nginx | 80 | Reverse proxy → API Backend |
| API Backend | 8118 | HTTP (internal) |
| Prediction Service | 8119 | gRPC (internal) |
| Auth Service | 8120 | gRPC (internal) — Java Spring Boot RBAC |
| Frontend | 36018 | React app |
| MySQL | 3306 | Docker |
| phpMyAdmin | 8081 | Admin UI |

## Key API Endpoints

### Auth & Users

| Method | Path | Notes |
|--------|------|-------|
| `POST` | `/api/auth/login` | Login — returns JWT with `accessible_markets` claim |
| `GET` | `/api/auth/me` | Verify token |
| `PUT` | `/api/auth/password` | Change password (JWT required) |
| `GET` | `/api/users` | List users (admin) |
| `POST` | `/api/users` | Create user (admin) |
| `DELETE` | `/api/users/{id}` | Delete user (admin) |

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
| `GET` | `/api/monitoring/overview` | Pipeline health — crawl freshness, algo counts, bot stats (JWT, 30s cache) |
| `GET` | `/api/schedules` | Cron schedule list (JWT) |
| `PUT` | `/api/schedules/{key}` | Update cron schedule live (JWT) |

### Triggers (admin/super_admin only)

All `POST /api/trigger/*` require admin or super_admin JWT.

| Method | Path | Notes |
|--------|------|-------|
| `POST` | `/api/trigger/train` | Train all or one algorithm |
| `POST` | `/api/trigger/gold-crawler` | Crawl Gold prices |
| `POST` | `/api/trigger/nasdaq-crawler` | Crawl NASDAQ |
| `POST` | `/api/trigger/crypto-crawler` | Crawl Crypto |
| `POST` | `/api/trigger/sp500-crawler` | Crawl S&P 500 |
| `POST` | `/api/trigger/reconcile` | Reconcile actual prices |
| `POST` | `/api/trigger/historical-backtest` | Walk-forward backtest — `?train_window=30&step_size=6&market_key=ALL` |
| `POST` | `/api/trigger/backup` | Manual DB backup (admin) |

## Cron Schedule (default)

Stored in `cron_schedules` DB table. Edit live via `PUT /api/schedules/{key}` or the Settings page.

| Job Key | Default Schedule | Task |
|---------|-----------------|------|
| `crawler_gold` | Every hour (minute 0) | Gold pipeline: crawl → train every 10 runs → predict |
| `crawler_nasdaq` | Every hour minute 15 (Mon-Fri) | NASDAQ pipeline |
| `crawler_sp500` | Every 30 min (Mon-Fri) | S&P 500 pipeline |
| `crawler_crypto` | Every 2 hours | Crypto pipeline |
| `train_gold` | Sunday 3 AM | Retrain Gold models |
| `train_nasdaq` | Sunday 4 AM | Retrain NASDAQ models |
| `train_crypto` | Sunday 5 AM | Retrain Crypto models |
| `train_sp500` | Sunday 7 AM | Retrain S&P 500 models |
| `daily_reconcile` | Daily 6 AM | Reconcile predictions with actual prices |
| `simulation_daily` | Daily 8 PM | Bot trading step |
| `daily_backup` | Daily 3 AM | mysqldump to `BACKUP_DIR` |

## ML Algorithms (11)

| Key | Name | Notes |
|-----|------|-------|
| `moving_average` | Moving Average | VWMA slope + RSI momentum + StochRSI overlay |
| `ema` | EMA/MACD | EMA slope + MACD boost + Bollinger %B mean-reversion |
| `lstm_nn` | LSTM | PyTorch, 2 layers, hidden=64, seq=60 |
| `gru_nn` | GRU | PyTorch, 2 layers, hidden=64, seq=60 |
| `arima_garch` | ARIMA-GARCH | statsmodels ARIMA(2,1,2) + arch GARCH(1,1) |
| `egarch` | EGARCH | arch EGARCH(1,1,1) with HARX mean |
| `sarima` | SARIMA | statsmodels SARIMA(1,1,1)(1,0,1,5) |
| `lightgbm` | LightGBM | ~30 features; Optuna (30 trials, 120s timeout) |
| `xgboost` | XGBoost | ~30 features; Optuna (30 trials, 120s timeout) |
| `random_forest` | Random Forest | n_estimators=200, max_depth=8 |
| `ensemble` | Ensemble | Equal-weight average of 10 base models |

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Auth Service | Java 21, Spring Boot 3, gRPC, Flyway, bcrypt |
| API Backend | Go 1.23+, net/http, gRPC, GORM v2, ZeroLog, shopspring/decimal |
| Prediction Service | Python 3.12, PyTorch, statsmodels, LightGBM, XGBoost, scikit-learn, APScheduler, SQLAlchemy |
| Frontend | React, TypeScript, Vite |
| Database | MySQL 8.0 |
| Proxy | Nginx |

## Timezone

All `datetime` columns in the DB store ICT (Asia/Ho_Chi_Minh, UTC+7) wallclock time — not UTC.

- **Python:** use `datetime.now()` only (container has `TZ=Asia/Ho_Chi_Minh`). Never `datetime.utcnow()`.
- **Go:** DSN has `loc=Asia%2FHo_Chi_Minh`; `time.Local = Asia/Ho_Chi_Minh` set at startup.
- **Java Auth Service:** configure JVM timezone to `Asia/Ho_Chi_Minh` for consistency.
