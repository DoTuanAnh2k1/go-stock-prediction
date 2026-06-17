# Database Optimization Plan: MySQL → PostgreSQL + TimescaleDB

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate from MySQL 8.0 to PostgreSQL 16 + TimescaleDB, add materialized views and pre-computed KPIs, so the system handles unbounded data growth without ever deleting historical records.

**Architecture:** PostgreSQL 16 + TimescaleDB extension as the sole datastore; time-series tables (prices, predictions, trades, snapshots) become hypertables with automatic chunk compression at 30 days; heavy aggregates (leaderboard, direction accuracy, monitoring) are pre-computed in materialized views and KPI columns — no more N+1 or full-scan queries at read time.

**Tech Stack:** PostgreSQL 16, TimescaleDB 2.x, GORM postgres driver, SQLAlchemy psycopg2, Spring Boot 3 + PostgreSQL JDBC + Flyway, Redis 7 (Phase 5 only).

## Global Constraints

- **Data retention:** Data is NEVER deleted. All historical records stay queryable forever. TimescaleDB compression handles storage growth.
- **ICT-at-rest timezone:** All `TIMESTAMP` columns store wall-clock `Asia/Ho_Chi_Minh` time. No UTC. Go DSN must include `TimeZone=Asia/Ho_Chi_Minh`. Python uses `datetime.now()` (container TZ=ICT), never `utcnow()`.
- **4 services must all connect to same DB:** Go API (GORM), Python Prediction (SQLAlchemy+psycopg2), Java Auth (Spring Boot JDBC+Flyway), phpMyAdmin replaced by pgAdmin.
- **Zero downtime goal:** Each phase is independently deployable; roll back = point docker-compose back to mysql image + database.sql.

---

## File Structure

```
docker-compose.yml                              # MODIFY: mysql → timescaledb, add pgadmin
database.sql                                    # REWRITE: full PostgreSQL schema + hypertables
database_migrate.sh                             # CREATE: mysqldump → pgloader/pg_restore helper
api/pkg/store/mysql/mysql.go                    # MODIFY: gorm driver mysql → postgres
api/pkg/models/models_config/config.go          # MODIFY: rename MySqlConfig → PostgresConfig fields
api/pkg/models/models_db/simulation.go          # MODIFY: add KPI columns to SimSession struct
api/go.mod                                      # MODIFY: add gorm postgres driver, remove mysql driver
auth-service/pom.xml                            # MODIFY: mysql-connector-j → postgresql, flyway-mysql → flyway-database-postgresql
auth-service/src/main/resources/application.yml # MODIFY: JDBC URL + driver + dialect
auth-service/src/main/resources/db/migration/V1__create_auth_tables.sql  # REWRITE: PostgreSQL syntax
auth-service/src/main/resources/db/migration/V2__add_user_profile_fields.sql  # REWRITE: PostgreSQL syntax
prediction/pyproject.toml                       # MODIFY: pymysql → psycopg2-binary
prediction/src/config.py                        # MODIFY: db_driver field, remove charset
prediction/src/database/connection.py           # MODIFY: connection string + engine args
prediction/src/orchestrator/training.py         # MODIFY: call update_session_kpis() after session end
prediction/src/database/repository.py          # MODIFY: add update_session_kpis(), refresh_views()
```

---

## Why PostgreSQL Over MySQL (Background)

| Feature | MySQL 8.0 | PostgreSQL 16 + TimescaleDB |
|---|---|---|
| Time-series partitioning | Manual `PARTITION BY RANGE` | Hypertables — automatic chunks, transparent queries |
| Chunk compression | None | 90–95% compression on cold chunks (30d+) |
| Continuous aggregates | None | `timescaledb.continuous` MV, auto-refreshed by background job |
| Materialized views | No `REFRESH CONCURRENTLY` | Full `REFRESH MATERIALIZED VIEW CONCURRENTLY` (non-blocking reads during refresh) |
| Partial indexes | No | `CREATE INDEX ... WHERE condition` — index only rows that matter |
| HikariCP reconnect | Problematic after restart | Standard PostgreSQL JDBC handles reconnect cleanly |
| Analytics query planner | Limited | Parallel query execution across hypertable chunks |
| Full-text, JSONB, arrays | Limited | Native, indexed |

---

## Phase 1: PostgreSQL + TimescaleDB Foundation

### Task 1: Replace MySQL with TimescaleDB in docker-compose

**Files:**
- Modify: `docker-compose.yml`

**Steps:**

- [ ] **Step 1: Back up current docker-compose.yml**

```bash
cp docker-compose.yml docker-compose.yml.mysql-backup
```

- [ ] **Step 2: Replace the `db` service block**

Find the `db:` service in `docker-compose.yml` and replace it with:

```yaml
  db:
    image: timescale/timescaledb:latest-pg16
    container_name: timescaledb
    environment:
      POSTGRES_USER: ${POSTGRES_USER:-postgres}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-123}
      POSTGRES_DB: ${POSTGRES_DB:-go_stock_prediction}
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./database.sql:/docker-entrypoint-initdb.d/01-schema.sql
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres -d go_stock_prediction"]
      interval: 10s
      timeout: 5s
      retries: 5
    restart: unless-stopped
```

- [ ] **Step 3: Replace `phpmyadmin` with `pgadmin` in docker-compose.yml**

```yaml
  pgadmin:
    image: dpage/pgadmin4:latest
    container_name: pgadmin
    environment:
      PGADMIN_DEFAULT_EMAIL: admin@local.dev
      PGADMIN_DEFAULT_PASSWORD: admin
    ports:
      - "127.0.0.1:8081:80"
    depends_on:
      - db
    restart: unless-stopped
```

- [ ] **Step 4: Replace the mysql `volumes:` entry**

```yaml
volumes:
  postgres_data:
  backup_data:
```

- [ ] **Step 5: Update `.env` with PostgreSQL variables**

```bash
# Old MySQL vars stay for reference; add PostgreSQL vars
POSTGRES_USER=postgres
POSTGRES_PASSWORD=123
POSTGRES_DB=go_stock_prediction
POSTGRES_HOST=db
POSTGRES_PORT=5432
```

- [ ] **Step 6: Update all service `depends_on` that reference `db`**

All services that had `condition: service_healthy` on `db` stay the same — the healthcheck name is identical, just the image changed.

- [ ] **Step 7: Verify container starts**

```bash
docker-compose up db -d
docker exec timescaledb pg_isready -U postgres -d go_stock_prediction
```

Expected: `timescaledb:5432 - accepting connections`

---

### Task 2: Rewrite database.sql for PostgreSQL + TimescaleDB

**Files:**
- Rewrite: `database.sql`

