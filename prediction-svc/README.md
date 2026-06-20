# Prediction Service (`prediction-svc`)

Python gRPC microservice for the **go-stock-prediction** platform. It owns all
market-data crawling, the machine-learning algorithms, model training,
prediction reconciliation, the trading-bot simulation, and the APScheduler cron
jobs that drive the pipelines.

- **Language / runtime:** Python 3.12
- **Transport:** gRPC on `:8119` (internal only — never exposed publicly)
- **Internal DNS name:** `prediction-svc:8119`
- **Container name:** `prediction-svc` (previously `prediction_service`)
- **Storage:** shared TimescaleDB / PostgreSQL 16 (`db:5432`, container `timescaledb`)
- **Consumer:** the Go API service (`api-svc:8118`) is the only gRPC client.

## 1. Overview

`prediction-svc` is a headless worker. It does not serve HTTP — the Go API
service calls it over gRPC to trigger crawls, training, predictions, reconcile,
backtests and bot simulation. Most heavy work runs on its own internal schedule
(APScheduler), so the trigger RPCs are mostly for on-demand/manual runs.

Responsibilities:

- **Crawling** four markets: Gold (SJC / XAU / Vietnamese dealers), NASDAQ,
  Crypto (BTC / ETH / SOL), S&P 500.
- **11 ML algorithms** producing per-market price predictions.
- **Training** (per-market and per-algorithm) with status tracking.
- **APScheduler cron jobs** backed by the `cron_schedules` DB table.
- **Reconcile** — fill in actual prices and compute accuracy + `direction_correct`.
- **Trading simulation** — bots, portfolios, live steps and backtests.

### Startup flow — `src/main.py`

1. **Config** — `get_settings()` loads env vars via pydantic-settings.
2. **Timezone** — sets `TZ=Asia/Ho_Chi_Minh` and calls `time.tzset()`.
3. **Logger** — `init_logger()` (structlog).
4. **Database** — `init_db()` builds the SQLAlchemy engine + session factory.
5. **gRPC server** — `start_grpc_server(port)` on `:8119`.
6. **Scheduler** — `init_scheduler(JOB_FUNCTIONS)` (waits for DB, upserts
   `DEFAULT_SCHEDULES`, registers enabled jobs, starts the 60s change watcher).
7. **Startup background threads** — after a short delay: seed simulation bots
   (`seed_bots()`) and run an initial `reconcile_predictions()`.
8. **Wait for signal** — block on `SIGTERM`/`SIGINT`, then
   `stop_grpc_server()` + `shutdown_scheduler()` and exit.

## 2. The 11 ML algorithms

Each algorithm implements the abstract `PredictionAlgorithm` interface in
`src/algorithms/base.py` (`predict(prices, volumes) -> PredictionResult`,
`get_name()`, `get_key()`, optional `train()`/`train_batch()`). Instances are
built and cached **per market** by `src/algorithms/registry.py`.

| Key | Class | Summary |
|-----|-------|---------|
| `moving_average` | `MovingAveragePredictor` | VWMA trend-slope projection + RSI momentum scaling + StochRSI overlay |
| `ema` | `EMAMACDPredictor` | EMA slope projection + MACD momentum boost + Bollinger %B mean-reversion overlay |
| `lstm_nn` | `LSTMPredictor` | PyTorch LSTM (2 layers, hidden=64, seq=60, dropout=0.2) |
| `gru_nn` | `GRUPredictor` | PyTorch GRU (2 layers, hidden=64, seq=60, dropout=0.2) |
| `arima_garch` | `ARIMAGARCHPredictor` | statsmodels ARIMA(2,1,2) + arch GARCH(1,1) |
| `egarch` | `EGARCHPredictor` | arch EGARCH(1,1,1) with HARX mean model |
| `sarima` | `SARIMAPredictor` | statsmodels SARIMA(1,1,1)(1,0,1,5), seasonal period 5 (trading week) |
| `lightgbm` | `LightGBMPredictor` | LightGBM on ~30 enhanced features; Optuna tuning; EMA fallback |
| `xgboost` | `XGBoostPredictor` | XGBoost on ~30 enhanced features; Optuna tuning; EMA fallback |
| `random_forest` | `RandomForestPredictor` | scikit-learn RandomForest (n_estimators=200, max_depth=8); no Optuna |
| `ensemble` | `EnsemblePredictor` | Equal-weight ensemble of the 10 base models above |

Cross-cutting behaviour:

- **Market-aware clamp** — every prediction is clamped to a per-market max
  change via `get_max_change_pct(self._market_key)` in `base.py`:
  GOLD ±15%, NASDAQ100 ±20%, SP500 ±15%, CRYPTO ±50% (default ±15% for unknown
  keys). The registry sets `_market_key` on each instance before `predict()`.
- **Shared feature builder** — `src/algorithms/features.py` exposes
  `build_basic_features()` (14 features) and `build_enhanced_features()`
  (~30 features), used by LightGBM / XGBoost / RandomForest. Uses `pandas-ta`
  when available, with a fully equivalent numpy-only fallback. `MIN_DATA_POINTS = 80`.
- **Optuna tuning** — LightGBM and XGBoost run a Bayesian hyperparameter search
  when data ≥ 200 points and `optuna` (the `[ml]` extra) is installed
  (≤ 30 trials, 120s timeout); otherwise fall back to default params.
- **Data ordering** — the repository returns prices DESC (newest first);
  algorithms expect ASC and reverse internally before building sequences.

## 3. Crawlers

All crawlers live in `src/crawlers/` and extend `BaseCrawler`. Each persists a
daily price plus (where applicable) intraday prices.

| Market | File | Data source(s) |
|--------|------|----------------|
| Gold | `gold.py` | Yahoo Finance `GC=F` (XAU/USD daily, intraday, history), USD→VND FX rate, BTMC API, Báo Tín Mạnh Hải, vang.today, Phú Quý (Vietnamese SJC/PNJ/DOJI/BTMC dealers) |
| NASDAQ | `nasdaq.py` | Yahoo Finance chart API — list of NASDAQ symbols (`NASDAQ_SYMBOLS`) |
| Crypto | `crypto.py` | CoinGecko simple-price + market-chart — BTC, ETH, SOL |
| S&P 500 | `sp500.py` | Yahoo Finance chart API — list of S&P 500 symbols (`SP500_SYMBOLS`) |

## 4. Scheduler & cron

`src/scheduler/manager.py` runs a single `BackgroundScheduler`
(timezone `Asia/Ho_Chi_Minh`) and is **DB-backed** via the `cron_schedules`
table.

- **Source of truth:** `DEFAULT_SCHEDULES` in `manager.py`. On startup the
  scheduler waits for the DB, then **true-upserts** every default schedule
  (`upsert_cron_schedule`) — so values edited in the DB are overwritten by the
  code defaults on restart.
- **Cron format:** the DB stores 6-field cron (`sec min hour day month weekday`,
  robfig/cron style). `parse_6field_cron()` strips the seconds field and maps
  weekday numbering to APScheduler's.
- **Live reload:** a `__schedule_watcher__` job polls the DB **every 60s**;
  changed rows are re-added or removed without a restart.

### Pipeline jobs — `src/scheduler/jobs.py`

The four market crawl jobs (`crawler_gold`, `crawler_nasdaq`, `crawler_sp500`,
`crawler_crypto`) all run `_run_pipeline(market_key, crawl_fn)`:

1. **Skip** entirely if the market is closed (see §6) → writes a `skipped` report.
2. **Crawl** new data (aborts the pipeline if the crawl fails).
3. Increment a per-market crawl counter; on **every 10th** crawl, run
   `train_for_market()`.
4. **Predict** via `run_for_market()` (which also drives the sim live step).
5. Write one row to `pipeline_reports` (status / steps / counts / duration) and
   call `delete_old_pipeline_reports(7)` for retention.

Other jobs: `daily_reconcile`, per-market training (`train_gold` / `train_nasdaq`
/ `train_crypto` / `train_sp500`), plus several legacy stand-alone predict/train
jobs that ship **disabled** (e.g. `gold_predict`, `predict_*`, `weekly_training`,
`daily_prediction`) because their work now runs inside the pipelines.

> **Note:** `daily_backup` is **not** owned here. The Go `api-svc` owns and runs
> the database backup schedule (`api/pkg/server/backup_scheduler.go`); it is not
> in `DEFAULT_SCHEDULES`.

## 5. Market calendar

`src/utils/market_calendar.py::is_market_open(market_key, when)` decides whether
a market is tradeable — no network access, no external libraries (NYSE holidays
are computed per year, including Good Friday via the Gregorian computus).

- **GOLD** — open on weekdays only (weekend-closed commodity), no time-of-day limit.
- **NASDAQ / SP500** — open only on NYSE trading days **and** within the ET
  session window (09:00–16:30 ET, ≈ 20:00–03:30 ICT); closed on weekends and
  NYSE holidays.
- **CRYPTO** (and any unrecognised key) — always open (24/7).

Three guard points enforce this in the service:

