#!/usr/bin/env bash
# =============================================================================
# 00-init-databases.sh — database-per-service bootstrap for the shared
# TimescaleDB *instance*. Runs once, on an empty data volume, from the Postgres
# entrypoint (/docker-entrypoint-initdb.d). Creates THREE isolated databases,
# each owned by a dedicated least-privilege role — no service can reach another
# service's tables at the SQL layer.
#
#   auth_db      owner auth_svc        (auth-svc; Flyway V1..V3 populates it)
#   market_db    owner prediction_svc  (prediction-svc owns; api_svc reads)
#   registry_db  owner service_mgt     (service-mgt)
#
# api_svc is a read-model consumer of market_db: SELECT on everything, plus the
# two tables api-svc actually writes (cron_schedules, sim_bots). It has NO access
# to auth_db or registry_db.
#
# Role passwords come from env (set on the db container): AUTH_DB_PASSWORD,
# MARKET_DB_PASSWORD, REGISTRY_DB_PASSWORD, API_DB_PASSWORD. Fail fast if unset.
#
# Schema files live in /schemas (mounted separately so they do NOT auto-run
# against the default database).
# =============================================================================
set -euo pipefail

: "${AUTH_DB_PASSWORD:?AUTH_DB_PASSWORD must be set}"
: "${MARKET_DB_PASSWORD:?MARKET_DB_PASSWORD must be set}"
: "${REGISTRY_DB_PASSWORD:?REGISTRY_DB_PASSWORD must be set}"
: "${API_DB_PASSWORD:?API_DB_PASSWORD must be set}"

SCHEMA_DIR="${SCHEMA_DIR:-/schemas}"
PSQL=(psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER")

echo ">> [db-init] creating roles + databases (database-per-service)"

# --- roles (LOGIN, no superuser). Idempotent via DO block. ---
"${PSQL[@]}" --dbname "$POSTGRES_DB" <<SQL
DO \$\$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'auth_svc') THEN
    CREATE ROLE auth_svc       LOGIN PASSWORD '${AUTH_DB_PASSWORD}';
  END IF;
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'prediction_svc') THEN
    CREATE ROLE prediction_svc LOGIN PASSWORD '${MARKET_DB_PASSWORD}';
  END IF;
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'service_mgt') THEN
    CREATE ROLE service_mgt    LOGIN PASSWORD '${REGISTRY_DB_PASSWORD}';
  END IF;
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'api_svc') THEN
    CREATE ROLE api_svc        LOGIN PASSWORD '${API_DB_PASSWORD}';
  END IF;
END
\$\$;

SELECT 'CREATE DATABASE auth_db     OWNER auth_svc'
  WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'auth_db')\gexec
SELECT 'CREATE DATABASE market_db   OWNER prediction_svc'
  WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'market_db')\gexec
SELECT 'CREATE DATABASE registry_db OWNER service_mgt'
  WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'registry_db')\gexec
SQL

# --- Lock down CONNECT: Postgres grants CONNECT to PUBLIC by default, which would
# --- let any role reach any database. Revoke it, then grant only to the owner(s). ---
"${PSQL[@]}" --dbname "$POSTGRES_DB" <<SQL
REVOKE CONNECT ON DATABASE auth_db     FROM PUBLIC;
REVOKE CONNECT ON DATABASE market_db   FROM PUBLIC;
REVOKE CONNECT ON DATABASE registry_db FROM PUBLIC;
GRANT  CONNECT ON DATABASE auth_db     TO auth_svc;
GRANT  CONNECT ON DATABASE market_db   TO prediction_svc;
GRANT  CONNECT ON DATABASE registry_db TO service_mgt;
SQL

# --- auth_db: Flyway (auth-svc) owns the schema; just let auth_svc create in public. ---
"${PSQL[@]}" --dbname auth_db <<SQL
GRANT ALL ON SCHEMA public TO auth_svc;
SQL

# --- market_db: superuser installs the extension, then the owner role loads schema. ---
"${PSQL[@]}" --dbname market_db <<SQL
CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;
GRANT ALL ON SCHEMA public TO prediction_svc;
SQL
echo ">> [db-init] loading market_db schema as prediction_svc"
# Connect over the unix socket (trust auth during initdb; TCP is not up yet).
psql -v ON_ERROR_STOP=1 \
  --username prediction_svc --dbname market_db -f "$SCHEMA_DIR/10-market_db.sql"

# --- market_db grants for api_svc: read everything, write only what api-svc writes. ---
"${PSQL[@]}" --dbname market_db <<SQL
GRANT CONNECT ON DATABASE market_db TO api_svc;
GRANT USAGE  ON SCHEMA public TO api_svc;
GRANT SELECT ON ALL TABLES    IN SCHEMA public TO api_svc;
ALTER DEFAULT PRIVILEGES FOR ROLE prediction_svc IN SCHEMA public
  GRANT SELECT ON TABLES TO api_svc;
-- api-svc's only writes: PUT /api/schedules -> cron_schedules; sim config -> sim_bots.
GRANT INSERT, UPDATE, DELETE ON cron_schedules TO api_svc;
GRANT UPDATE                 ON sim_bots        TO api_svc;
SQL

# --- registry_db: owner role loads its (non-hypertable) schema. ---
"${PSQL[@]}" --dbname registry_db <<SQL
GRANT ALL ON SCHEMA public TO service_mgt;
SQL
echo ">> [db-init] loading registry_db schema as service_mgt"
psql -v ON_ERROR_STOP=1 \
  --username service_mgt --dbname registry_db -f "$SCHEMA_DIR/20-registry_db.sql"

echo ">> [db-init] done — auth_db / market_db / registry_db ready, per-service roles + api_svc read-model"