This is the biggest single change. PostgreSQL syntax differences from MySQL:
- `AUTO_INCREMENT` → `BIGSERIAL` (or `SERIAL`)
- `TINYINT(1)` → `BOOLEAN`
- `DATETIME` → `TIMESTAMP` (without timezone, ICT-at-rest)
- `ENGINE=InnoDB DEFAULT CHARSET=utf8mb4` → remove entirely
- `BIGINT UNSIGNED` → `BIGINT` (PostgreSQL has no UNSIGNED; add CHECK constraint if needed)
- Backtick identifiers `` `col` `` → none needed (lowercase identifiers are fine)
- `INDEX idx_name (col)` inside CREATE TABLE → separate `CREATE INDEX` statements after table creation
- `ON UPDATE CURRENT_TIMESTAMP` → use a trigger or remove (PostgreSQL doesn't support this column option)
- `INSERT INTO ... ON DUPLICATE KEY UPDATE` → `INSERT INTO ... ON CONFLICT (col) DO UPDATE SET ...`

**Steps:**

- [ ] **Step 1: Enable TimescaleDB extension at top of database.sql**

```sql
-- Enable TimescaleDB
CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;
```

- [ ] **Step 2: Convert sync_logs table**

```sql
CREATE TABLE IF NOT EXISTS sync_logs (
    id          BIGSERIAL PRIMARY KEY,
    source      VARCHAR(50)  NOT NULL,
    status      VARCHAR(20)  NOT NULL,
    message     TEXT,
    records     INTEGER      DEFAULT 0,
    created_at  TIMESTAMP    NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_sync_logs_source ON sync_logs(source);
CREATE INDEX IF NOT EXISTS idx_sync_logs_created_at ON sync_logs(created_at DESC);
```

- [ ] **Step 3: Convert gold_prices table (representative — repeat for nasdaq_prices, sp500_prices, crypto_prices)**

```sql
CREATE TABLE IF NOT EXISTS gold_prices (
    id           BIGSERIAL PRIMARY KEY,
    trading_date TIMESTAMP   NOT NULL,
    open_price   NUMERIC(20,4),
    high_price   NUMERIC(20,4),
    low_price    NUMERIC(20,4),
    close_price  NUMERIC(20,4),
    volume       BIGINT,
    source       VARCHAR(50),
    created_at   TIMESTAMP   NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_gold_prices_trading_date ON gold_prices(trading_date DESC);
CREATE INDEX IF NOT EXISTS idx_gold_prices_created_at   ON gold_prices(created_at DESC);
```

Repeat for: `nasdaq_prices`, `sp500_prices`, `crypto_prices` (add `symbol VARCHAR(20)` for multi-symbol tables).

- [ ] **Step 4: Convert intraday price tables (representative)**

```sql
CREATE TABLE IF NOT EXISTS gold_intraday_prices (
    id           BIGSERIAL PRIMARY KEY,
    symbol       VARCHAR(20) NOT NULL,
    timestamp    TIMESTAMP   NOT NULL,
    open_price   NUMERIC(20,4),
    high_price   NUMERIC(20,4),
    low_price    NUMERIC(20,4),
    close_price  NUMERIC(20,4),
    volume       BIGINT,
    created_at   TIMESTAMP   NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_gold_intraday_timestamp ON gold_intraday_prices(timestamp DESC);
```

Repeat for: `nasdaq_intraday_prices`, `sp500_intraday_prices`, `crypto_intraday_prices`.

- [ ] **Step 5: Convert prediction tables (representative)**

```sql
CREATE TABLE IF NOT EXISTS gold_predictions (
    id                BIGSERIAL   PRIMARY KEY,
    prediction_date   TIMESTAMP   NOT NULL,
    target_date       TIMESTAMP   NOT NULL,
    algorithm_name    VARCHAR(50) NOT NULL,
    predicted_price   NUMERIC(20,4),
    current_price     NUMERIC(20,4),
    confidence        NUMERIC(5,4),
    direction_correct BOOLEAN,
    created_at        TIMESTAMP   NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_gold_pred_date      ON gold_predictions(prediction_date DESC);
CREATE INDEX IF NOT EXISTS idx_gold_pred_algo      ON gold_predictions(algorithm_name, prediction_date DESC);
CREATE INDEX IF NOT EXISTS idx_gold_pred_direction ON gold_predictions(algorithm_name, created_at)
    WHERE direction_correct IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_gold_pred_created   ON gold_predictions(created_at DESC);
```

Note the **partial index** `WHERE direction_correct IS NOT NULL` — this is a PostgreSQL feature that massively speeds up direction-accuracy queries. Repeat for: `nasdaq_predictions`, `sp500_predictions`, `crypto_predictions`.

- [ ] **Step 6: Convert training_logs and training_metrics tables**

```sql
CREATE TABLE IF NOT EXISTS training_logs (
    id             BIGSERIAL   PRIMARY KEY,
    session_id     VARCHAR(50) NOT NULL,
    algorithm_name VARCHAR(50) NOT NULL,
    market_key     VARCHAR(20),
    status         VARCHAR(20) NOT NULL DEFAULT 'running',
    started_at     TIMESTAMP   NOT NULL DEFAULT NOW(),
    completed_at   TIMESTAMP,
    duration_secs  NUMERIC(10,2),
    error_message  TEXT,
    created_at     TIMESTAMP   NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_training_logs_algo       ON training_logs(algorithm_name, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_training_logs_market     ON training_logs(market_key, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_training_logs_created_at ON training_logs(created_at DESC);

CREATE TABLE IF NOT EXISTS training_metrics (
    id             BIGSERIAL   PRIMARY KEY,
    training_log_id BIGINT     REFERENCES training_logs(id) ON DELETE CASCADE,
    metric_name    VARCHAR(50) NOT NULL,
    metric_value   NUMERIC(20,8),
    created_at     TIMESTAMP   NOT NULL DEFAULT NOW()
);
```

- [ ] **Step 7: Convert users table**

```sql
CREATE TABLE IF NOT EXISTS users (
    id            BIGSERIAL   PRIMARY KEY,
    username      VARCHAR(50) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    role          VARCHAR(20) NOT NULL DEFAULT 'user',
    full_name     VARCHAR(100),
    email         VARCHAR(255),
    phone         VARCHAR(30),
    created_at    TIMESTAMP   NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP   NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
```

- [ ] **Step 8: Convert cron_schedules table**

```sql
CREATE TABLE IF NOT EXISTS cron_schedules (
    id              BIGSERIAL   PRIMARY KEY,
    job_key         VARCHAR(50) NOT NULL UNIQUE,
    job_name        VARCHAR(100) NOT NULL,
    cron_expression VARCHAR(100) NOT NULL,
    enabled         BOOLEAN     NOT NULL DEFAULT TRUE,
    updated_at      TIMESTAMP   NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_cron_schedules_job_key ON cron_schedules(job_key);
```

- [ ] **Step 9: Convert simulation tables**

```sql
CREATE TABLE IF NOT EXISTS sim_bots (
    id               VARCHAR(50)  PRIMARY KEY,
    market           VARCHAR(20)  NOT NULL,
    algorithm        VARCHAR(50)  NOT NULL,
    display_name     VARCHAR(100) NOT NULL,
    initial_capital  NUMERIC(20,2) NOT NULL,
    currency         VARCHAR(5)   NOT NULL,
    buy_threshold    NUMERIC(5,2)  DEFAULT 1.50,
    sell_threshold   NUMERIC(5,2)  DEFAULT 1.00,
    min_confidence   NUMERIC(4,2)  DEFAULT 0.60,
    stop_loss        NUMERIC(5,2)  DEFAULT 5.00,
    take_profit      NUMERIC(5,2)  DEFAULT 8.00,
    max_position_pct NUMERIC(5,2)  DEFAULT 15.00,
    max_positions    INTEGER       DEFAULT 5,
    is_active        BOOLEAN       DEFAULT TRUE,
    created_at       TIMESTAMP     NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMP     NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_sim_bots_market ON sim_bots(market);

CREATE TABLE IF NOT EXISTS sim_sessions (
    id           BIGSERIAL   PRIMARY KEY,
    bot_id       VARCHAR(50) NOT NULL REFERENCES sim_bots(id),
    start_date   TIMESTAMP   NOT NULL,
    end_date     TIMESTAMP,
    status       VARCHAR(20) NOT NULL DEFAULT 'running',
    mode         VARCHAR(20) NOT NULL DEFAULT 'backtest',
    -- Pre-computed KPIs (populated by Python after session end):
    total_trades INTEGER     DEFAULT 0,
    wins         INTEGER     DEFAULT 0,
    losses       INTEGER     DEFAULT 0,
    breakeven    INTEGER     DEFAULT 0,
    total_pnl    NUMERIC(20,2) DEFAULT 0,
    total_return_pct NUMERIC(8,4),
    win_rate     NUMERIC(5,4),
    profit_factor NUMERIC(8,4),
    max_drawdown_pct NUMERIC(8,4),
    kpi_updated_at TIMESTAMP,
    created_at   TIMESTAMP   NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_sim_sessions_bot_id ON sim_sessions(bot_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_sim_sessions_status ON sim_sessions(status, mode);

CREATE TABLE IF NOT EXISTS sim_trades (
    id             BIGSERIAL    PRIMARY KEY,
    session_id     BIGINT       NOT NULL REFERENCES sim_sessions(id),
    bot_id         VARCHAR(50)  NOT NULL,
    symbol         VARCHAR(20)  NOT NULL,
    action         VARCHAR(5)   NOT NULL,
    quantity       NUMERIC(20,6) NOT NULL,
    price          NUMERIC(20,4) NOT NULL,
    trade_value    NUMERIC(20,2) NOT NULL,
    signal_strength NUMERIC(8,4),
    confidence     NUMERIC(4,3),
    trade_date     TIMESTAMP    NOT NULL,
    close_reason   VARCHAR(20),
    entry_trade_id BIGINT,
    pnl            NUMERIC(20,2),
    pnl_pct        NUMERIC(8,4),
    created_at     TIMESTAMP    NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_sim_trades_session   ON sim_trades(session_id, trade_date);
CREATE INDEX IF NOT EXISTS idx_sim_trades_bot       ON sim_trades(bot_id, trade_date DESC);
CREATE INDEX IF NOT EXISTS idx_sim_trades_created   ON sim_trades(created_at DESC);

CREATE TABLE IF NOT EXISTS sim_portfolio_snapshots (
    id              BIGSERIAL    PRIMARY KEY,
    session_id      BIGINT       NOT NULL REFERENCES sim_sessions(id),
    bot_id          VARCHAR(50)  NOT NULL,
    snapshot_date   TIMESTAMP    NOT NULL,
    cash_balance    NUMERIC(20,2) NOT NULL,
    positions_value NUMERIC(20,2) NOT NULL,
    total_value     NUMERIC(20,2) NOT NULL,
    total_return_pct NUMERIC(8,4),
    open_positions  INTEGER      DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_sim_snap_session_date ON sim_portfolio_snapshots(session_id, snapshot_date DESC);
CREATE INDEX IF NOT EXISTS idx_sim_snap_bot_date     ON sim_portfolio_snapshots(bot_id, snapshot_date DESC);
```

- [ ] **Step 10: Verify schema loads cleanly**

```bash
docker-compose up db -d
docker exec -i timescaledb psql -U postgres -d go_stock_prediction < database.sql
docker exec timescaledb psql -U postgres -d go_stock_prediction -c "\dt"
```

Expected: all tables listed with no errors.

- [ ] **Step 11: Commit**

```bash
git add database.sql docker-compose.yml .env
git commit -m "chore(db): replace MySQL with TimescaleDB PostgreSQL, rewrite schema"
```

---

### Task 3: Go API — Switch GORM Driver to PostgreSQL

**Files:**
- Modify: `api/go.mod`
- Modify: `api/pkg/store/mysql/mysql.go`
- Modify: `api/pkg/models/models_config/config.go`

**Steps:**

- [ ] **Step 1: Swap GORM driver in go.mod**

```bash
cd api
go get gorm.io/driver/postgres
go get github.com/lib/pq
```

Then in `api/go.mod`, confirm `gorm.io/driver/postgres` appears. You can optionally remove `gorm.io/driver/mysql` if no other file imports it.

- [ ] **Step 2: Update config struct in `api/pkg/models/models_config/config.go`**

Find the `MySqlConfig` struct and rename/update fields:

```go
// Before:
type MySqlConfig struct {
    Host     string
    Port     string
    User     string
    Password string
    Name     string
    Debug    bool
}

// After (rename to PostgresConfig, keep backward-compatible env var names):
type PostgresConfig struct {
    Host     string
    Port     string
    User     string
    Password string
    Name     string
    Debug    bool
}
```

Update `DatabaseConfig` if it embeds `MySqlConfig`:

```go
type DatabaseConfig struct {
    Postgres PostgresConfig  // renamed from Mysql
}
```

Update `config.go` loading logic to read `POSTGRES_*` env vars (or keep `MYSQL_*` if you prefer not to rename env vars — both work, just be consistent).

- [ ] **Step 3: Rewrite `api/pkg/store/mysql/mysql.go`**

The file can stay at `api/pkg/store/mysql/mysql.go` path-wise (renaming the directory is optional), but change the driver:

```go
package mysql

import (
    "fmt"
    "go-stock-prediction/pkg/logger"
    "go-stock-prediction/pkg/models/models_config"
    modelsdb "go-stock-prediction/pkg/models/models_db"

    "gorm.io/driver/postgres"
    "gorm.io/gorm"
)

func (c *Client) Init(cfg models_config.DatabaseConfig) error {
    pg := cfg.Postgres  // was cfg.Mysql
    dsn := fmt.Sprintf(
        "host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Ho_Chi_Minh",
        pg.Host, pg.User, pg.Password, pg.Name, pg.Port,
    )
    gormLogger := logger.NewGormLogger(pg.Debug)
    db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
        Logger: gormLogger,
    })
    if err != nil {
        logger.Logger.Debugf("Error connecting to database: error=%v", err)
        return err
    }
    c.Db = db
    c.cfg = pg

    if err := db.AutoMigrate(modelsdb.AllModels...); err != nil {
        logger.Logger.Errorf("AutoMigrate failed: %v", err)
        return err
    }
    logger.Logger.Info("AutoMigrate completed successfully")
    return nil
}
```

Note: `AutoMigrate` with GORM + PostgreSQL is safe — GORM translates `decimal(20,4)` → `NUMERIC(20,4)`, `bool` → `BOOLEAN`, `time.Time` → `TIMESTAMP` automatically.

- [ ] **Step 4: Build and check for compile errors**

```bash
cd api && go build ./...
```

Expected: no errors. If you see `undefined: MySqlConfig`, grep for remaining references:

```bash
grep -r "MySqlConfig\|mysql\.Open\|driver/mysql" api/pkg/
```

Fix each remaining reference.

- [ ] **Step 5: Verify connection against running TimescaleDB**

```bash
docker-compose up db api -d
docker logs api_service 2>&1 | grep -E "AutoMigrate|Connect|Error"
```

Expected: `AutoMigrate completed successfully`

- [ ] **Step 6: Commit**

```bash
git add api/go.mod api/go.sum api/pkg/store/mysql/mysql.go api/pkg/models/models_config/
git commit -m "feat(api): switch GORM driver from MySQL to PostgreSQL"
```

---

### Task 4: Python Prediction Service — Switch SQLAlchemy to psycopg2

**Files:**
- Modify: `prediction/pyproject.toml`
- Modify: `prediction/src/config.py`
- Modify: `prediction/src/database/connection.py`

**Steps:**

- [ ] **Step 1: Update `prediction/pyproject.toml`**

Remove `pymysql` (or `mysqlclient`), add `psycopg2-binary`:

```toml
[project]
dependencies = [
    # ... other deps ...
    "psycopg2-binary>=2.9",
    # remove: "PyMySQL>=1.0" or "mysqlclient"
]
```

- [ ] **Step 2: Update `prediction/src/config.py`**

```python
class Settings(BaseSettings):
    # gRPC
    grpc_server_port: int = 8119

    # Database — PostgreSQL
    postgres_host: str = "localhost"
    postgres_port: int = 5432
    postgres_user: str = "postgres"
    postgres_password: str = "123"
    postgres_db: str = "go_stock_prediction"
    postgres_debug: bool = False

    # Keep old MYSQL_ vars as fallback aliases for docker-compose transition
    mysql_host: str = "localhost"   # deprecated
    mysql_port: int = 3306          # deprecated

    class Config:
        env_file = ".env"
        case_sensitive = False
        extra = "ignore"
```

- [ ] **Step 3: Update `prediction/src/database/connection.py`**

```python
def init_db() -> None:
    global _engine, _SessionLocal

    cfg = get_settings()

    connection_string = (
        f"postgresql+psycopg2://{cfg.postgres_user}:{cfg.postgres_password}"
        f"@{cfg.postgres_host}:{cfg.postgres_port}/{cfg.postgres_db}"
    )

    _engine = create_engine(
        connection_string,
        echo=cfg.postgres_debug,
        pool_pre_ping=True,
        pool_size=10,
        max_overflow=20,
        pool_recycle=3600,
        connect_args={"options": "-c TimeZone=Asia/Ho_Chi_Minh"},
    )

    _SessionLocal = sessionmaker(bind=_engine, autocommit=False, autoflush=False)
    log.info("database.connected", host=cfg.postgres_host, db=cfg.postgres_db)
```

The `connect_args={"options": "-c TimeZone=..."}` sets the PostgreSQL session timezone — equivalent to MySQL's `loc=Asia%2FHo_Chi_Minh` in DSN.

- [ ] **Step 4: Check ORM models for MySQL-specific types**

```bash
grep -n "mysql\|TINYINT\|AUTO_INCREMENT\|charset" prediction/src/database/models.py
```

SQLAlchemy ORM models typically use database-agnostic types (`DateTime`, `Numeric`, `Boolean`, `String`) — these work unchanged with PostgreSQL. Fix any MySQL dialect imports if found.

- [ ] **Step 5: Verify in Docker**

```bash
docker-compose build prediction
docker-compose up prediction -d
docker logs prediction_service 2>&1 | grep -E "connected|Error|Traceback"
```

Expected: `database.connected host=db db=go_stock_prediction`

- [ ] **Step 6: Run unit tests**

```bash
docker exec prediction_service python -m pytest tests/unit/ -v
```

Expected: all passing (unit tests use mock data, not real DB — they verify algorithm math, not connectivity).

- [ ] **Step 7: Commit**

```bash
git add prediction/pyproject.toml prediction/src/config.py prediction/src/database/connection.py
git commit -m "feat(prediction): switch SQLAlchemy from MySQL/pymysql to PostgreSQL/psycopg2"
```

---

### Task 5: Java Auth Service — Switch Spring Boot to PostgreSQL

**Files:**
- Modify: `auth-service/pom.xml`
- Modify: `auth-service/src/main/resources/application.yml`
- Rewrite: `auth-service/src/main/resources/db/migration/V1__create_auth_tables.sql`
- Rewrite: `auth-service/src/main/resources/db/migration/V2__add_user_profile_fields.sql`

**Steps:**

- [ ] **Step 1: Update `auth-service/pom.xml`**

Find and replace MySQL dependencies:

```xml
<!-- REMOVE these: -->
<dependency>
    <groupId>com.mysql</groupId>
    <artifactId>mysql-connector-j</artifactId>
    <scope>runtime</scope>
</dependency>
<!-- Flyway MySQL module -->
<dependency>
    <groupId>org.flywaydb</groupId>
    <artifactId>flyway-mysql</artifactId>
</dependency>

<!-- ADD these: -->
<dependency>
    <groupId>org.postgresql</groupId>
    <artifactId>postgresql</artifactId>
    <scope>runtime</scope>
</dependency>
<!-- No extra Flyway module needed for PostgreSQL — core flyway-core supports it -->
```

- [ ] **Step 2: Update `auth-service/src/main/resources/application.yml`**

```yaml
spring:
  datasource:
    url: jdbc:postgresql://${DB_HOST:localhost}:${DB_PORT:5432}/${DB_NAME:go_stock_prediction}?currentSchema=public
    username: ${DB_USER:postgres}
    password: ${DB_PASSWORD:123}
    driver-class-name: org.postgresql.Driver
  jpa:
    hibernate:
      ddl-auto: validate
    show-sql: false
    properties:
      hibernate:
        dialect: org.hibernate.dialect.PostgreSQLDialect
        jdbc:
          time_zone: Asia/Ho_Chi_Minh
  flyway:
    enabled: true
    baseline-on-migrate: true
    locations: classpath:db/migration
```

- [ ] **Step 3: Rewrite Flyway V1 migration for PostgreSQL**

Rewrite `auth-service/src/main/resources/db/migration/V1__create_auth_tables.sql`:

```sql
-- Flyway V1: create market groups tables (users table owned by GORM/Go API)
CREATE TABLE IF NOT EXISTS market_groups (
    id          BIGSERIAL    PRIMARY KEY,
    name        VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    created_at  TIMESTAMP    NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP    NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS market_group_markets (
    group_id    BIGINT      NOT NULL REFERENCES market_groups(id) ON DELETE CASCADE,
    market_key  VARCHAR(20) NOT NULL,
    PRIMARY KEY (group_id, market_key)
);

CREATE TABLE IF NOT EXISTS user_market_groups (
    user_id   BIGINT NOT NULL,
    group_id  BIGINT NOT NULL REFERENCES market_groups(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, group_id)
);
CREATE INDEX IF NOT EXISTS idx_umg_user_id ON user_market_groups(user_id);
```

Note: the FK `user_id → users(id)` is removed because in PostgreSQL the `users` table is created by GORM `AutoMigrate` on Go API startup — Flyway runs before GORM, so the FK would fail if users table doesn't exist yet. Keep it as a soft reference (enforced at application level).

- [ ] **Step 4: Rewrite Flyway V2 migration for PostgreSQL**

Rewrite `auth-service/src/main/resources/db/migration/V2__add_user_profile_fields.sql`:

```sql
-- Flyway V2: add profile fields to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS full_name VARCHAR(100);
ALTER TABLE users ADD COLUMN IF NOT EXISTS email     VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS phone     VARCHAR(30);
```

PostgreSQL supports `ADD COLUMN IF NOT EXISTS` — no `ALTER IGNORE` workaround needed.

- [ ] **Step 5: Build auth service**

```bash
cd auth-service && mvn clean package -DskipTests
```

Expected: `BUILD SUCCESS`

- [ ] **Step 6: Start auth service and verify Flyway**

```bash
docker-compose build auth
docker-compose up auth -d
docker logs auth_service 2>&1 | grep -E "Flyway|HikariPool|Started|ERROR"
```

Expected:
```
Flyway Community Edition ... by Redgate
Successfully validated 2 migrations
Successfully applied 2 migrations to schema "public"
HikariPool-1 - Start completed.
Started AuthServiceApplication
```

- [ ] **Step 7: Commit**

```bash
git add auth-service/pom.xml auth-service/src/main/resources/
git commit -m "feat(auth): switch Spring Boot datasource from MySQL to PostgreSQL, rewrite Flyway migrations"
```

---

## Phase 2: TimescaleDB Hypertables + Compression

### Task 6: Create Hypertables for All Time-Series Tables

**Files:**
- Modify: `database.sql` (add hypertable creation at end)

Hypertables automatically partition a regular PostgreSQL table by a time column into "chunks." Queries transparently span chunks. Old chunks get compressed. This is the foundation that makes unbounded data growth manageable.

**Steps:**

- [ ] **Step 1: Append hypertable creation to `database.sql`**

Add this block after all `CREATE TABLE` statements:

```sql
-- =========================================================
-- TimescaleDB Hypertables
-- Convert time-series tables to hypertables for automatic
-- partitioning, compression, and parallel chunk queries.
-- =========================================================

-- Price tables: 1-month chunks (high write frequency)
SELECT create_hypertable('gold_prices',            'trading_date', chunk_time_interval => INTERVAL '1 month',  if_not_exists => TRUE);
SELECT create_hypertable('nasdaq_prices',           'trading_date', chunk_time_interval => INTERVAL '1 month',  if_not_exists => TRUE);
SELECT create_hypertable('sp500_prices',            'trading_date', chunk_time_interval => INTERVAL '1 month',  if_not_exists => TRUE);
SELECT create_hypertable('crypto_prices',           'trading_date', chunk_time_interval => INTERVAL '1 month',  if_not_exists => TRUE);

-- Intraday tables: 1-week chunks (very high frequency)
SELECT create_hypertable('gold_intraday_prices',    'timestamp', chunk_time_interval => INTERVAL '1 week',  if_not_exists => TRUE);
SELECT create_hypertable('nasdaq_intraday_prices',  'timestamp', chunk_time_interval => INTERVAL '1 week',  if_not_exists => TRUE);
SELECT create_hypertable('sp500_intraday_prices',   'timestamp', chunk_time_interval => INTERVAL '1 week',  if_not_exists => TRUE);
SELECT create_hypertable('crypto_intraday_prices',  'timestamp', chunk_time_interval => INTERVAL '1 week',  if_not_exists => TRUE);

-- Prediction tables: 3-month chunks
SELECT create_hypertable('gold_predictions',        'prediction_date', chunk_time_interval => INTERVAL '3 months', if_not_exists => TRUE);
SELECT create_hypertable('nasdaq_predictions',      'prediction_date', chunk_time_interval => INTERVAL '3 months', if_not_exists => TRUE);
SELECT create_hypertable('sp500_predictions',       'prediction_date', chunk_time_interval => INTERVAL '3 months', if_not_exists => TRUE);
SELECT create_hypertable('crypto_predictions',      'prediction_date', chunk_time_interval => INTERVAL '3 months', if_not_exists => TRUE);

-- Simulation tables
SELECT create_hypertable('sim_trades',              'trade_date',    chunk_time_interval => INTERVAL '3 months', if_not_exists => TRUE);
SELECT create_hypertable('sim_portfolio_snapshots', 'snapshot_date', chunk_time_interval => INTERVAL '3 months', if_not_exists => TRUE);

-- Operational logs: 1-month chunks
SELECT create_hypertable('sync_logs',               'created_at', chunk_time_interval => INTERVAL '1 month', if_not_exists => TRUE);
SELECT create_hypertable('training_logs',           'created_at', chunk_time_interval => INTERVAL '1 month', if_not_exists => TRUE);
```

- [ ] **Step 2: Verify hypertables created**

```bash
docker exec timescaledb psql -U postgres -d go_stock_prediction -c \
  "SELECT hypertable_name, num_chunks FROM timescaledb_information.hypertables ORDER BY 1;"
```

Expected: all 16 tables listed.

- [ ] **Step 3: Commit**

```bash
git add database.sql
git commit -m "feat(db): add TimescaleDB hypertables for all time-series tables"
```

---

### Task 7: Compression Policies — Automatic Storage Management

**Files:**
- Modify: `database.sql` (append compression config)

TimescaleDB compresses chunks older than the policy age using columnar storage. Typical compression ratio: 90-95% for financial time-series. No data is lost — compressed chunks are fully queryable.

**Steps:**

- [ ] **Step 1: Append compression setup to `database.sql`**

```sql
-- =========================================================
-- TimescaleDB Compression Policies
-- Chunks older than 30 days are automatically compressed.
-- Compressed chunks are fully queryable but read-only.
-- Compression ratio: typically 90-95% for price/prediction data.
-- =========================================================

-- Enable compression on each hypertable
ALTER TABLE gold_prices           SET (timescaledb.compress, timescaledb.compress_orderby = 'trading_date DESC', timescaledb.compress_segmentby = 'source');
ALTER TABLE nasdaq_prices         SET (timescaledb.compress, timescaledb.compress_orderby = 'trading_date DESC', timescaledb.compress_segmentby = 'symbol');
ALTER TABLE sp500_prices          SET (timescaledb.compress, timescaledb.compress_orderby = 'trading_date DESC', timescaledb.compress_segmentby = 'symbol');
ALTER TABLE crypto_prices         SET (timescaledb.compress, timescaledb.compress_orderby = 'trading_date DESC', timescaledb.compress_segmentby = 'symbol');

ALTER TABLE gold_intraday_prices   SET (timescaledb.compress, timescaledb.compress_orderby = 'timestamp DESC', timescaledb.compress_segmentby = 'symbol');
ALTER TABLE nasdaq_intraday_prices SET (timescaledb.compress, timescaledb.compress_orderby = 'timestamp DESC', timescaledb.compress_segmentby = 'symbol');
ALTER TABLE sp500_intraday_prices  SET (timescaledb.compress, timescaledb.compress_orderby = 'timestamp DESC', timescaledb.compress_segmentby = 'symbol');
ALTER TABLE crypto_intraday_prices SET (timescaledb.compress, timescaledb.compress_orderby = 'timestamp DESC', timescaledb.compress_segmentby = 'symbol');

ALTER TABLE gold_predictions       SET (timescaledb.compress, timescaledb.compress_orderby = 'prediction_date DESC', timescaledb.compress_segmentby = 'algorithm_name');
ALTER TABLE nasdaq_predictions     SET (timescaledb.compress, timescaledb.compress_orderby = 'prediction_date DESC', timescaledb.compress_segmentby = 'algorithm_name');
ALTER TABLE sp500_predictions      SET (timescaledb.compress, timescaledb.compress_orderby = 'prediction_date DESC', timescaledb.compress_segmentby = 'algorithm_name');
ALTER TABLE crypto_predictions     SET (timescaledb.compress, timescaledb.compress_orderby = 'prediction_date DESC', timescaledb.compress_segmentby = 'algorithm_name');

ALTER TABLE sim_trades             SET (timescaledb.compress, timescaledb.compress_orderby = 'trade_date DESC', timescaledb.compress_segmentby = 'bot_id');
ALTER TABLE sim_portfolio_snapshots SET (timescaledb.compress, timescaledb.compress_orderby = 'snapshot_date DESC', timescaledb.compress_segmentby = 'bot_id');

ALTER TABLE sync_logs              SET (timescaledb.compress, timescaledb.compress_orderby = 'created_at DESC');
ALTER TABLE training_logs          SET (timescaledb.compress, timescaledb.compress_orderby = 'created_at DESC');

-- Add automatic compression policies (compress chunks older than 30 days)
SELECT add_compression_policy('gold_prices',            INTERVAL '30 days', if_not_exists => true);
SELECT add_compression_policy('nasdaq_prices',          INTERVAL '30 days', if_not_exists => true);
SELECT add_compression_policy('sp500_prices',           INTERVAL '30 days', if_not_exists => true);
SELECT add_compression_policy('crypto_prices',          INTERVAL '30 days', if_not_exists => true);
SELECT add_compression_policy('gold_intraday_prices',   INTERVAL '7 days',  if_not_exists => true);
SELECT add_compression_policy('nasdaq_intraday_prices', INTERVAL '7 days',  if_not_exists => true);
SELECT add_compression_policy('sp500_intraday_prices',  INTERVAL '7 days',  if_not_exists => true);
SELECT add_compression_policy('crypto_intraday_prices', INTERVAL '7 days',  if_not_exists => true);
SELECT add_compression_policy('gold_predictions',       INTERVAL '30 days', if_not_exists => true);
SELECT add_compression_policy('nasdaq_predictions',     INTERVAL '30 days', if_not_exists => true);
SELECT add_compression_policy('sp500_predictions',      INTERVAL '30 days', if_not_exists => true);
SELECT add_compression_policy('crypto_predictions',     INTERVAL '30 days', if_not_exists => true);
SELECT add_compression_policy('sim_trades',             INTERVAL '30 days', if_not_exists => true);
SELECT add_compression_policy('sim_portfolio_snapshots',INTERVAL '30 days', if_not_exists => true);
SELECT add_compression_policy('sync_logs',              INTERVAL '30 days', if_not_exists => true);
SELECT add_compression_policy('training_logs',          INTERVAL '30 days', if_not_exists => true);
```

- [ ] **Step 2: Verify compression policies registered**

```bash
docker exec timescaledb psql -U postgres -d go_stock_prediction -c \
  "SELECT hypertable_name, compress_after FROM timescaledb_information.compression_settings ORDER BY 1;"
```

- [ ] **Step 3: (Optional) Manually compress existing old data**

If migrating from an existing MySQL dump with years of data:

```bash
docker exec timescaledb psql -U postgres -d go_stock_prediction -c \
  "SELECT compress_chunk(c) FROM show_chunks('gold_prices', older_than => INTERVAL '30 days') c;"
```

Repeat for other tables. This can take minutes for large datasets.

- [ ] **Step 4: Check compression ratio**

```bash
docker exec timescaledb psql -U postgres -d go_stock_prediction -c \
  "SELECT hypertable_name,
          pg_size_pretty(before_compression_total_bytes) AS before,
          pg_size_pretty(after_compression_total_bytes)  AS after,
          ROUND(100 - 100.0*after_compression_total_bytes/NULLIF(before_compression_total_bytes,0), 1) AS savings_pct
   FROM chunk_compression_stats('gold_prices')
   LIMIT 5;"
```

Expected: 85-95% savings on financial OHLCV data.

- [ ] **Step 5: Commit**

```bash
git add database.sql
git commit -m "feat(db): add TimescaleDB compression policies — 30d chunks compressed automatically"
```

---

### Task 8: Continuous Aggregates for Direction Accuracy

**Files:**
- Modify: `database.sql` (append continuous aggregate definitions)

Continuous aggregates are TimescaleDB's auto-refreshing materialized views for hypertables. They recompute only the changed time buckets — much cheaper than `REFRESH MATERIALIZED VIEW` on a full table.

**Steps:**

- [ ] **Step 1: Append continuous aggregates to `database.sql`**

```sql
-- =========================================================
-- Continuous Aggregates — Direction Accuracy per Algorithm
-- Refreshed automatically by TimescaleDB background job.
-- Only counts rows where direction_correct IS NOT NULL.
-- =========================================================

CREATE MATERIALIZED VIEW IF NOT EXISTS direction_accuracy_gold
WITH (timescaledb.continuous, timescaledb.materialized_only = FALSE) AS
SELECT
    time_bucket('1 day', prediction_date)       AS bucket,
    algorithm_name,
    COUNT(*)                                    AS total,
    COUNT(*) FILTER (WHERE direction_correct = TRUE)  AS correct
FROM gold_predictions
WHERE direction_correct IS NOT NULL
GROUP BY bucket, algorithm_name
WITH NO DATA;

CREATE MATERIALIZED VIEW IF NOT EXISTS direction_accuracy_nasdaq
WITH (timescaledb.continuous, timescaledb.materialized_only = FALSE) AS
SELECT
    time_bucket('1 day', prediction_date)       AS bucket,
    algorithm_name,
    COUNT(*)                                    AS total,
    COUNT(*) FILTER (WHERE direction_correct = TRUE)  AS correct
FROM nasdaq_predictions
WHERE direction_correct IS NOT NULL
GROUP BY bucket, algorithm_name
WITH NO DATA;

CREATE MATERIALIZED VIEW IF NOT EXISTS direction_accuracy_sp500
WITH (timescaledb.continuous, timescaledb.materialized_only = FALSE) AS
SELECT
    time_bucket('1 day', prediction_date)       AS bucket,
    algorithm_name,
    COUNT(*)                                    AS total,
    COUNT(*) FILTER (WHERE direction_correct = TRUE)  AS correct
FROM sp500_predictions
WHERE direction_correct IS NOT NULL
GROUP BY bucket, algorithm_name
WITH NO DATA;

CREATE MATERIALIZED VIEW IF NOT EXISTS direction_accuracy_crypto
WITH (timescaledb.continuous, timescaledb.materialized_only = FALSE) AS
SELECT
    time_bucket('1 day', prediction_date)       AS bucket,
    algorithm_name,
    COUNT(*)                                    AS total,
    COUNT(*) FILTER (WHERE direction_correct = TRUE)  AS correct
FROM crypto_predictions
WHERE direction_correct IS NOT NULL
GROUP BY bucket, algorithm_name
WITH NO DATA;

-- Refresh policies: refresh daily at 6AM (after reconcile runs at 6AM)
SELECT add_continuous_aggregate_policy('direction_accuracy_gold',
    start_offset => INTERVAL '7 days', end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 day', if_not_exists => true);
SELECT add_continuous_aggregate_policy('direction_accuracy_nasdaq',
    start_offset => INTERVAL '7 days', end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 day', if_not_exists => true);
SELECT add_continuous_aggregate_policy('direction_accuracy_sp500',
    start_offset => INTERVAL '7 days', end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 day', if_not_exists => true);
SELECT add_continuous_aggregate_policy('direction_accuracy_crypto',
    start_offset => INTERVAL '7 days', end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 day', if_not_exists => true);
```

- [ ] **Step 2: Update `GET /api/predictions/direction-accuracy` to read from aggregates**

In `api/pkg/store/mysql/direction_accuracy.go`, update the query to read from the continuous aggregates instead of the raw prediction tables:

```go
// New query reads from direction_accuracy_{market} views — runs in milliseconds.
func (c *Client) GetDirectionAccuracy(market string) ([]modelsapi.DirectionAccuracyRow, error) {
    viewName := fmt.Sprintf("direction_accuracy_%s", strings.ToLower(market))
    query := fmt.Sprintf(`
        SELECT algorithm_name AS algorithm,
               SUM(total)   AS total,
               SUM(correct) AS correct
        FROM %s
        GROUP BY algorithm_name
        ORDER BY algorithm_name
    `, viewName)
    // ... scan rows into []DirectionAccuracyRow
}
```

- [ ] **Step 3: Verify continuous aggregates**

```bash
# Manually refresh for testing
docker exec timescaledb psql -U postgres -d go_stock_prediction -c \
  "CALL refresh_continuous_aggregate('direction_accuracy_gold', NULL, NULL);"

docker exec timescaledb psql -U postgres -d go_stock_prediction -c \
  "SELECT algorithm_name, SUM(total), SUM(correct) FROM direction_accuracy_gold GROUP BY 1 ORDER BY 1;"
```

- [ ] **Step 4: Commit**

```bash
git add database.sql api/pkg/store/mysql/direction_accuracy.go
git commit -m "feat(db): add TimescaleDB continuous aggregates for direction accuracy"
```

---

## Phase 3: Pre-computed KPIs in sim_sessions

### Task 9: Add KPI Columns to SimSession — Go GORM Model

**Files:**
- Modify: `api/pkg/models/models_db/simulation.go`

The `sim_sessions` table already has KPI columns in the new schema (Task 2). Now update the Go GORM struct to match.

**Steps:**

- [ ] **Step 1: Update `SimSession` struct in `api/pkg/models/models_db/simulation.go`**

```go
type SimSession struct {
    ID        int64      `gorm:"primaryKey;autoIncrement" json:"id"`
    BotID     string     `gorm:"size:50;not null;index" json:"bot_id"`
    StartDate time.Time  `gorm:"not null" json:"start_date"`
    EndDate   *time.Time `json:"end_date,omitempty"`
    Status    string     `gorm:"size:20;default:'running'" json:"status"`
    Mode      string     `gorm:"size:20;default:'backtest'" json:"mode"`
    CreatedAt time.Time  `json:"created_at"`

    // Pre-computed KPIs — set by Python after session end
    TotalTrades    int              `gorm:"default:0"  json:"total_trades"`
    Wins           int              `gorm:"default:0"  json:"wins"`
    Losses         int              `gorm:"default:0"  json:"losses"`
    Breakeven      int              `gorm:"default:0"  json:"breakeven"`
    TotalPnl       decimal.Decimal  `gorm:"type:decimal(20,2);default:0" json:"total_pnl"`
    TotalReturnPct *decimal.Decimal `gorm:"type:decimal(8,4)"  json:"total_return_pct,omitempty"`
    WinRate        *decimal.Decimal `gorm:"type:decimal(5,4)"  json:"win_rate,omitempty"`
    ProfitFactor   *decimal.Decimal `gorm:"type:decimal(8,4)"  json:"profit_factor,omitempty"`
    MaxDrawdownPct *decimal.Decimal `gorm:"type:decimal(8,4)"  json:"max_drawdown_pct,omitempty"`
    KpiUpdatedAt   *time.Time       `json:"kpi_updated_at,omitempty"`
}
```

- [ ] **Step 2: Build to verify no compile errors**

```bash
cd api && go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add api/pkg/models/models_db/simulation.go
git commit -m "feat(api): add pre-computed KPI fields to SimSession GORM model"
```

---

### Task 10: Python Service — Compute and Store KPIs After Session End

**Files:**
- Modify: `prediction/src/database/repository.py` (add `update_session_kpis`)
- Modify: `prediction/src/orchestrator/training.py` (call `update_session_kpis` after each session)

**Steps:**

- [ ] **Step 1: Add `update_session_kpis` to `prediction/src/database/repository.py`**

```python
def update_session_kpis(self, session_id: int) -> None:
    """Compute trade statistics and store pre-aggregated KPIs in sim_sessions."""
    with self.get_session() as db:
        # Aggregate SELL trades only (they carry realized PnL)
        row = db.execute(text("""
            SELECT
                COUNT(*)                                    AS total_trades,
                COUNT(*) FILTER (WHERE pnl > 0)            AS wins,
                COUNT(*) FILTER (WHERE pnl < 0)            AS losses,
                COUNT(*) FILTER (WHERE pnl = 0)            AS breakeven,
                COALESCE(SUM(pnl), 0)                      AS total_pnl,
                COALESCE(SUM(pnl) FILTER (WHERE pnl > 0), 0) AS win_pnl,
                COALESCE(ABS(SUM(pnl) FILTER (WHERE pnl < 0)), 0) AS loss_pnl
            FROM sim_trades
            WHERE session_id = :sid AND action = 'SELL'
        """), {"sid": session_id}).fetchone()

        total   = row.total_trades or 0
        wins    = row.wins or 0
        losses  = row.losses or 0
        total_pnl = float(row.total_pnl or 0)
        win_rate = (wins / total) if total > 0 else None
        profit_factor = (row.win_pnl / row.loss_pnl) if row.loss_pnl and row.loss_pnl > 0 else None

        # Compute max drawdown from portfolio snapshots
        snaps = db.execute(text("""
            SELECT total_value FROM sim_portfolio_snapshots
            WHERE session_id = :sid ORDER BY snapshot_date
        """), {"sid": session_id}).fetchall()

        max_drawdown_pct = None
        if snaps:
            values = [float(s.total_value) for s in snaps]
            peak = values[0]
            max_dd = 0.0
            for v in values:
                peak = max(peak, v)
                if peak > 0:
                    dd = (peak - v) / peak * 100
                    max_dd = max(max_dd, dd)
            max_drawdown_pct = max_dd

        # Get initial capital for return pct
        sess = db.execute(text("""
            SELECT b.initial_capital
            FROM sim_sessions s JOIN sim_bots b ON b.id = s.bot_id
            WHERE s.id = :sid
        """), {"sid": session_id}).fetchone()
        initial_capital = float(sess.initial_capital) if sess else None
        total_return_pct = (total_pnl / initial_capital * 100) if initial_capital else None

        db.execute(text("""
            UPDATE sim_sessions SET
                total_trades    = :total,
                wins            = :wins,
                losses          = :losses,
                breakeven       = :breakeven,
                total_pnl       = :total_pnl,
                total_return_pct= :return_pct,
                win_rate        = :win_rate,
                profit_factor   = :profit_factor,
                max_drawdown_pct= :max_dd,
                kpi_updated_at  = NOW()
            WHERE id = :sid
        """), {
            "total": total, "wins": wins, "losses": losses,
            "breakeven": row.breakeven or 0,
            "total_pnl": total_pnl,
            "return_pct": total_return_pct,
            "win_rate": win_rate,
            "profit_factor": profit_factor,
            "max_dd": max_drawdown_pct,
            "sid": session_id,
        })
        db.commit()
```

- [ ] **Step 2: Call `update_session_kpis` after each session completes**

In `prediction/src/orchestrator/training.py` (or wherever sessions are finalized), find the point where `session.status = 'completed'` is set and add the KPI call:

```python
# After marking session completed:
session.status = 'completed'
session.end_date = datetime.now()
db.commit()

# Compute and store pre-aggregated KPIs
repo.update_session_kpis(session.id)
log.info("session.kpis_updated", session_id=session.id)
```

Similarly call it at the end of `run_live_step` in the simulation engine when a live session snapshot is taken.

- [ ] **Step 3: Verify KPIs written**

After triggering a backtest via API:

```bash
docker exec timescaledb psql -U postgres -d go_stock_prediction -c \
  "SELECT id, total_trades, wins, losses, total_pnl, win_rate, kpi_updated_at
   FROM sim_sessions WHERE kpi_updated_at IS NOT NULL ORDER BY kpi_updated_at DESC LIMIT 5;"
```

- [ ] **Step 4: Commit**

```bash
git add prediction/src/database/repository.py prediction/src/orchestrator/training.py
git commit -m "feat(prediction): compute and store pre-aggregated KPIs in sim_sessions after session end"
```

---

### Task 11: Go API — Leaderboard Reads from Pre-computed KPIs

**Files:**
- Modify: `api/pkg/server/api_simulation.go`
- Modify: `api/pkg/store/repository/simulation.go` (add new interface method)
- Modify: `api/pkg/store/mysql/simulation.go` (implement)

Now that KPIs are pre-computed, the leaderboard can be a simple JOIN query instead of N+1 aggregation.

**Steps:**

- [ ] **Step 1: Add `GetLeaderboardEntries` to `api/pkg/store/repository/simulation.go`**

```go
// GetLeaderboardEntries returns all sessions with pre-computed KPIs joined with bot config.
// Returns one entry per session — caller picks best session per bot.
GetLeaderboardEntries(mode string) ([]modelsdb.LeaderboardEntry, error)
```

Add `LeaderboardEntry` struct to `api/pkg/models/models_db/simulation.go`:

```go
type LeaderboardEntry struct {
    SessionID       int64            `gorm:"column:session_id"`
    BotID           string           `gorm:"column:bot_id"`
    Market          string           `gorm:"column:market"`
    Algorithm       string           `gorm:"column:algorithm"`
    DisplayName     string           `gorm:"column:display_name"`
    InitialCapital  decimal.Decimal  `gorm:"column:initial_capital"`
    Currency        string           `gorm:"column:currency"`
    StartDate       time.Time        `gorm:"column:start_date"`
    EndDate         *time.Time       `gorm:"column:end_date"`
    Mode            string           `gorm:"column:mode"`
    Status          string           `gorm:"column:status"`
    TotalTrades     int              `gorm:"column:total_trades"`
    Wins            int              `gorm:"column:wins"`
    Losses          int              `gorm:"column:losses"`
    Breakeven       int              `gorm:"column:breakeven"`
    TotalPnl        decimal.Decimal  `gorm:"column:total_pnl"`
    TotalReturnPct  *decimal.Decimal `gorm:"column:total_return_pct"`
    WinRate         *decimal.Decimal `gorm:"column:win_rate"`
    ProfitFactor    *decimal.Decimal `gorm:"column:profit_factor"`
    MaxDrawdownPct  *decimal.Decimal `gorm:"column:max_drawdown_pct"`
    CurrentValue    *decimal.Decimal `gorm:"column:current_value"`
}
```

- [ ] **Step 2: Implement `GetLeaderboardEntries` in `api/pkg/store/mysql/simulation.go`**

```go
func (c *Client) GetLeaderboardEntries(mode string) ([]modelsdb.LeaderboardEntry, error) {
    var entries []modelsdb.LeaderboardEntry
    err := c.Db.Raw(`
        SELECT
            s.id              AS session_id,
            s.bot_id,
            b.market,
            b.algorithm,
            b.display_name,
            b.initial_capital,
            b.currency,
            s.start_date,
            s.end_date,
            s.mode,
            s.status,
            s.total_trades,
            s.wins,
            s.losses,
            s.breakeven,
            s.total_pnl,
            s.total_return_pct,
            s.win_rate,
            s.profit_factor,
            s.max_drawdown_pct,
            snap.total_value  AS current_value
        FROM sim_sessions s
        JOIN sim_bots b ON b.id = s.bot_id
        LEFT JOIN LATERAL (
            SELECT total_value
            FROM sim_portfolio_snapshots
            WHERE session_id = s.id
            ORDER BY snapshot_date DESC
            LIMIT 1
        ) snap ON true
        WHERE s.mode = ? AND s.status IN ('completed', 'running')
        ORDER BY s.total_return_pct DESC NULLS LAST
    `, mode).Scan(&entries).Error
    return entries, err
}
```

Note: `LATERAL` is a PostgreSQL feature — this is more efficient than a subquery in MySQL.

- [ ] **Step 3: Simplify `GetSimLeaderboard` in `api/pkg/server/api_simulation.go`**

The current handler does N+1 queries. Replace with the single batch query:

```go
func (s *Server) GetSimLeaderboard(w http.ResponseWriter, r *http.Request) {
    // 60-second cache remains
    if cached, ok := leaderboardCache.Get(); ok {
        ResponseSuccess(w, cached)
        return
    }

    mode := r.URL.Query().Get("mode")
    if mode == "" { mode = "backtest" }

    entries, err := s.store.GetLeaderboardEntries(mode)
    if err != nil {
        ResponseError(w, http.StatusInternalServerError, err.Error())
        return
    }

    // Pick best session per bot (already sorted by return_pct DESC)
    seen := map[string]bool{}
    var result []leaderboardEntry
    for _, e := range entries {
        if seen[e.BotID] { continue }
        seen[e.BotID] = true
        result = append(result, toLeaderboardEntry(e))
    }

    leaderboardCache.Set(result)
    ResponseSuccess(w, result)
}
```

- [ ] **Step 4: Benchmark the new query**

```bash
TOKEN=$(curl -s -X POST http://localhost:8118/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"chon","password":"..."}' | jq -r '.token')

time curl -s http://localhost:8118/api/simulation/leaderboard \
  -H "Authorization: Bearer $TOKEN" | jq '.data | length'
```

Expected: under 50ms cold (was 6+ seconds), under 5ms cached.

- [ ] **Step 5: Commit**

```bash
git add api/pkg/server/api_simulation.go api/pkg/store/repository/simulation.go api/pkg/store/mysql/simulation.go api/pkg/models/models_db/simulation.go
git commit -m "perf(api): leaderboard reads from pre-computed KPI columns — single JOIN query"
```

---

## Phase 4: Materialized Views for Monitoring Overview

### Task 12: Create Monitoring Materialized View

**Files:**
- Modify: `database.sql` (append monitoring view)
- Modify: `prediction/src/database/repository.py` (add `refresh_monitoring_view`)
- Modify: `api/pkg/store/mysql/monitoring.go` (read from view if beneficial)

The `/api/monitoring/overview` endpoint currently runs multiple raw SQL queries per request (cached 30s). With a materialized view, the cache fallback becomes a simple `SELECT *` scan.

**Steps:**

- [ ] **Step 1: Append monitoring view to `database.sql`**

```sql
-- =========================================================
-- Monitoring Materialized View
-- Refreshed by Python after each crawl / predict run.
-- =========================================================
CREATE MATERIALIZED VIEW IF NOT EXISTS monitoring_crawl_stats AS
SELECT
    'gold'   AS market,
    MAX(trading_date) AS last_daily_at,
    COUNT(*) FILTER (WHERE trading_date >= NOW() - INTERVAL '1 day') AS daily_today
FROM gold_prices
UNION ALL
SELECT
    'nasdaq' AS market,
    MAX(trading_date), COUNT(*) FILTER (WHERE trading_date >= NOW() - INTERVAL '1 day')
FROM nasdaq_prices
UNION ALL
SELECT
    'sp500'  AS market,
    MAX(trading_date), COUNT(*) FILTER (WHERE trading_date >= NOW() - INTERVAL '1 day')
FROM sp500_prices
UNION ALL
SELECT
    'crypto' AS market,
    MAX(trading_date), COUNT(*) FILTER (WHERE trading_date >= NOW() - INTERVAL '1 day')
FROM crypto_prices;

CREATE UNIQUE INDEX IF NOT EXISTS idx_monitoring_crawl_market ON monitoring_crawl_stats(market);
```

- [ ] **Step 2: Add `refresh_monitoring_view` to Python repository**

```python
def refresh_monitoring_view(self) -> None:
    """Refresh monitoring materialized view after crawl/predict runs."""
    with self.get_session() as db:
        db.execute(text("REFRESH MATERIALIZED VIEW CONCURRENTLY monitoring_crawl_stats"))
        db.commit()
    log.info("monitoring_view.refreshed")
```

- [ ] **Step 3: Call refresh after each successful crawl**

In `prediction/src/scheduler/jobs.py`, in `_run_pipeline()` after the crawl step:

```python
crawl_result = crawler.crawl()
repo.refresh_monitoring_view()   # <-- add this line
```

- [ ] **Step 4: Verify view**

```bash
docker exec timescaledb psql -U postgres -d go_stock_prediction -c \
  "SELECT * FROM monitoring_crawl_stats;"
```

- [ ] **Step 5: Commit**

```bash
git add database.sql prediction/src/database/repository.py prediction/src/scheduler/jobs.py
git commit -m "feat(db): add monitoring_crawl_stats materialized view, refresh after each crawl"
```

---

## Phase 5: Redis for Volatile Hot State (Optional Enhancement)

This phase is optional. The previous phases deliver the major performance gains. Add Redis only if real-time latency (<5ms) is needed for latest prices/predictions on the dashboard.

### Task 13: Add Redis to docker-compose

**Files:**
- Modify: `docker-compose.yml`

**Steps:**

- [ ] **Step 1: Add Redis service**

```yaml
  redis:
    image: redis:7-alpine
    container_name: redis
    ports:
      - "127.0.0.1:6379:6379"
    volumes:
      - redis_data:/data
    command: redis-server --appendonly yes --maxmemory 256mb --maxmemory-policy allkeys-lru
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
    restart: unless-stopped
```

Add `redis_data:` to the `volumes:` section.

---

### Task 14: Python Writes Latest Predictions to Redis

**Files:**
- Modify: `prediction/pyproject.toml` (add `redis>=5.0`)
- Create: `prediction/src/cache/redis_client.py`
- Modify: `prediction/src/orchestrator/runner.py` (publish after predict)

**Steps:**

- [ ] **Step 1: Add Redis client module at `prediction/src/cache/redis_client.py`**

```python
"""Redis client for publishing hot prediction state."""
from __future__ import annotations
import json
from datetime import datetime
from typing import Any

import redis

from src.config import get_settings
from src.utils.logger import get_logger

log = get_logger("redis")
_client: redis.Redis | None = None


def get_redis() -> redis.Redis:
    global _client
    if _client is None:
        cfg = get_settings()
        _client = redis.Redis(
            host=getattr(cfg, "redis_host", "redis"),
            port=getattr(cfg, "redis_port", 6379),
            decode_responses=True,
        )
    return _client


def publish_prediction(market: str, algorithm: str, predicted_price: float,
                       current_price: float, confidence: float) -> None:
    key = f"prediction:latest:{market.lower()}:{algorithm}"
    payload = json.dumps({
        "predicted_price": predicted_price,
        "current_price": current_price,
        "confidence": confidence,
        "updated_at": datetime.now().isoformat(),
    })
    get_redis().set(key, payload, ex=7200)   # expires in 2h


def publish_latest_price(market: str, symbol: str, price: float) -> None:
    key = f"price:latest:{market.lower()}:{symbol}"
    get_redis().set(key, json.dumps({"price": price, "updated_at": datetime.now().isoformat()}), ex=3600)
```

- [ ] **Step 2: Call `publish_prediction` in `runner.py` after each successful prediction**

```python
from src.cache.redis_client import publish_prediction

# After saving prediction to DB:
publish_prediction(market_key, result.algorithm_name,
                   float(result.predicted_price), float(result.current_price),
                   float(result.confidence))
```

- [ ] **Step 3: Commit**

```bash
git add prediction/src/cache/ prediction/src/orchestrator/runner.py prediction/pyproject.toml
git commit -m "feat(prediction): publish latest predictions to Redis after each predict run"
```

---

### Task 15: Go API Reads Latest Predictions from Redis

**Files:**
- Modify: `api/go.mod` (add `go-redis/v9`)
- Create: `api/pkg/cache/redis.go`
- Modify: relevant endpoints to check Redis first

**Steps:**

- [ ] **Step 1: Add go-redis**

```bash
cd api && go get github.com/redis/go-redis/v9
```

- [ ] **Step 2: Create `api/pkg/cache/redis.go`**

```go
package cache

import (
    "context"
    "encoding/json"
    "fmt"
    "os"

    "github.com/redis/go-redis/v9"
)

var rdb *redis.Client

func Init() {
    host := os.Getenv("REDIS_HOST")
    if host == "" { host = "redis" }
    port := os.Getenv("REDIS_PORT")
    if port == "" { port = "6379" }
    rdb = redis.NewClient(&redis.Options{Addr: host + ":" + port})
}

func GetLatestPrediction(ctx context.Context, market, algorithm string) (map[string]any, error) {
    key := fmt.Sprintf("prediction:latest:%s:%s", market, algorithm)
    val, err := rdb.Get(ctx, key).Result()
    if err != nil { return nil, err }
    var out map[string]any
    return out, json.Unmarshal([]byte(val), &out)
}
```

- [ ] **Step 3: Commit**

```bash
git add api/pkg/cache/ api/go.mod api/go.sum
git commit -m "feat(api): add Redis client for hot prediction state reads"
```

---

## Data Migration from MySQL

If migrating an existing production MySQL database (not starting fresh):

**Steps:**

- [ ] **Step 1: Export MySQL data**

```bash
mysqldump -u root -p123 go_stock_prediction \
  --no-create-info --skip-triggers --compatible=ansi \
  --single-transaction > data_export.sql
```

- [ ] **Step 2: Install pgloader**

```bash
docker run --rm dimitri/pgloader pgloader --version
```

- [ ] **Step 3: Create pgloader config `database_migrate.sh`**

```bash
#!/bin/bash
# pgloader migration from MySQL to PostgreSQL

docker run --rm --network host dimitri/pgloader pgloader \
  "mysql://root:123@localhost:3306/go_stock_prediction" \
  "postgresql://postgres:123@localhost:5432/go_stock_prediction" \
  --with "data only" \
  --with "workers = 4" \
  --with "concurrency = 2"
```

pgloader handles:
- Type coercion (TINYINT(1) → boolean, DATETIME → timestamp)
- Charset conversion (utf8mb4 → UTF-8)
- Batch inserts for performance

- [ ] **Step 4: Run migration**

```bash
# Start only the DBs (both MySQL old and new PostgreSQL)
docker-compose up db -d  # TimescaleDB
chmod +x database_migrate.sh && ./database_migrate.sh
```

- [ ] **Step 5: Verify row counts match**

```bash
# Compare row counts between source and destination
mysql -u root -p123 go_stock_prediction -e \
  "SELECT 'gold_prices', COUNT(*) FROM gold_prices UNION ALL SELECT 'gold_predictions', COUNT(*) FROM gold_predictions;"

docker exec timescaledb psql -U postgres -d go_stock_prediction -c \
  "SELECT 'gold_prices', COUNT(*) FROM gold_prices UNION ALL SELECT 'gold_predictions', COUNT(*) FROM gold_predictions;"
```

Expected: identical counts.

- [ ] **Step 6: Re-run compression on migrated data**

```bash
docker exec timescaledb psql -U postgres -d go_stock_prediction -c "
    SELECT compress_chunk(c) FROM show_chunks('gold_prices', older_than => INTERVAL '30 days') c;
    SELECT compress_chunk(c) FROM show_chunks('nasdaq_prices', older_than => INTERVAL '30 days') c;
    SELECT compress_chunk(c) FROM show_chunks('sp500_prices', older_than => INTERVAL '30 days') c;
    SELECT compress_chunk(c) FROM show_chunks('crypto_prices', older_than => INTERVAL '30 days') c;
"
```

---

## Expected Outcomes After Each Phase

| After Phase | Leaderboard cold | Monitoring cold | Storage (1yr) | Direction accuracy |
|---|---|---|---|---|
| Before (MySQL) | 6-8s | 3-5s | baseline | 2-3s |
| Phase 1 (PostgreSQL) | 6-8s | 3-5s | baseline | 2-3s |
| Phase 2 (Hypertables+compression) | 4-6s | 2-4s | -85% (cold chunks compressed) | 500ms |
| Phase 3 (Pre-computed KPIs) | **<100ms** | 2-4s | same | 500ms |
| Phase 4 (Materialized views) | <50ms | **<50ms** | same | **<10ms** |
| Phase 5 (Redis) | <10ms | <20ms | same | <10ms |

---

## Rollback Plan

Each phase is independently reversible:

- **Phase 1 rollback:** `cp docker-compose.yml.mysql-backup docker-compose.yml && docker-compose up db -d` — MySQL is back. Go/Python/Java connection strings revert to MySQL DSNs.
- **Phase 2 rollback:** Drop hypertables (`SELECT revert_hypertable('...')`) and regular tables remain.
- **Phase 3 rollback:** Drop KPI columns (`ALTER TABLE sim_sessions DROP COLUMN total_trades...`). Leaderboard falls back to N+1 batch queries.
- **Phase 4 rollback:** `DROP MATERIALIZED VIEW leaderboard_mv CASCADE`. Queries go back to direct table reads.
- **Phase 5 rollback:** Remove Redis from docker-compose. Go API falls through to DB on cache miss.