1. `scheduler/jobs.py::_run_pipeline` — skips the whole pipeline when closed.
2. `orchestrator/runner.py::run_for_market` — returns 0 predictions when closed.
3. `simulation/engine.py::run_live_step` — filters out bots whose market is closed.

## 6. Direction accuracy / reconcile

`src/orchestrator/training.py::reconcile_predictions()` walks pending predictions
across all four prediction tables (`gold_predictions`, `nasdaq_predictions`,
`sp500_predictions`, `crypto_predictions`):

- Finds the actual price on/after the prediction's `target_date`.
- Computes `accuracy = max(0, 1 - |actual - predicted| / actual)` and sets
  `status` to `confirmed` (≥ 0.70) or `wrong`.
- Sets the nullable boolean `direction_correct` — `True` when the predicted
  up/down direction matches the actual move (`NULL` until reconciled).
- Backfills `direction_correct` for older rows that were reconciled before the
  column existed.

The Go API's `/api/predictions/direction-accuracy` endpoint reads these columns.

## 7. gRPC contract

Defined in `api-svc/proto/prediction/prediction.proto` and implemented by
`PredictionServicer` in `src/grpc_server/server.py`.

**Triggers / actions**

| RPC | Behaviour |
|-----|-----------|
| `TriggerGoldCrawler` | Crawl gold synchronously (daily + intraday) |
| `TriggerGoldHistory` | Import XAU history in background |
| `TriggerGoldPredict` | Run gold predictions (forced) |
| `TriggerNasdaqCrawler` / `TriggerNasdaqPredict` | NASDAQ crawl / predict (background / forced) |
| `TriggerCryptoCrawler` / `TriggerCryptoPredict` | Crypto crawl / predict |
| `TriggerSP500Crawler` / `TriggerSP500Predict` | S&P 500 crawl / predict |
| `TriggerTrain` | Train one algorithm (`algorithm` field) or all in background |
| `TriggerReconcile` | Run `reconcile_predictions()` synchronously |
| `TriggerHistoricalBacktest` | Walk-forward backtest in background (concurrency-guarded; `train_window`, `step_size`, `market_key`) |
| `GetTrainingStatus` | Current training status (`is_training`, progress, phase, totals) |

**Simulation**

| RPC | Behaviour |
|-----|-----------|
| `TriggerSimulationBacktest` | Run a sim backtest for one bot or all bots |
| `TriggerSimulationLiveStep` | Run one live simulation step |
| `ResetSimBots` | Close old live sessions and create fresh ones for active bots |

**Streaming predict** (server-streaming `PipelineLogEvent`)

| RPC | Behaviour |
|-----|-----------|
| `StreamGoldPredict` / `StreamNasdaqPredict` / `StreamCryptoPredict` / `StreamSP500Predict` | Run `run_for_market` and stream live log/progress events |

> The proto also retains a few removed/no-op RPCs (`TriggerCrawler`,
> `TriggerStockHistory`, `TriggerStockCrawl`, `TriggerStockPredict`) for
> backward compatibility — they return failure.

## 8. Directory structure

```
prediction-svc/
├── Makefile                     # proto, install, run, lint, test targets
├── pyproject.toml               # deps (torch, statsmodels, arch, lightgbm, xgboost,
│                                #   grpcio, APScheduler, SQLAlchemy, pandas-ta; optuna in [ml])
├── scripts/gen_proto.sh         # local proto regen (reads api-svc/proto)
├── proto/                       # proto staging dir
└── src/
    ├── main.py                  # entry point (startup flow)
    ├── config.py                # pydantic Settings (env vars)
    ├── database/
    │   ├── connection.py        # SQLAlchemy engine + session factory
    │   ├── models.py            # ORM models (mirror the Go GORM structs)
    │   └── repository.py        # all query methods + cron + pipeline-report helpers
    ├── grpc_server/server.py    # PredictionServicer — implements the RPCs
    ├── algorithms/
    │   ├── base.py              # PredictionAlgorithm, MARKET_MAX_CHANGE, get_max_change_pct
    │   ├── registry.py          # per-market instance cache (build/get/set)
    │   ├── features.py          # shared feature builders (basic 14 / enhanced ~30)
    │   ├── moving_average.py, ema_macd.py, lstm.py, gru.py, arima_garch.py,
    │   ├── egarch.py, sarima.py, lightgbm_model.py, xgboost_model.py,
    │   ├── random_forest.py     # the 10 base algorithms
    │   └── ensemble.py          # equal-weight ensemble of the 10
    ├── crawlers/
    │   ├── base.py, gold.py, nasdaq.py, crypto.py, sp500.py
    ├── scheduler/
    │   ├── manager.py           # APScheduler + DEFAULT_SCHEDULES + 60s watcher
    │   └── jobs.py              # job functions + _run_pipeline + JOB_FUNCTIONS
    ├── orchestrator/
    │   ├── runner.py            # run_all_markets / run_for_market
    │   └── training.py          # train_for_market / train_all / reconcile_predictions / backtest
    ├── simulation/              # trading bots: engine, bot, portfolio, signal, metrics, seeder
    ├── utils/
    │   ├── logger.py, timezone, number_parser.py, market_calendar.py
    └── proto/prediction/        # generated stubs (do not edit by hand)
```

