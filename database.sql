-- ============================================================
-- FINANCIAL PREDICTION SYSTEM — PostgreSQL + TimescaleDB Schema
-- Replaces legacy MySQL VN-stock schema.
-- ICT-at-rest: ALL TIMESTAMP columns store Asia/Ho_Chi_Minh
-- wallclock time without timezone conversion (plain TIMESTAMP).
-- ============================================================

-- Enable TimescaleDB
CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;

-- ============================================================
-- OPERATIONAL / METADATA TABLES
-- ============================================================

-- sync_logs: track crawl / data-sync runs
CREATE TABLE IF NOT EXISTS sync_logs (
    id            BIGSERIAL    PRIMARY KEY,
    sync_date     TIMESTAMP    NOT NULL,
    success_count INT          NOT NULL DEFAULT 0,
    error_count   INT          NOT NULL DEFAULT 0,
    duration_ms   BIGINT       NOT NULL,
    source        VARCHAR(50),
    error_message TEXT,
    created_at    TIMESTAMP    NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP    NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_sync_logs_sync_date  ON sync_logs(sync_date);
CREATE INDEX IF NOT EXISTS idx_sync_logs_source     ON sync_logs(source);
CREATE INDEX IF NOT EXISTS idx_sync_logs_deleted_at ON sync_logs(deleted_at);

-- macro_indicators: economic indicator time-series
CREATE TABLE IF NOT EXISTS macro_indicators (
    id             BIGSERIAL      PRIMARY KEY,
    indicator_name VARCHAR(30)    NOT NULL,
    indicator_date TIMESTAMP      NOT NULL,
    value          NUMERIC(20,6)  NOT NULL,
    source         VARCHAR(50),
    created_at     TIMESTAMP      NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMP      NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_macro_name_date
    ON macro_indicators(indicator_name, indicator_date);

-- training_logs: per-session per-algorithm training records
CREATE TABLE IF NOT EXISTS training_logs (
    id             BIGSERIAL    PRIMARY KEY,
    session_id     VARCHAR(36)  NOT NULL,
    algorithm_name VARCHAR(50)  NOT NULL,
    market_key     VARCHAR(20)  NOT NULL DEFAULT 'gold',
    total_stocks   INT          NOT NULL,
    success_count  INT          NOT NULL,
    error_count    INT          NOT NULL,
    accuracy       NUMERIC(5,2),
    duration_ms    BIGINT       NOT NULL,
    error_details  TEXT,
    started_at     TIMESTAMP    NOT NULL,
    completed_at   TIMESTAMP    NOT NULL,
    created_at     TIMESTAMP    NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMP    NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_training_logs_session_id    ON training_logs(session_id);
CREATE INDEX IF NOT EXISTS idx_training_logs_algorithm     ON training_logs(algorithm_name);
CREATE INDEX IF NOT EXISTS idx_training_logs_market_key    ON training_logs(market_key);
CREATE INDEX IF NOT EXISTS idx_training_logs_started_at    ON training_logs(started_at);
CREATE INDEX IF NOT EXISTS idx_training_logs_deleted_at    ON training_logs(deleted_at);

-- cron_schedules: DB-backed dynamic cron configuration
CREATE TABLE IF NOT EXISTS cron_schedules (
    id              BIGSERIAL    PRIMARY KEY,
    job_key         VARCHAR(100) NOT NULL UNIQUE,
    job_name        VARCHAR(200) NOT NULL,
    cron_expression VARCHAR(100) NOT NULL,
    enabled         BOOLEAN      NOT NULL DEFAULT TRUE,
    updated_at      TIMESTAMP    NOT NULL DEFAULT NOW()
);

-- users: application user accounts (GORM managed; Java Auth Service also writes here)
CREATE TABLE IF NOT EXISTS users (
    id            BIGSERIAL    PRIMARY KEY,
    created_at    TIMESTAMP    NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP    NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMP,
    username      VARCHAR(50)  NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role          VARCHAR(255) NOT NULL DEFAULT 'user',
    full_name     VARCHAR(100),
    email         VARCHAR(255),
    phone         VARCHAR(30)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username   ON users(username);
CREATE INDEX        IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);

-- ============================================================
-- AUTH / RBAC TABLES  (Flyway V1+V2 — using PostgreSQL syntax)
-- Java Auth Service owns these; CREATE IF NOT EXISTS is safe.
-- ============================================================

CREATE TABLE IF NOT EXISTS market_groups (
    id          BIGSERIAL    PRIMARY KEY,
    name        VARCHAR(100) NOT NULL,
    description TEXT,
    created_at  TIMESTAMP    NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP    NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_market_groups_name UNIQUE (name)
);

CREATE TABLE IF NOT EXISTS market_group_markets (
    group_id   BIGINT      NOT NULL REFERENCES market_groups(id) ON DELETE CASCADE,
    market_key VARCHAR(20) NOT NULL,
    PRIMARY KEY (group_id, market_key)
);

CREATE TABLE IF NOT EXISTS user_market_groups (
    user_id  BIGINT NOT NULL,
    group_id BIGINT NOT NULL REFERENCES market_groups(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, group_id)
);
CREATE INDEX IF NOT EXISTS idx_umg_user_id ON user_market_groups(user_id);

-- ============================================================
-- GOLD TABLES
-- ============================================================

-- gold_prices: daily buy/sell prices per source × product
CREATE TABLE IF NOT EXISTS gold_prices (
    id           BIGSERIAL      PRIMARY KEY,
    source       VARCHAR(10)    NOT NULL,
    product_type VARCHAR(20)    NOT NULL,
    trading_date TIMESTAMP      NOT NULL,
    buy_price    NUMERIC(15,2),
    sell_price   NUMERIC(15,2),
    currency     VARCHAR(3)     NOT NULL,
    created_at   TIMESTAMP      NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMP      NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_gold_source_product_date
    ON gold_prices(source, product_type, trading_date);
CREATE INDEX IF NOT EXISTS idx_gold_prices_deleted_at ON gold_prices(deleted_at);

-- gold_intraday_prices: sub-daily gold price ticks
CREATE TABLE IF NOT EXISTS gold_intraday_prices (
    id           BIGSERIAL      PRIMARY KEY,
    source       VARCHAR(50)    NOT NULL,
    product_type VARCHAR(50)    NOT NULL,
    timestamp    TIMESTAMP      NOT NULL,
    buy_price    NUMERIC(15,2),
    sell_price   NUMERIC(15,2),
    currency     VARCHAR(3)     NOT NULL,
    created_at   TIMESTAMP      NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMP      NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_gold_intraday_src_prod_ts
    ON gold_intraday_prices(source, product_type, timestamp);

-- gold_predictions: ML algorithm predictions for gold
CREATE TABLE IF NOT EXISTS gold_predictions (
    id                BIGSERIAL      PRIMARY KEY,
    source            VARCHAR(50)    NOT NULL,
    product_type      VARCHAR(50)    NOT NULL,
    predicted_price   NUMERIC(20,2)  NOT NULL,
    current_price     NUMERIC(20,2)  NOT NULL,
    confidence        NUMERIC(5,4),
    algorithm_name    VARCHAR(50)    NOT NULL,
    prediction_date   TIMESTAMP      NOT NULL,
    target_date       TIMESTAMP      NOT NULL,
    actual_price      NUMERIC(20,2),
    accuracy          NUMERIC(5,4),
    direction_correct BOOLEAN,
    status            VARCHAR(20)    NOT NULL DEFAULT 'pending',
    created_at        TIMESTAMP      NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMP      NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_gold_pred_src_type_algo
    ON gold_predictions(source, product_type, algorithm_name);
CREATE INDEX IF NOT EXISTS idx_gold_pred_prediction_date ON gold_predictions(prediction_date);
CREATE INDEX IF NOT EXISTS idx_gold_pred_target_date     ON gold_predictions(target_date);
CREATE INDEX IF NOT EXISTS idx_gold_pred_deleted_at      ON gold_predictions(deleted_at);
CREATE INDEX IF NOT EXISTS idx_gold_pred_direction
    ON gold_predictions(algorithm_name, created_at)
    WHERE direction_correct IS NOT NULL;

-- ============================================================
-- NASDAQ TABLES
-- ============================================================

-- nasdaq_prices: daily OHLCV for NASDAQ 100 symbols
CREATE TABLE IF NOT EXISTS nasdaq_prices (
    id           BIGSERIAL      PRIMARY KEY,
    symbol       VARCHAR(10)    NOT NULL,
    company_name VARCHAR(200),
    open_price   NUMERIC(15,4),
    high_price   NUMERIC(15,4),
    low_price    NUMERIC(15,4),
    close_price  NUMERIC(15,4)  NOT NULL,
    volume       BIGINT,
    trading_date TIMESTAMP      NOT NULL,
    currency     VARCHAR(3)     NOT NULL DEFAULT 'USD',
    created_at   TIMESTAMP      NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMP      NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_nasdaq_symbol_date
    ON nasdaq_prices(symbol, trading_date);
CREATE INDEX IF NOT EXISTS idx_nasdaq_prices_deleted_at ON nasdaq_prices(deleted_at);

-- nasdaq_intraday_prices: sub-daily OHLCV for NASDAQ symbols
CREATE TABLE IF NOT EXISTS nasdaq_intraday_prices (
    id          BIGSERIAL      PRIMARY KEY,
    symbol      VARCHAR(20)    NOT NULL,
    timestamp   TIMESTAMP      NOT NULL,
    open_price  NUMERIC(20,6),
    high_price  NUMERIC(20,6),
    low_price   NUMERIC(20,6),
    close_price NUMERIC(20,6),
    volume      BIGINT,
    created_at  TIMESTAMP      NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP      NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_nasdaq_intraday_symbol_ts
    ON nasdaq_intraday_prices(symbol, timestamp);

-- nasdaq_predictions: ML predictions for NASDAQ symbols
CREATE TABLE IF NOT EXISTS nasdaq_predictions (
    id                BIGSERIAL      PRIMARY KEY,
    symbol            VARCHAR(10)    NOT NULL,
    algorithm_name    VARCHAR(100)   NOT NULL,
    predicted_price   NUMERIC(15,4)  NOT NULL,
    current_price     NUMERIC(15,4)  NOT NULL,
    confidence        NUMERIC(5,4),
    prediction_date   TIMESTAMP      NOT NULL,
    target_date       TIMESTAMP      NOT NULL,
    actual_price      NUMERIC(15,4),
    accuracy          NUMERIC(5,4),
    direction_correct BOOLEAN,
    status            VARCHAR(20)    NOT NULL DEFAULT 'pending',
    created_at        TIMESTAMP      NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMP      NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_nasdaq_pred ON nasdaq_predictions(symbol, algorithm_name);
CREATE INDEX IF NOT EXISTS idx_nasdaq_pred_prediction_date ON nasdaq_predictions(prediction_date);
CREATE INDEX IF NOT EXISTS idx_nasdaq_pred_target_date     ON nasdaq_predictions(target_date);
CREATE INDEX IF NOT EXISTS idx_nasdaq_pred_deleted_at      ON nasdaq_predictions(deleted_at);
CREATE INDEX IF NOT EXISTS idx_nasdaq_pred_direction
    ON nasdaq_predictions(algorithm_name, created_at)
    WHERE direction_correct IS NOT NULL;

-- ============================================================
-- S&P 500 TABLES
-- ============================================================

-- sp500_prices: daily OHLCV for S&P 500 symbols
CREATE TABLE IF NOT EXISTS sp500_prices (
    id           BIGSERIAL      PRIMARY KEY,
    symbol       VARCHAR(10)    NOT NULL,
    company_name VARCHAR(200),
    open_price   NUMERIC(15,4),
    high_price   NUMERIC(15,4),
    low_price    NUMERIC(15,4),
    close_price  NUMERIC(15,4)  NOT NULL,
    volume       BIGINT,
    trading_date TIMESTAMP      NOT NULL,
    currency     VARCHAR(3)     NOT NULL DEFAULT 'USD',
    created_at   TIMESTAMP      NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMP      NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sp500_symbol_date
    ON sp500_prices(symbol, trading_date);
CREATE INDEX IF NOT EXISTS idx_sp500_prices_deleted_at ON sp500_prices(deleted_at);

-- sp500_intraday_prices: sub-daily OHLCV for S&P 500 symbols
CREATE TABLE IF NOT EXISTS sp500_intraday_prices (
    id          BIGSERIAL      PRIMARY KEY,
    symbol      VARCHAR(20)    NOT NULL,
    timestamp   TIMESTAMP      NOT NULL,
    open_price  NUMERIC(20,6),
    high_price  NUMERIC(20,6),
    low_price   NUMERIC(20,6),
    close_price NUMERIC(20,6),
    volume      BIGINT,
    created_at  TIMESTAMP      NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP      NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sp500_intraday_symbol_ts
    ON sp500_intraday_prices(symbol, timestamp);

-- sp500_predictions: ML predictions for S&P 500 symbols
CREATE TABLE IF NOT EXISTS sp500_predictions (
    id                BIGSERIAL      PRIMARY KEY,
    symbol            VARCHAR(10)    NOT NULL,
    algorithm_name    VARCHAR(100)   NOT NULL,
    predicted_price   NUMERIC(15,4)  NOT NULL,
    current_price     NUMERIC(15,4)  NOT NULL,
    confidence        NUMERIC(5,4),
    prediction_date   TIMESTAMP      NOT NULL,
    target_date       TIMESTAMP      NOT NULL,
    actual_price      NUMERIC(15,4),
    accuracy          NUMERIC(5,4),
    direction_correct BOOLEAN,
    status            VARCHAR(20)    NOT NULL DEFAULT 'pending',
    created_at        TIMESTAMP      NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMP      NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_sp500_pred ON sp500_predictions(symbol, algorithm_name);
CREATE INDEX IF NOT EXISTS idx_sp500_pred_prediction_date ON sp500_predictions(prediction_date);
CREATE INDEX IF NOT EXISTS idx_sp500_pred_target_date     ON sp500_predictions(target_date);
CREATE INDEX IF NOT EXISTS idx_sp500_pred_deleted_at      ON sp500_predictions(deleted_at);
CREATE INDEX IF NOT EXISTS idx_sp500_pred_direction
    ON sp500_predictions(algorithm_name, created_at)
    WHERE direction_correct IS NOT NULL;

-- ============================================================
-- CRYPTO TABLES
-- ============================================================

-- crypto_prices: daily price data for tracked cryptocurrencies
CREATE TABLE IF NOT EXISTS crypto_prices (
    id           BIGSERIAL      PRIMARY KEY,
    coin_id      VARCHAR(50)    NOT NULL,
    symbol       VARCHAR(10)    NOT NULL,
    close_price  NUMERIC(20,2)  NOT NULL,
    market_cap   NUMERIC(30,2),
    volume24h    NUMERIC(30,2),
    trading_date TIMESTAMP      NOT NULL,
    currency     VARCHAR(3)     NOT NULL DEFAULT 'USD',
    created_at   TIMESTAMP      NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMP      NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_crypto_coin_date
    ON crypto_prices(coin_id, trading_date);
CREATE INDEX IF NOT EXISTS idx_crypto_prices_deleted_at ON crypto_prices(deleted_at);

-- crypto_intraday_prices: sub-daily price/market-cap/volume for cryptos
CREATE TABLE IF NOT EXISTS crypto_intraday_prices (
    id         BIGSERIAL      PRIMARY KEY,
    coin_id    VARCHAR(50)    NOT NULL,
    timestamp  TIMESTAMP      NOT NULL,
    price      NUMERIC(30,8),
    market_cap NUMERIC(30,2),
    volume     NUMERIC(30,2),
    created_at TIMESTAMP      NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP      NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_crypto_intraday_coin_ts
    ON crypto_intraday_prices(coin_id, timestamp);

-- crypto_predictions: ML predictions for tracked cryptos
CREATE TABLE IF NOT EXISTS crypto_predictions (
    id                BIGSERIAL      PRIMARY KEY,
    coin_id           VARCHAR(50)    NOT NULL,
    symbol            VARCHAR(10)    NOT NULL,
    algorithm_name    VARCHAR(100)   NOT NULL,
    predicted_price   NUMERIC(20,2)  NOT NULL,
    current_price     NUMERIC(20,2)  NOT NULL,
    confidence        NUMERIC(5,4),
    prediction_date   TIMESTAMP      NOT NULL,
    target_date       TIMESTAMP      NOT NULL,
    actual_price      NUMERIC(20,2),
    accuracy          NUMERIC(5,4),
    direction_correct BOOLEAN,
    status            VARCHAR(20)    NOT NULL DEFAULT 'pending',
    created_at        TIMESTAMP      NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMP      NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_crypto_pred ON crypto_predictions(coin_id, algorithm_name);
CREATE INDEX IF NOT EXISTS idx_crypto_pred_prediction_date ON crypto_predictions(prediction_date);
CREATE INDEX IF NOT EXISTS idx_crypto_pred_target_date     ON crypto_predictions(target_date);
CREATE INDEX IF NOT EXISTS idx_crypto_pred_deleted_at      ON crypto_predictions(deleted_at);
CREATE INDEX IF NOT EXISTS idx_crypto_pred_direction
    ON crypto_predictions(algorithm_name, created_at)
    WHERE direction_correct IS NOT NULL;

-- ============================================================
-- SIMULATION TABLES
-- ============================================================

-- sim_bots: trading bot configuration (market × algorithm pairs)
CREATE TABLE IF NOT EXISTS sim_bots (
    id               VARCHAR(50)   PRIMARY KEY,
    market           VARCHAR(20)   NOT NULL,
    algorithm        VARCHAR(50)   NOT NULL,
    symbol           VARCHAR(30),  -- NULL = pooled per-market bot; set = per-symbol bot scoped to this symbol
    display_name     VARCHAR(100)  NOT NULL,
    initial_capital  NUMERIC(20,2) NOT NULL,
    currency         VARCHAR(5)    NOT NULL,
    buy_threshold    NUMERIC(5,2)  DEFAULT 1.50,
    sell_threshold   NUMERIC(5,2)  DEFAULT 1.00,
    min_confidence   NUMERIC(4,2)  DEFAULT 0.60,
    stop_loss        NUMERIC(5,2)  DEFAULT 5.00,
    take_profit      NUMERIC(5,2)  DEFAULT 8.00,
    max_position_pct NUMERIC(5,2)  DEFAULT 15.00,
    max_positions    INT           DEFAULT 5,
    is_active        BOOLEAN       DEFAULT TRUE,
    created_at       TIMESTAMP     NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMP     NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_sim_bots_market ON sim_bots(market);
CREATE INDEX IF NOT EXISTS idx_sim_bots_market_symbol ON sim_bots(market, symbol);
-- Idempotent add for existing databases (schema is mounted only on first init)
ALTER TABLE sim_bots ADD COLUMN IF NOT EXISTS symbol VARCHAR(30);
-- trailing_stop: TRUE = SL is computed off the intraday peak price since entry
-- (stateless — recomputed every step from *_intraday_prices MAX) instead of
-- the fixed entry price. See _v11/_v12 pooled variants in seeder.py.
ALTER TABLE sim_bots ADD COLUMN IF NOT EXISTS trailing_stop BOOLEAN NOT NULL DEFAULT FALSE;

-- sim_sessions: backtest and live simulation runs per bot
CREATE TABLE IF NOT EXISTS sim_sessions (
    id               BIGSERIAL     PRIMARY KEY,
    bot_id           VARCHAR(50)   NOT NULL REFERENCES sim_bots(id),
    start_date       TIMESTAMP     NOT NULL,
    end_date         TIMESTAMP,
    status           VARCHAR(20)   NOT NULL DEFAULT 'running',
    mode             VARCHAR(20)   NOT NULL DEFAULT 'backtest',
    -- Pre-computed KPI columns (populated by Python after session end)
    total_trades     INT           DEFAULT 0,
    wins             INT           DEFAULT 0,
    losses           INT           DEFAULT 0,
    breakeven        INT           DEFAULT 0,
    total_pnl        NUMERIC(20,2) DEFAULT 0,
    total_return_pct NUMERIC(8,4),
    win_rate         NUMERIC(5,4),
    profit_factor    NUMERIC(8,4),
    max_drawdown_pct NUMERIC(8,4),
    kpi_updated_at   TIMESTAMP,
    created_at       TIMESTAMP     NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_sim_sessions_bot_id ON sim_sessions(bot_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_sim_sessions_status ON sim_sessions(status, mode);

-- sim_trades: individual BUY/SELL trade records
CREATE TABLE IF NOT EXISTS sim_trades (
    id              BIGSERIAL      PRIMARY KEY,
    session_id      BIGINT         NOT NULL REFERENCES sim_sessions(id),
    bot_id          VARCHAR(50)    NOT NULL,
    symbol          VARCHAR(20)    NOT NULL,
    action          VARCHAR(5)     NOT NULL,
    quantity        NUMERIC(20,6)  NOT NULL,
    price           NUMERIC(20,4)  NOT NULL,
    trade_value     NUMERIC(20,2)  NOT NULL,
    signal_strength NUMERIC(8,4),
    confidence      NUMERIC(4,3),
    trade_date      TIMESTAMP      NOT NULL,
    close_reason    VARCHAR(20),
    entry_trade_id  BIGINT,
    pnl             NUMERIC(20,2),
    pnl_pct         NUMERIC(8,4),
    created_at      TIMESTAMP      NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_sim_trades_session ON sim_trades(session_id, trade_date);
CREATE INDEX IF NOT EXISTS idx_sim_trades_bot     ON sim_trades(bot_id, trade_date DESC);
CREATE INDEX IF NOT EXISTS idx_sim_trades_created ON sim_trades(created_at DESC);

-- sim_portfolio_snapshots: daily portfolio state snapshots per bot/session
CREATE TABLE IF NOT EXISTS sim_portfolio_snapshots (
    id               BIGSERIAL      PRIMARY KEY,
    session_id       BIGINT         NOT NULL REFERENCES sim_sessions(id),
    bot_id           VARCHAR(50)    NOT NULL,
    snapshot_date    TIMESTAMP      NOT NULL,
    cash_balance     NUMERIC(20,2)  NOT NULL,
    positions_value  NUMERIC(20,2)  NOT NULL,
    total_value      NUMERIC(20,2)  NOT NULL,
    total_return_pct NUMERIC(8,4),
    open_positions   INT            DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_sim_snap_session_date
    ON sim_portfolio_snapshots(session_id, snapshot_date DESC);
CREATE INDEX IF NOT EXISTS idx_sim_snap_bot_date
    ON sim_portfolio_snapshots(bot_id, snapshot_date DESC);
-- Exactly one snapshot per session per day. Upsert target for live-step
-- (ON CONFLICT) — prevents duplicate intraday rows from hourly pipeline runs.
-- Includes the hypertable partition column (snapshot_date) as required.
CREATE UNIQUE INDEX IF NOT EXISTS uq_sim_snap_session_date
    ON sim_portfolio_snapshots(session_id, snapshot_date);

-- ============================================================
-- TIMESCALEDB HYPERTABLES
-- Chunk intervals chosen by data frequency:
--   daily price/pred  → 1 month (INTERVAL '1 month')
--   intraday          → 1 week  (INTERVAL '7 days')
--   predictions       → 3 months
--   sim time-series   → 3 months
--   operational logs  → 1 month
-- ============================================================

-- ============================================================
-- FIX PKs: TimescaleDB requires partition column in every PK/unique index.
-- ============================================================
ALTER TABLE sync_logs                DROP CONSTRAINT IF EXISTS sync_logs_pkey;
ALTER TABLE sync_logs                ADD PRIMARY KEY (id, created_at);
ALTER TABLE training_logs            DROP CONSTRAINT IF EXISTS training_logs_pkey;
ALTER TABLE training_logs            ADD PRIMARY KEY (id, created_at);
ALTER TABLE gold_prices              DROP CONSTRAINT IF EXISTS gold_prices_pkey;
ALTER TABLE gold_prices              ADD PRIMARY KEY (id, trading_date);
ALTER TABLE nasdaq_prices            DROP CONSTRAINT IF EXISTS nasdaq_prices_pkey;
ALTER TABLE nasdaq_prices            ADD PRIMARY KEY (id, trading_date);
ALTER TABLE sp500_prices             DROP CONSTRAINT IF EXISTS sp500_prices_pkey;
ALTER TABLE sp500_prices             ADD PRIMARY KEY (id, trading_date);
ALTER TABLE crypto_prices            DROP CONSTRAINT IF EXISTS crypto_prices_pkey;
ALTER TABLE crypto_prices            ADD PRIMARY KEY (id, trading_date);
ALTER TABLE gold_intraday_prices     DROP CONSTRAINT IF EXISTS gold_intraday_prices_pkey;
ALTER TABLE gold_intraday_prices     ADD PRIMARY KEY (id, timestamp);
ALTER TABLE nasdaq_intraday_prices   DROP CONSTRAINT IF EXISTS nasdaq_intraday_prices_pkey;
ALTER TABLE nasdaq_intraday_prices   ADD PRIMARY KEY (id, timestamp);
ALTER TABLE sp500_intraday_prices    DROP CONSTRAINT IF EXISTS sp500_intraday_prices_pkey;
ALTER TABLE sp500_intraday_prices    ADD PRIMARY KEY (id, timestamp);
ALTER TABLE crypto_intraday_prices   DROP CONSTRAINT IF EXISTS crypto_intraday_prices_pkey;
ALTER TABLE crypto_intraday_prices   ADD PRIMARY KEY (id, timestamp);
ALTER TABLE gold_predictions         DROP CONSTRAINT IF EXISTS gold_predictions_pkey;
ALTER TABLE gold_predictions         ADD PRIMARY KEY (id, prediction_date);
ALTER TABLE nasdaq_predictions       DROP CONSTRAINT IF EXISTS nasdaq_predictions_pkey;
ALTER TABLE nasdaq_predictions       ADD PRIMARY KEY (id, prediction_date);
ALTER TABLE sp500_predictions        DROP CONSTRAINT IF EXISTS sp500_predictions_pkey;
ALTER TABLE sp500_predictions        ADD PRIMARY KEY (id, prediction_date);
ALTER TABLE crypto_predictions       DROP CONSTRAINT IF EXISTS crypto_predictions_pkey;
ALTER TABLE crypto_predictions       ADD PRIMARY KEY (id, prediction_date);
ALTER TABLE sim_trades               DROP CONSTRAINT IF EXISTS sim_trades_pkey;
ALTER TABLE sim_trades               ADD PRIMARY KEY (id, trade_date);
ALTER TABLE sim_portfolio_snapshots  DROP CONSTRAINT IF EXISTS sim_portfolio_snapshots_pkey;
ALTER TABLE sim_portfolio_snapshots  ADD PRIMARY KEY (id, snapshot_date);

-- Daily price tables (1-month chunks)
SELECT create_hypertable('gold_prices',   'trading_date', chunk_time_interval => INTERVAL '1 month', if_not_exists => TRUE);
SELECT create_hypertable('nasdaq_prices', 'trading_date', chunk_time_interval => INTERVAL '1 month', if_not_exists => TRUE);
SELECT create_hypertable('sp500_prices',  'trading_date', chunk_time_interval => INTERVAL '1 month', if_not_exists => TRUE);
SELECT create_hypertable('crypto_prices', 'trading_date', chunk_time_interval => INTERVAL '1 month', if_not_exists => TRUE);

-- Intraday price tables (1-week chunks)
SELECT create_hypertable('gold_intraday_prices',   'timestamp', chunk_time_interval => INTERVAL '7 days', if_not_exists => TRUE);
SELECT create_hypertable('nasdaq_intraday_prices', 'timestamp', chunk_time_interval => INTERVAL '7 days', if_not_exists => TRUE);
SELECT create_hypertable('sp500_intraday_prices',  'timestamp', chunk_time_interval => INTERVAL '7 days', if_not_exists => TRUE);
SELECT create_hypertable('crypto_intraday_prices', 'timestamp', chunk_time_interval => INTERVAL '7 days', if_not_exists => TRUE);

-- Prediction tables (3-month chunks)
SELECT create_hypertable('gold_predictions',   'prediction_date', chunk_time_interval => INTERVAL '3 months', if_not_exists => TRUE);
SELECT create_hypertable('nasdaq_predictions', 'prediction_date', chunk_time_interval => INTERVAL '3 months', if_not_exists => TRUE);
SELECT create_hypertable('sp500_predictions',  'prediction_date', chunk_time_interval => INTERVAL '3 months', if_not_exists => TRUE);
SELECT create_hypertable('crypto_predictions', 'prediction_date', chunk_time_interval => INTERVAL '3 months', if_not_exists => TRUE);

-- Simulation time-series tables (3-month chunks)
SELECT create_hypertable('sim_trades',              'trade_date',    chunk_time_interval => INTERVAL '3 months', if_not_exists => TRUE);
SELECT create_hypertable('sim_portfolio_snapshots', 'snapshot_date', chunk_time_interval => INTERVAL '3 months', if_not_exists => TRUE);

-- Operational log tables (1-month chunks)
SELECT create_hypertable('sync_logs',     'created_at', chunk_time_interval => INTERVAL '1 month', if_not_exists => TRUE);
SELECT create_hypertable('training_logs', 'created_at', chunk_time_interval => INTERVAL '1 month', if_not_exists => TRUE);

-- ============================================================
-- TIMESCALEDB COMPRESSION POLICIES
-- Compress old chunks to save storage; retain query speed on
-- recent data. compress_segmentby improves per-entity scan speed.
-- ============================================================

-- Daily price tables: compress after 30 days
ALTER TABLE gold_prices   SET (
    timescaledb.compress,
    timescaledb.compress_orderby   = 'trading_date DESC',
    timescaledb.compress_segmentby = 'source'
);
SELECT add_compression_policy('gold_prices', INTERVAL '30 days', if_not_exists => TRUE);

ALTER TABLE nasdaq_prices SET (
    timescaledb.compress,
    timescaledb.compress_orderby   = 'trading_date DESC',
    timescaledb.compress_segmentby = 'symbol'
);
SELECT add_compression_policy('nasdaq_prices', INTERVAL '30 days', if_not_exists => TRUE);

ALTER TABLE sp500_prices  SET (
    timescaledb.compress,
    timescaledb.compress_orderby   = 'trading_date DESC',
    timescaledb.compress_segmentby = 'symbol'
);
SELECT add_compression_policy('sp500_prices', INTERVAL '30 days', if_not_exists => TRUE);

ALTER TABLE crypto_prices SET (
    timescaledb.compress,
    timescaledb.compress_orderby   = 'trading_date DESC',
    timescaledb.compress_segmentby = 'symbol'
);
SELECT add_compression_policy('crypto_prices', INTERVAL '30 days', if_not_exists => TRUE);

-- Intraday price tables: compress after 7 days
ALTER TABLE gold_intraday_prices SET (
    timescaledb.compress,
    timescaledb.compress_orderby   = 'timestamp DESC',
    timescaledb.compress_segmentby = 'source'
);
SELECT add_compression_policy('gold_intraday_prices', INTERVAL '7 days', if_not_exists => TRUE);

ALTER TABLE nasdaq_intraday_prices SET (
    timescaledb.compress,
    timescaledb.compress_orderby   = 'timestamp DESC',
    timescaledb.compress_segmentby = 'symbol'
);
SELECT add_compression_policy('nasdaq_intraday_prices', INTERVAL '7 days', if_not_exists => TRUE);

ALTER TABLE sp500_intraday_prices SET (
    timescaledb.compress,
    timescaledb.compress_orderby   = 'timestamp DESC',
    timescaledb.compress_segmentby = 'symbol'
);
SELECT add_compression_policy('sp500_intraday_prices', INTERVAL '7 days', if_not_exists => TRUE);

ALTER TABLE crypto_intraday_prices SET (
    timescaledb.compress,
    timescaledb.compress_orderby   = 'timestamp DESC',
    timescaledb.compress_segmentby = 'coin_id'
);
SELECT add_compression_policy('crypto_intraday_prices', INTERVAL '7 days', if_not_exists => TRUE);

-- Prediction tables: compress after 30 days
ALTER TABLE gold_predictions SET (
    timescaledb.compress,
    timescaledb.compress_orderby   = 'prediction_date DESC',
    timescaledb.compress_segmentby = 'algorithm_name'
);
SELECT add_compression_policy('gold_predictions', INTERVAL '30 days', if_not_exists => TRUE);

ALTER TABLE nasdaq_predictions SET (
    timescaledb.compress,
    timescaledb.compress_orderby   = 'prediction_date DESC',
    timescaledb.compress_segmentby = 'algorithm_name'
);
SELECT add_compression_policy('nasdaq_predictions', INTERVAL '30 days', if_not_exists => TRUE);

ALTER TABLE sp500_predictions SET (
    timescaledb.compress,
    timescaledb.compress_orderby   = 'prediction_date DESC',
    timescaledb.compress_segmentby = 'algorithm_name'
);
SELECT add_compression_policy('sp500_predictions', INTERVAL '30 days', if_not_exists => TRUE);

ALTER TABLE crypto_predictions SET (
    timescaledb.compress,
    timescaledb.compress_orderby   = 'prediction_date DESC',
    timescaledb.compress_segmentby = 'algorithm_name'
);
SELECT add_compression_policy('crypto_predictions', INTERVAL '30 days', if_not_exists => TRUE);

-- Simulation time-series: compress after 30 days
ALTER TABLE sim_trades SET (
    timescaledb.compress,
    timescaledb.compress_orderby   = 'trade_date DESC',
    timescaledb.compress_segmentby = 'bot_id'
);
SELECT add_compression_policy('sim_trades', INTERVAL '30 days', if_not_exists => TRUE);

ALTER TABLE sim_portfolio_snapshots SET (
    timescaledb.compress,
    timescaledb.compress_orderby   = 'snapshot_date DESC',
    timescaledb.compress_segmentby = 'bot_id'
);
SELECT add_compression_policy('sim_portfolio_snapshots', INTERVAL '30 days', if_not_exists => TRUE);

-- Operational log tables: compress after 30 days
ALTER TABLE sync_logs SET (
    timescaledb.compress,
    timescaledb.compress_orderby = 'created_at DESC'
);
SELECT add_compression_policy('sync_logs', INTERVAL '30 days', if_not_exists => TRUE);

ALTER TABLE training_logs SET (
    timescaledb.compress,
    timescaledb.compress_orderby = 'created_at DESC'
);
SELECT add_compression_policy('training_logs', INTERVAL '30 days', if_not_exists => TRUE);

-- ============================================================
-- CONTINUOUS AGGREGATES — Direction Accuracy per Market
-- Materialized views with daily refresh for fast dashboard queries.
-- ============================================================

-- Gold direction accuracy by algorithm (daily grain)
CREATE MATERIALIZED VIEW IF NOT EXISTS gold_direction_accuracy_daily
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 day', prediction_date) AS bucket,
    algorithm_name,
    COUNT(*) FILTER (WHERE direction_correct IS NOT NULL) AS total,
    COUNT(*) FILTER (WHERE direction_correct = TRUE)      AS correct
FROM gold_predictions
WHERE direction_correct IS NOT NULL
GROUP BY bucket, algorithm_name
WITH NO DATA;

SELECT add_continuous_aggregate_policy(
    'gold_direction_accuracy_daily',
    start_offset => INTERVAL '7 days',
    end_offset   => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

-- NASDAQ direction accuracy by algorithm (daily grain)
CREATE MATERIALIZED VIEW IF NOT EXISTS nasdaq_direction_accuracy_daily
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 day', prediction_date) AS bucket,
    algorithm_name,
    COUNT(*) FILTER (WHERE direction_correct IS NOT NULL) AS total,
    COUNT(*) FILTER (WHERE direction_correct = TRUE)      AS correct
FROM nasdaq_predictions
WHERE direction_correct IS NOT NULL
GROUP BY bucket, algorithm_name
WITH NO DATA;

SELECT add_continuous_aggregate_policy(
    'nasdaq_direction_accuracy_daily',
    start_offset => INTERVAL '7 days',
    end_offset   => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

-- S&P 500 direction accuracy by algorithm (daily grain)
CREATE MATERIALIZED VIEW IF NOT EXISTS sp500_direction_accuracy_daily
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 day', prediction_date) AS bucket,
    algorithm_name,
    COUNT(*) FILTER (WHERE direction_correct IS NOT NULL) AS total,
    COUNT(*) FILTER (WHERE direction_correct = TRUE)      AS correct
FROM sp500_predictions
WHERE direction_correct IS NOT NULL
GROUP BY bucket, algorithm_name
WITH NO DATA;

SELECT add_continuous_aggregate_policy(
    'sp500_direction_accuracy_daily',
    start_offset => INTERVAL '7 days',
    end_offset   => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

-- Crypto direction accuracy by algorithm (daily grain)
CREATE MATERIALIZED VIEW IF NOT EXISTS crypto_direction_accuracy_daily
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 day', prediction_date) AS bucket,
    algorithm_name,
    COUNT(*) FILTER (WHERE direction_correct IS NOT NULL) AS total,
    COUNT(*) FILTER (WHERE direction_correct = TRUE)      AS correct
FROM crypto_predictions
WHERE direction_correct IS NOT NULL
GROUP BY bucket, algorithm_name
WITH NO DATA;

SELECT add_continuous_aggregate_policy(
    'crypto_direction_accuracy_daily',
    start_offset => INTERVAL '7 days',
    end_offset   => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

-- ============================================================
-- MONITORING MATERIALIZED VIEW
-- Provides a fast, refreshable snapshot of crawl/prediction
-- freshness per market for the /api/monitoring/overview endpoint.
-- UNIQUE index on market allows CONCURRENTLY refresh.
-- ============================================================

CREATE MATERIALIZED VIEW IF NOT EXISTS monitoring_crawl_stats AS
SELECT
    market,
    MAX(last_daily_at)    AS last_daily_at,
    MAX(last_intraday_at) AS last_intraday_at,
    SUM(daily_today)      AS daily_today,
    SUM(intraday_today)   AS intraday_today
FROM (
    -- Gold daily
    SELECT
        'GOLD'             AS market,
        MAX(trading_date)  AS last_daily_at,
        NULL::TIMESTAMP    AS last_intraday_at,
        COUNT(*) FILTER (WHERE trading_date::date = (NOW() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date) AS daily_today,
        0                  AS intraday_today
    FROM gold_prices
    WHERE deleted_at IS NULL

    UNION ALL

    -- Gold intraday
    SELECT
        'GOLD'             AS market,
        NULL::TIMESTAMP    AS last_daily_at,
        MAX(timestamp)     AS last_intraday_at,
        0                  AS daily_today,
        COUNT(*) FILTER (WHERE timestamp::date = (NOW() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date) AS intraday_today
    FROM gold_intraday_prices

    UNION ALL

    -- NASDAQ daily
    SELECT
        'NASDAQ'           AS market,
        MAX(trading_date)  AS last_daily_at,
        NULL::TIMESTAMP    AS last_intraday_at,
        COUNT(*) FILTER (WHERE trading_date::date = (NOW() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date) AS daily_today,
        0                  AS intraday_today
    FROM nasdaq_prices
    WHERE deleted_at IS NULL

    UNION ALL

    -- NASDAQ intraday
    SELECT
        'NASDAQ'           AS market,
        NULL::TIMESTAMP    AS last_daily_at,
        MAX(timestamp)     AS last_intraday_at,
        0                  AS daily_today,
        COUNT(*) FILTER (WHERE timestamp::date = (NOW() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date) AS intraday_today
    FROM nasdaq_intraday_prices

    UNION ALL

    -- S&P 500 daily
    SELECT
        'SP500'            AS market,
        MAX(trading_date)  AS last_daily_at,
        NULL::TIMESTAMP    AS last_intraday_at,
        COUNT(*) FILTER (WHERE trading_date::date = (NOW() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date) AS daily_today,
        0                  AS intraday_today
    FROM sp500_prices
    WHERE deleted_at IS NULL

    UNION ALL

    -- S&P 500 intraday
    SELECT
        'SP500'            AS market,
        NULL::TIMESTAMP    AS last_daily_at,
        MAX(timestamp)     AS last_intraday_at,
        0                  AS daily_today,
        COUNT(*) FILTER (WHERE timestamp::date = (NOW() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date) AS intraday_today
    FROM sp500_intraday_prices

    UNION ALL

    -- Crypto daily
    SELECT
        'CRYPTO'           AS market,
        MAX(trading_date)  AS last_daily_at,
        NULL::TIMESTAMP    AS last_intraday_at,
        COUNT(*) FILTER (WHERE trading_date::date = (NOW() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date) AS daily_today,
        0                  AS intraday_today
    FROM crypto_prices
    WHERE deleted_at IS NULL

    UNION ALL

    -- Crypto intraday
    SELECT
        'CRYPTO'           AS market,
        NULL::TIMESTAMP    AS last_daily_at,
        MAX(timestamp)     AS last_intraday_at,
        0                  AS daily_today,
        COUNT(*) FILTER (WHERE timestamp::date = (NOW() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date) AS intraday_today
    FROM crypto_intraday_prices
) sub
GROUP BY market
WITH NO DATA;

-- UNIQUE index enables REFRESH MATERIALIZED VIEW CONCURRENTLY
CREATE UNIQUE INDEX IF NOT EXISTS idx_monitoring_crawl_stats_market
    ON monitoring_crawl_stats(market);

-- ============================================================
-- Pipeline Reports — stores per-run metadata written by the Python prediction service
-- ============================================================
CREATE TABLE IF NOT EXISTS pipeline_reports (
    id                BIGSERIAL       PRIMARY KEY,
    pipeline_key      VARCHAR(50)     NOT NULL,
    market            VARCHAR(20)     NOT NULL,
    status            VARCHAR(20)     NOT NULL,
    started_at        TIMESTAMP       NOT NULL,
    finished_at       TIMESTAMP       NULL,
    duration_ms       BIGINT          NOT NULL DEFAULT 0,
    crawled_count     INTEGER         NOT NULL DEFAULT 0,
    predictions_count INTEGER         NOT NULL DEFAULT 0,
    trained           BOOLEAN         NOT NULL DEFAULT FALSE,
    steps             JSONB,
    error             TEXT,
    created_at        TIMESTAMP       NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pipeline_reports_key
    ON pipeline_reports(pipeline_key, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_pipeline_reports_created
    ON pipeline_reports(created_at);

-- ============================================================
-- Pipeline crawl counters — persist per-market crawl count across restarts.
-- Source of truth for "train every 10th crawl" logic in the prediction-svc
-- pipeline (replaces the old in-memory _crawl_counts dict so the count
-- survives prediction-svc restarts). Not a hypertable.
-- ============================================================
CREATE TABLE IF NOT EXISTS pipeline_crawl_counters (
    market_key   VARCHAR(32)  PRIMARY KEY,
    crawl_count  BIGINT       NOT NULL DEFAULT 0,
    updated_at   TIMESTAMP    NOT NULL DEFAULT NOW()
);

-- ============================================================
-- Service Management — registry / discovery (service-mgt)
-- Source of truth for service registrations; service-mgt mirrors it into an
-- in-memory cache (write-through). Not a hypertable.
-- ============================================================
CREATE TABLE IF NOT EXISTS service_instances (
    id            BIGSERIAL     PRIMARY KEY,
    service_name  VARCHAR(64)   NOT NULL,
    instance_id   VARCHAR(128)  NOT NULL UNIQUE,
    address       VARCHAR(255)  NOT NULL,
    port          INTEGER       NOT NULL,
    metadata      JSONB,
    status        VARCHAR(16)   NOT NULL DEFAULT 'UP',
    ttl_seconds   INTEGER       NOT NULL DEFAULT 30,
    last_seen     TIMESTAMP     NOT NULL DEFAULT NOW(),
    registered_at TIMESTAMP     NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP     NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_service_instances_name
    ON service_instances(service_name, status);

CREATE INDEX IF NOT EXISTS idx_service_instances_status
    ON service_instances(status);

-- ============================================================
-- HOURLY PREDICTION / BOT TRADING SCHEMA (additive, idempotent)
-- Adds snapshot_at / trade_at columns for sub-daily granularity.
-- All statements use ADD COLUMN IF NOT EXISTS / CREATE INDEX IF NOT EXISTS
-- so they are safe to run against existing production databases.
--
-- CRITICAL (TimescaleDB): CREATE UNIQUE INDEX on a hypertable MUST be a
-- separate statement from any DML (DELETE/UPDATE). Mixing them in the same
-- transaction/block causes Timescale to roll back the whole block.
-- ============================================================

-- sim_portfolio_snapshots: add snapshot_at for hourly upsert key
ALTER TABLE sim_portfolio_snapshots
    ADD COLUMN IF NOT EXISTS snapshot_at TIMESTAMP;

-- Backfill: existing rows without snapshot_at get snapshot_date as fallback
UPDATE sim_portfolio_snapshots
    SET snapshot_at = snapshot_date
    WHERE snapshot_at IS NULL;

-- sim_trades: add trade_at for hourly granularity
ALTER TABLE sim_trades
    ADD COLUMN IF NOT EXISTS trade_at TIMESTAMP;

-- Backfill: existing trades without trade_at get trade_date as fallback
UPDATE sim_trades
    SET trade_at = trade_date
    WHERE trade_at IS NULL;

-- Drop the old daily-only unique index: it enforces 1 row/(session,date) and
-- would block the multiple hourly snapshots/day that live-step now writes.
DROP INDEX IF EXISTS uq_sim_snap_session_date;

-- Unique index for hourly live-step upsert. TimescaleDB requires the partition
-- column (snapshot_date) to be part of any unique index on a hypertable, so the
-- key is (session_id, snapshot_at, snapshot_date). Since snapshot_at is derived
-- from snapshot_date this is effectively "1 row per session per snapshot_at"
-- (per-hour for live, per-day midnight for backtest).
-- MUST be a separate statement (not inside a DML transaction block) on
-- TimescaleDB hypertables to avoid Timescale rollback.
CREATE UNIQUE INDEX IF NOT EXISTS uq_sim_snap_session_at
    ON sim_portfolio_snapshots (session_id, snapshot_at, snapshot_date);

-- ============================================================
-- CANDLESTICK CHART OHLC COLUMNS (additive, idempotent)
-- Adds open_price / high_price / low_price to CRYPTO and GOLD tables.
-- NASDAQ and SP500 already have OHLC columns — no changes needed.
-- All statements use ADD COLUMN IF NOT EXISTS so they are safe to run
-- against existing production databases.
-- ============================================================

-- crypto_prices: daily OHLC (close_price already exists)
ALTER TABLE crypto_prices          ADD COLUMN IF NOT EXISTS open_price NUMERIC(20,2);
ALTER TABLE crypto_prices          ADD COLUMN IF NOT EXISTS high_price NUMERIC(20,2);
ALTER TABLE crypto_prices          ADD COLUMN IF NOT EXISTS low_price  NUMERIC(20,2);

-- crypto_intraday_prices: hourly OHLC (price/close already exists)
ALTER TABLE crypto_intraday_prices ADD COLUMN IF NOT EXISTS open_price NUMERIC(30,8);
ALTER TABLE crypto_intraday_prices ADD COLUMN IF NOT EXISTS high_price NUMERIC(30,8);
ALTER TABLE crypto_intraday_prices ADD COLUMN IF NOT EXISTS low_price  NUMERIC(30,8);

-- gold_prices: daily OHLC for XAU source (buy_price/sell_price kept for VN sources)
ALTER TABLE gold_prices            ADD COLUMN IF NOT EXISTS open_price NUMERIC(15,2);
ALTER TABLE gold_prices            ADD COLUMN IF NOT EXISTS high_price NUMERIC(15,2);
ALTER TABLE gold_prices            ADD COLUMN IF NOT EXISTS low_price  NUMERIC(15,2);

-- gold_intraday_prices: hourly OHLC for XAU source
ALTER TABLE gold_intraday_prices   ADD COLUMN IF NOT EXISTS open_price NUMERIC(15,2);
ALTER TABLE gold_intraday_prices   ADD COLUMN IF NOT EXISTS high_price NUMERIC(15,2);
ALTER TABLE gold_intraday_prices   ADD COLUMN IF NOT EXISTS low_price  NUMERIC(15,2);

-- ============================================================
-- END OF SCHEMA
-- ============================================================

-- ============================================================
-- STOCK FUNDAMENTALS (báo cáo tài chính — snapshot mới nhất per symbol)
-- Nguồn: yfinance (Yahoo Finance) — chỉ NASDAQ / SP500 có dữ liệu.
-- 1 row / (market_key, symbol); crawler upsert ON CONFLICT.
-- Dùng bởi thuật toán transformer_nn làm static context features.
-- ============================================================

CREATE TABLE IF NOT EXISTS stock_fundamentals (
    id BIGSERIAL PRIMARY KEY,
    market_key VARCHAR(16) NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    pe_ratio NUMERIC(20,4),
    forward_pe NUMERIC(20,4),
    price_to_book NUMERIC(20,4),
    eps_ttm NUMERIC(20,4),
    revenue_growth NUMERIC(12,6),
    earnings_growth NUMERIC(12,6),
    profit_margin NUMERIC(12,6),
    debt_to_equity NUMERIC(20,4),
    dividend_yield NUMERIC(12,6),
    beta NUMERIC(12,6),
    market_cap NUMERIC(30,2),
    fetched_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_fundamentals_market_symbol UNIQUE (market_key, symbol)
);
CREATE INDEX IF NOT EXISTS idx_fundamentals_market ON stock_fundamentals (market_key);

-- ============================================================
-- STOCK SPLITS — authoritative ex-date register (NASDAQ / SP500 only)
-- Source: Yahoo Finance chart API events.splits (returned per daily fetch).
-- Crawler records new splits on detection; apply_pending_splits adjusts
-- historical price rows once (idempotent via applied_at IS NOT NULL guard).
-- Not a hypertable.
-- ============================================================
CREATE TABLE IF NOT EXISTS stock_splits (
    id            BIGSERIAL     PRIMARY KEY,
    market_key    VARCHAR(20)   NOT NULL,           -- 'NASDAQ' | 'SP500'
    symbol        VARCHAR(20)   NOT NULL,
    split_date    DATE          NOT NULL,            -- ex-date (ICT wallclock)
    ratio         NUMERIC(12,6) NOT NULL,            -- numerator/denominator  e.g. 4.0 for 4:1
    numerator     NUMERIC(12,6) NOT NULL,
    denominator   NUMERIC(12,6) NOT NULL,
    applied_at    TIMESTAMP,                         -- NULL = history not yet adjusted
    created_at    TIMESTAMP     NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_stock_splits_sym_date
    ON stock_splits (market_key, symbol, split_date);
