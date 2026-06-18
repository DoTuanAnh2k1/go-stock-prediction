#!/usr/bin/env bash
# ==============================================================
# database_migrate.sh — Migrate data from MySQL → PostgreSQL
# using pgloader (https://pgloader.io/)
#
# Usage:
#   ./database_migrate.sh [OPTIONS]
#
# Options:
#   --mysql-host HOST      MySQL host          (default: 127.0.0.1)
#   --mysql-port PORT      MySQL port          (default: 3306)
#   --mysql-user USER      MySQL user          (default: root)
#   --mysql-pass PASS      MySQL password      (default: from MYSQL_PASSWORD env)
#   --mysql-db   DB        MySQL database      (default: go_stock_prediction)
#   --pg-host    HOST      PostgreSQL host     (default: 127.0.0.1)
#   --pg-port    PORT      PostgreSQL port     (default: 5432)
#   --pg-user    USER      PostgreSQL user     (default: postgres)
#   --pg-pass    PASS      PostgreSQL password (default: from POSTGRES_PASSWORD env)
#   --pg-db      DB        PostgreSQL database (default: go_stock_prediction)
#   --dry-run              Print pgloader command without running it
#   -h, --help             Show this help message
#
# Prerequisites:
#   - pgloader installed and on PATH  (apt install pgloader  OR  brew install pgloader)
#   - The target PostgreSQL DB already exists and database.sql has been loaded:
#       psql -U postgres go_stock_prediction < database.sql
#
# Tables migrated (all non-legacy tables used by the application):
#   sync_logs, gold_prices, gold_intraday_prices, gold_predictions,
#   nasdaq_prices, nasdaq_intraday_prices, nasdaq_predictions,
#   sp500_prices, sp500_intraday_prices, sp500_predictions,
#   crypto_prices, crypto_intraday_prices, crypto_predictions,
#   macro_indicators, training_logs, cron_schedules,
#   users, market_groups, market_group_markets, user_market_groups,
#   sim_bots, sim_sessions, sim_trades, sim_portfolio_snapshots
#
# Tables intentionally NOT migrated (legacy VN-stock tables):
#   exchanges, stocks, stock_prices, predictions
# ==============================================================

set -euo pipefail

# ---- Defaults ------------------------------------------------
MYSQL_HOST="${MYSQL_HOST:-127.0.0.1}"
MYSQL_PORT="${MYSQL_PORT:-3306}"
MYSQL_USER="${MYSQL_USER:-root}"
MYSQL_PASS="${MYSQL_PASSWORD:-}"
MYSQL_DB="${MYSQL_DB:-go_stock_prediction}"

PG_HOST="${PG_HOST:-127.0.0.1}"
PG_PORT="${PG_PORT:-5432}"
PG_USER="${PG_USER:-postgres}"
PG_PASS="${POSTGRES_PASSWORD:-}"
PG_DB="${PG_DB:-go_stock_prediction}"

DRY_RUN=0

# ---- Argument parsing ----------------------------------------
while [[ $# -gt 0 ]]; do
    case "$1" in
        --mysql-host) MYSQL_HOST="$2"; shift 2 ;;
        --mysql-port) MYSQL_PORT="$2"; shift 2 ;;
        --mysql-user) MYSQL_USER="$2"; shift 2 ;;
        --mysql-pass) MYSQL_PASS="$2"; shift 2 ;;
        --mysql-db)   MYSQL_DB="$2";   shift 2 ;;
        --pg-host)    PG_HOST="$2";    shift 2 ;;
        --pg-port)    PG_PORT="$2";    shift 2 ;;
        --pg-user)    PG_USER="$2";    shift 2 ;;
        --pg-pass)    PG_PASS="$2";    shift 2 ;;
        --pg-db)      PG_DB="$2";      shift 2 ;;
        --dry-run)    DRY_RUN=1;       shift ;;
        -h|--help)
            sed -n '2,50p' "$0" | grep '^#' | sed 's/^# \{0,2\}//'
            exit 0
            ;;
        *) echo "Unknown option: $1" >&2; exit 1 ;;
    esac
done

# ---- Validate prerequisites ----------------------------------
if ! command -v pgloader &>/dev/null; then
    echo "ERROR: pgloader is not installed or not on PATH." >&2
    echo "  Install: apt install pgloader  OR  brew install pgloader" >&2
    exit 1
fi

echo "=== MySQL → PostgreSQL migration ==="
echo "  Source: mysql://${MYSQL_USER}@${MYSQL_HOST}:${MYSQL_PORT}/${MYSQL_DB}"
echo "  Target: postgresql://${PG_USER}@${PG_HOST}:${PG_PORT}/${PG_DB}"
echo ""

