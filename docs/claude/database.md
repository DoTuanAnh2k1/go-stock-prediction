# Database & Cron Schedules

## Database

- **Engine:** TimescaleDB (PostgreSQL 16) — image `timescale/timescaledb:latest-pg16`, container `timescaledb`, port 5432
- **ORM:** GORM v2 (`api-svc/pkg/store/postgres/`)
- **Auto-migrate:** Tắt — schema do `database.sql` quản lý (hypertable composite PK xung đột AutoMigrate). Init tự động qua mount `/docker-entrypoint-initdb.d/01-schema.sql`.
- **Schema đầy đủ:** `database.sql` ở root (BIGSERIAL, NUMERIC, TIMESTAMP, NOW())

### Tables chính

| Table | Loại | Ghi chú |
|-------|------|---------|
| `gold_prices`, `nasdaq_prices`, `sp500_prices`, `crypto_prices` | Hypertable | Composite PK (id + time column) |
| `*_intraday_prices` | Hypertable | — |
| `gold_predictions`, `nasdaq_predictions`, `sp500_predictions`, `crypto_predictions` | Hypertable | partition by `prediction_date`; có cột `direction_correct *bool` nullable |
| `sim_trades` | Hypertable | partition by `trade_date`; thêm cột `trade_at TIMESTAMP` (backfill = trade_date) |
| `sim_portfolio_snapshots` | Hypertable | partition by `snapshot_date`; thêm cột `snapshot_at TIMESTAMP` — upsert theo giờ, unique index `uq_sim_snap_session_at (session_id, snapshot_at)` |
| `sync_logs`, `training_logs` | Hypertable | partition by `created_at` |
| `training_metrics` | — | — |
| `users` | — | Flyway V2: thêm `full_name`, `email`, `phone` nullable |
| `cron_schedules` | — | `job_key` PK, `cron_expression`, `enabled`, `updated_at` |
| `pipeline_reports` | — | Không hypertable; retention 7 ngày tự động; cột: `pipeline_key`, `status`, `steps` JSONB, `duration_ms`, `crawled_count`, `predictions_count`, `trained` |
| `pipeline_crawl_counters` | — | Không hypertable; `market_key` PK, `crawl_count` BIGINT; atomic upsert + RETURNING |
| `service_instances` | — | service-mgt; 4 dòng UP khi `SERVICE_MGT_ENABLED=true` |
| `market_groups`, `market_group_markets`, `user_market_groups` | — | Flyway V1; quản lý bởi Java Auth Service |
| `cli_handlers`, `commands`, `command_groups`, `command_group_commands`, `user_command_groups` | — | Flyway V3; Command RBAC |
| `sim_bots` | — | Cột `symbol` nullable — NULL = pooled per-market; có giá trị = per-symbol (algo key với hậu tố `__ps`). Cột `trailing_stop BOOLEAN NOT NULL DEFAULT FALSE` — `TRUE` = SL tính theo đỉnh giá intraday kể từ lúc entry (stateless, `MAX(price)` trên `*_intraday_prices` mỗi step, KHÔNG lưu peak trong DB) thay vì entry price cố định; TP không đổi (vẫn entry-anchored). Hai variant pooled `_v11` ("Trailing Tight", sl=3%, tp=99% — gần như chỉ trail) và `_v12` ("Trailing Wide", sl=6%, tp=20%) seed cho mọi (market × algorithm) — 4×12×2=96 bot, chỉ pooled (không seed per-symbol). |
| `stock_fundamentals` | — | Không hypertable; 1 row/(market_key, symbol) — snapshot báo cáo tài chính mới nhất (yfinance, upsert ON CONFLICT); unique `uq_fundamentals_market_symbol`; dùng bởi `transformer_nn` |
| `stock_splits` | — | Không hypertable; cột: `id, market_key, symbol, split_date, ratio, numerator, denominator, applied_at, created_at`; unique `uq_stock_splits_sym_date (market_key, symbol, split_date)`; `applied_at NULL` = chưa apply history adjustment; chỉ NASDAQ/SP500 |

### Conventions quan trọng