## 9. Build & run

The Docker image is built from the **repo root** as the build context (it needs
the shared proto from `api-svc/proto` plus the `prediction-svc/` sources).

```bash
# From the repo root — start the whole stack (db, prediction-svc, auth-svc, api-svc, web-svc, gateway, pgadmin)
docker compose --env-file .env -f deploy/docker-compose.yaml up -d

# Logs for this service
docker compose -f deploy/docker-compose.yaml logs -f prediction-svc
```

`deploy/prediction-svc.Dockerfile` is a two-stage build:

1. **proto-builder** — regenerates the Python gRPC stubs from
   `api-svc/proto/prediction/prediction.proto`.
2. **runtime** — `python:3.12-slim`, installs the package with `[dev,ml]`
   extras, copies `src/` + `tests/`, runs as non-root, exposes `:8119`.

Local run (without Docker):

```bash
cd prediction-svc
make install          # pip install -e ".[dev]"
make run              # python -m src.main
```

Regenerate proto stubs:

```bash
cd prediction-svc
make proto            # runs scripts/gen_proto.sh against api-svc/proto
```

## 10. Tests

```bash
cd prediction-svc

make test-unit            # unit tests (no DB needed) — algorithms, features, market calendar, simulation
make test-integration     # integration tests (need the running stack)

# Phase suites (run inside the container)
make test-phase1 .. make test-phase5
make test-simulation

# Or run directly inside the container
docker exec prediction-svc python -m pytest tests/ -v
```

- Unit tests use mock data; optional-dependency tests skip gracefully
  (`pytest.importorskip("torch")`).
- Integration / phase tests target the running Docker stack and use
  `docker exec prediction-svc ...`.

## 11. How to extend

### Add a new algorithm

1. Create `src/algorithms/<name>.py` implementing `PredictionAlgorithm`
   (`predict`, `get_name`, `get_key`). Apply the market-aware clamp:

   ```python
   from src.algorithms.base import PredictionAlgorithm, PredictionResult, get_max_change_pct

   class MyAlgorithm(PredictionAlgorithm):
       def predict(self, prices, volumes=None):
           current = prices[-1]
           predicted = ...  # your logic
           max_change = current * get_max_change_pct(self._market_key)
           predicted = max(current - max_change, min(current + max_change, predicted))
           return PredictionResult(predicted, 0.5, current, self.get_key())
       def get_name(self): return "My Algorithm"
       def get_key(self): return "my_algo"
   ```

2. Register it in `src/algorithms/registry.py` `build_algorithms()` (add the
   instance to the `instances` dict; optionally pass it into `EnsemblePredictor`).
3. Add metadata in the Go API registry
   `api-svc/pkg/service/predict/registry/algorithms.go` so
   `/api/training/algorithms` lists it.
4. Tree-based models can reuse `build_enhanced_features()` from `features.py`.

### Add a new market

1. Add DB models + migration: Go GORM struct in `api-svc/pkg/models/models_db/`
   and the matching SQLAlchemy model in `src/database/models.py`
   (plus schema in the root `database.sql`).
2. Add repository methods in `src/database/repository.py` (and Go side if needed).
3. Create a crawler in `src/crawlers/<name>.py` implementing `BaseCrawler`.
4. Register a cron job in `src/scheduler/jobs.py` (and `DEFAULT_SCHEDULES` in
   `scheduler/manager.py`).
5. Wire prediction logic into `orchestrator/runner.py`.
6. Add a trigger gRPC RPC + Go endpoint if a manual trigger is required.

## 12. Timezone — ICT-at-rest

All DB timestamps are stored as **Asia/Ho_Chi_Minh wallclock**
(`TIMESTAMP WITHOUT TIME ZONE`, no UTC conversion). In this service:

- The container sets `TZ=Asia/Ho_Chi_Minh`, so **always use `datetime.now()`**.
- **Never use `datetime.utcnow()`** — it would write UTC and be 7 hours off the
  intended business value.