# ---- Build pgloader command file in a temp directory ---------
PGLOADER_FILE=$(mktemp /tmp/pgloader_migrate_XXXXXX.load)
trap 'rm -f "$PGLOADER_FILE"' EXIT

# Encode passwords for URLs (basic % encoding for @ and spaces)
url_encode() {
    local raw="$1"
    printf '%s' "$raw" | python3 -c "import sys, urllib.parse; print(urllib.parse.quote(sys.stdin.read(), safe=''))"
}

MYSQL_PASS_ENC=$(url_encode "$MYSQL_PASS")
PG_PASS_ENC=$(url_encode "$PG_PASS")

# Build connection strings
MYSQL_DSN="mysql://${MYSQL_USER}:${MYSQL_PASS_ENC}@${MYSQL_HOST}:${MYSQL_PORT}/${MYSQL_DB}"
PG_DSN="postgresql://${PG_USER}:${PG_PASS_ENC}@${PG_HOST}:${PG_PORT}/${PG_DB}"

cat > "$PGLOADER_FILE" <<PGLOADER
LOAD DATABASE
  FROM      ${MYSQL_DSN}
  INTO      ${PG_DSN}

WITH
  -- Use PostgreSQL COPY for fast bulk load
  workers = 4,
  concurrency = 1,
  batch rows = 25000,
  batch size = 20MB,
  prefetch rows = 25000,
  -- Drop and recreate indexes after load (faster)
  create indexes,
  -- Reset sequences so BIGSERIAL starts from max(id)+1
  reset sequences,
  -- Use 'public' schema
  schema only = false,
  -- Do not attempt to include/create schema objects already in PG
  create tables = false,
  include drop = false

-- Type casting: MySQL TINYINT(1) → BOOLEAN; DATETIME → TIMESTAMP (no TZ)
CAST
  type tinyint  when (= precision 1) to boolean    drop typemod,
  type datetime                       to timestamp  drop typemod,
  type timestamp                      to timestamp  drop typemod,
  type text                           to text,
  type mediumtext                     to text,
  type longtext                       to text,
  type decimal    to numeric,
  type double     to double precision,
  type bigint unsigned to bigint

-- Only migrate the application tables; skip legacy VN-stock tables
INCLUDING ONLY TABLE NAMES MATCHING
  'sync_logs',
  'gold_prices',
  'gold_intraday_prices',
  'gold_predictions',
  'nasdaq_prices',
  'nasdaq_intraday_prices',
  'nasdaq_predictions',
  'sp500_prices',
  'sp500_intraday_prices',
  'sp500_predictions',
  'crypto_prices',
  'crypto_intraday_prices',
  'crypto_predictions',
  'macro_indicators',
  'training_logs',
  'cron_schedules',
  'users',
  'market_groups',
  'market_group_markets',
  'user_market_groups',
  'sim_bots',
  'sim_sessions',
  'sim_trades',
  'sim_portfolio_snapshots'

SET PostgreSQL PARAMETERS
  maintenance_work_mem to '256MB',
  work_mem             to '64MB'

;
PGLOADER

if [[ "$DRY_RUN" -eq 1 ]]; then
    echo "--- pgloader command file (dry-run, passwords redacted) ---"
    sed "s|${MYSQL_PASS_ENC}|***|g; s|${PG_PASS_ENC}|***|g" "$PGLOADER_FILE"
    echo "--- end ---"
    echo "Dry run complete. Not running pgloader."
    exit 0
fi

echo "Running pgloader..."
pgloader "$PGLOADER_FILE"
PGLOADER_EXIT=$?

if [[ "$PGLOADER_EXIT" -ne 0 ]]; then
    echo ""
    echo "ERROR: pgloader exited with code ${PGLOADER_EXIT}." >&2
    echo "Check the pgloader output above for details." >&2
    exit "$PGLOADER_EXIT"
fi

echo ""
echo "=== Migration complete ==="
echo ""
echo "Post-migration steps:"
echo "  1. Verify row counts in PostgreSQL match MySQL."
echo "  2. Refresh monitoring view: REFRESH MATERIALIZED VIEW CONCURRENTLY monitoring_crawl_stats;"
echo "  3. Seed hypertable continuous aggregates (if needed):"
echo "       CALL refresh_continuous_aggregate('gold_direction_accuracy_daily', NULL, NULL);"
echo "       -- Repeat for nasdaq, sp500, crypto."
echo "  4. Update connection strings in .env to point to PostgreSQL."
echo "  5. Restart all application services."