- **ORM column:** Cột volume của `crypto_prices` là `volume24h` (Go `Volume24h`, Python `volume24h`)
- **OHLC columns (candlestick):** `nasdaq_prices`/`sp500_prices` + intraday đã có `open_price`/`high_price`/`low_price`/`close_price` (OHLCV) từ đầu. `crypto_prices`/`crypto_intraday_prices` và `gold_prices`/`gold_intraday_prices` được thêm `open_price`/`high_price`/`low_price` **nullable** (ALTER ADD COLUMN IF NOT EXISTS) cho biểu đồ nến. Crypto: crawler lấy OHLC từ CoinGecko `/coins/{id}/ohlc` (aggregate về daily/hourly, ICT). Gold: chỉ source `XAU` có OHLC (Yahoo GC=F); nguồn VN (SJC/BTMC/Phú Quý) để NULL, `close_price`≈`sell_price`. Cột cũ chưa backfill = NULL → backfill qua trigger `crypto-history`/`gold-history`.
- **direction_correct:** NULL = chưa reconcile; 1 = hướng đúng; 0 = hướng sai
- **sim_portfolio_snapshots — upsert theo giờ:** `INSERT ... ON CONFLICT (session_id, snapshot_at) DO UPDATE` — 1 row/session/giờ. `CREATE UNIQUE INDEX` phải TÁCH RIÊNG khỏi DML transaction (Timescale sẽ rollback cả block nếu gộp)
- **RBAC:** `flyway-database-postgresql` (Flyway 10.x) để nhận diện PG16
- **sim_bots.symbol:** `idx_sim_bots_market_symbol`; GORM `Symbol *string`; Python `symbol`
- **stock_splits.applied_at:** NULL = chưa chỉnh intraday; non-NULL = đã apply (hoặc non-cliff, không cần chỉnh). Source of truth idempotent — `record_split` dùng INSERT ON CONFLICT DO NOTHING.

## Cron Schedules

Lưu trong bảng `cron_schedules`, chỉnh live qua `/api/schedules` hoặc Settings page — **không cần restart**.

**Nguồn sự thật:** `DEFAULT_SCHEDULES` trong `prediction-svc/src/scheduler/manager.py` — upsert mỗi lần Python khởi động (ghi đè DB nếu khác code). Go `seedCronSchedules()` chỉ insert-if-not-exists.

**Ngoại lệ `daily_backup`:** Nguồn sự thật là `backup_scheduler.go` (Go, insert-if-not-exists). Chỉnh được live qua API; Go poll 60s.

### Danh sách jobs hiện tại

| Job Key | Schedule mặc định | Enabled | Công việc |
|---------|-------------------|---------|-----------|
| `daily_reconcile` | `0 0 6 * * *` | bật | Reconcile tất cả markets (catch-all) |
| `crawler_gold` | `0 0 * * * *` | bật | Pipeline Gold: crawl → train/10 → predict → reconcile → report |
| `crawler_nasdaq` | `0 15 * * * 1-5` | bật | Pipeline NASDAQ (phút 15, T2-T6) |
| `crawler_sp500` | `0 0,30 * * * 1-5` | bật | Pipeline S&P 500 (phút 0 và 30, T2-T6) |
| `crawler_crypto` | `0 0 * * * *` | bật | Pipeline Crypto (mỗi giờ phút 0) |
| `train_gold` | `0 0 3 * * 0` | bật | Training Gold (Chủ nhật 3AM) |
| `train_nasdaq` | `0 0 4 * * 0` | bật | Training NASDAQ (Chủ nhật 4AM) |
| `train_crypto` | `0 0 5 * * 0` | bật | Training Crypto (Chủ nhật 5AM) |
| `train_sp500` | `0 0 7 * * 0` | bật | Training S&P 500 (Chủ nhật 7AM) |
| `train_meta` | `0 0 8 * * 0` | bật | Training Meta-Stack LightGBM classifier cho 4 markets (Chủ nhật 8AM) |
| `crawler_fundamentals` | `0 0 6 * * 6` | bật | Crawl báo cáo tài chính yfinance → `stock_fundamentals` (Thứ Bảy 6AM) — feeds `transformer_nn` |
| `train_transformer` | `0 30 2 * * 1,3,5` | bật | Refresh transformer_nn trên intraday bars (Thứ 2/4/6 2:30AM — giữa các lần train Chủ nhật) |
| `simulation_daily` | `0 0 20 * * *` | bật | Bot trading (8PM); live-step theo giờ; NASDAQ/SP500 skip nếu is_intraday_open=False |
| `daily_backup` | `0 0 3 * * *` | bật | Backup PostgreSQL lúc 3AM — **Go api-svc** chạy, không phải Python |
| `gold_predict`, `predict_nasdaq`, `predict_crypto`, `predict_sp500` | — | **tắt** | Disabled — đã chạy trong pipeline |
| `weekly_training`, `daily_prediction` | — | **tắt** | Disabled — thay bằng per-market jobs |

### Pipeline logic (_run_pipeline trong jobs.py)

1. Crawl — skip nếu `is_market_open` = False
2. Increment counter per-market (DB atomic) → mỗi 10 lần → `train_for_market()`
3. `run_for_market()` — NASDAQ/SP500 skip predict nếu `is_intraday_open` = False; `target = now + 1h`
4. `reconcile_predictions(only_market=market_key)` — chấm prediction đã chín (`target_date <= now`)
5. Ghi `pipeline_reports` row
6. `delete_old_pipeline_reports(7)` — retention tự động

### Bot RL DQN

`simulation/bot.py` nhánh rl_dqn: action policy trực tiếp (không threshold); SL/TP vẫn là hard guard. Observation = `build_enhanced_features()` + position_state. BUY chỉ thực thi khi softmax confidence ≥ `_RL_BUY_CONF_FLOOR` (0.38 — trên uniform 1/3, chống churn); SELL không gate. Seeder: **1 bot RL/market** (4 tổng). Checkpoint: `${RL_MODEL_DIR}/rl_dqn_{market}.pt`.

**RL DQN v3 (training):** Dueling Double-DQN — LayerNorm input + static feature scaling (RSI/stoch/ROC về cùng scale với log-returns, tag `feat_scale` trong checkpoint; checkpoint MLP cũ vẫn load và nhận raw features). Prioritized replay (α=0.6, β=0.4) + 3-step return + Polyak soft target (τ=0.005). Reward = PnL chuẩn hóa theo rolling σ + shaping định hướng (±0.15·direction·return/σ) để Q(buy/sell|flat) mang tín hiệu hướng → predict() chính xác hơn. Walk-forward validation: window mới nhất mỗi series bị hold-out, giữ weights của epoch có greedy reward (RAW) tốt nhất. Fix alignment: `_make_windows` tail-align prices với feature rows (trước đây reward lệch 20 bước so với observation). predict(): độ lớn dự đoán tỷ lệ với softmax gap `p_buy − p_sell`; HOLD nghiêng theo phía trội thay vì luôn nghiêng lên.

### Bot Conviction (transformer_nn)

`simulation/bot.py` nhánh is_conviction (`base_key == "transformer_nn"`): KHÔNG dùng threshold %-giá (prediction hourly chỉ ±0.1–0.3%, không bao giờ chạm ngưỡng thang ngày như 0.5%) — quyết định bằng **P(up) từ direction head** (`TransformerPredictor.predict_direction_proba`, None = hold). BUY khi `p_up ≥ max(0.5 + buy_threshold/100, 0.52)`; SELL khi đang giữ và `p_up ≤ min(0.5 − sell_threshold/100, 0.48)` (floor/ceiling 0.52/0.48 validate bằng trade replay 28/6→2/7: +1.16%/+1.29% vs B&H +0.84%). Dữ liệu intraday khi có `now` (live-step), daily khi backtest — như RL. SL/TP (kể cả trailing _v11/_v12) vẫn là hard guard chạy trước.

### Bot Meta-Stack

`simulation/bot.py` nhánh is_meta (`base_key == "meta_stack"`): đọc tất cả dự đoán hiện tại của các algo + direction accuracy rolling (K=40 lần, as-of t, chống leakage) → `MetaStackModel.predict_proba()` → P(up) đã calibrate; quyết định BUY nếu `p > 0.5 + δ`, SELL nếu `p < 0.5 - δ` và đang giữ vị thế, size theo conviction `(p−0.5)/0.5 × max_position_pct` (dùng `Portfolio.buy(position_pct=...)`). SL/TP là hard guard như mọi bot. Fallback: reliability-weighted vote khi thiếu LightGBM/checkpoint. Seeder: **1 bot `meta_stack`/market** (4 tổng pooled); `meta_stack__ps` per-symbol khi `PER_SYMBOL_ENABLED=true`. Checkpoint: `${RL_MODEL_DIR}/meta_{market}.pkl` (pooled) / `meta_{market}_{symbol}.pkl` (per-symbol). Leaderboard/monitoring tự hiện vì là rows `sim_bots` thường.
