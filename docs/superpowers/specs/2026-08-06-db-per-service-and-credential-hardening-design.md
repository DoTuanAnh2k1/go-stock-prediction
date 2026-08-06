# Design — Database-per-service + Credential Hardening

Date: 2026-08-06
Status: Approved (one-shot implementation)
Motivation: Capstone re-grade (92/100) deducted two points — (1) weak seeded/committed
credentials (`admin/123`, `admin123`, `POSTGRES_PASSWORD=123`, hardcoded super-admin), and
(2) a single shared PostgreSQL/TimescaleDB used by every service (data-ownership anti-pattern).

## Goals

1. **No weak or committed credentials.** Every credential is supplied at deploy time via
   env/Secret with fail-fast if missing. The seeded super-admin password is env-driven, never
   hardcoded. Remove dead `admin/admin123` config.
2. **Database-per-service.** Each backend owns its own PostgreSQL database with a dedicated
   least-privilege role. No service reaches into another service's data at the SQL layer.

### Delivered vs. the "fully DB-less api-svc" variant (important)

The approved intent was for `api-svc` to become **fully DB-less**, reading market/prediction data
via a new gRPC query API on `prediction-svc`. That query API is a port of ~115 read methods +
~20 RPCs + DTO builders from Go to Python — a large migration that cannot be completed **and
verified end-to-end** in a single pass without risking the passing build.

**Phase 1 (on `main`):** `api-svc` connects to `market_db` as a dedicated **read-only
least-privilege role `api_svc`** — `SELECT` + `INSERT/UPDATE/DELETE` on only `cron_schedules`
and `sim_bots`; no access to `auth_db`/`registry_db`. Zero api-svc code change; fully verified.

**Phase 2 (branch `phase2/api-svc-dbless`) — DELIVERED, mechanically verified:** `api-svc` is now
**fully DB-less** (`DB_DRIVER=grpc`, holds no Postgres credentials). Implemented via the repository
pattern: a new `pkg/store/grpcstore` implements the entire `DatabaseStore` interface by delegating
every read to prediction-svc over one generic RPC — `Query(method, params_json) → result_json` —
and unmarshalling `result_json` back into the SAME Go model types the handlers already use (so the
JSON keys are the Go json tags = DB columns, and **handlers/DTOs/JSON output are untouched** — the
api-svc handler tests, which mock `DatabaseStore`, still pass unchanged). prediction-svc dispatches
the 63 methods api-svc actually calls against `market_db` (`src/grpc_server/query_handlers.py`);
the ~54 write methods are compile-time stubs (api-svc never calls them). Serialization matches Go:
Decimal→string, datetime→RFC3339 `+07:00` (ICT). Verified: `go build`/`go vet`/`go test -short`
green, Python `py_compile`/ruff clean, helm+compose render clean. **Remaining before merge:** live
smoke-test of the dashboard (JSON field-level parity — decimal/time formats, exact query results —
can only be confirmed against a running stack).

Auth/RBAC data is reached via gRPC to `auth-svc` (pre-existing); user/RBAC tables never leave
`auth_db`.

## Non-goals

- Splitting the single TimescaleDB **instance** into multiple servers. Three isolated logical
  databases on one instance is the accepted "database-per-service" interpretation for this
  capstone; Postgres databases share no tables and cross-database SQL is impossible, so
  isolation is real and demonstrable.
- Changing the public HTTP JSON API contract. The web frontend must remain untouched.

## Target topology

| Database      | Owner role       | Tables |
|---------------|------------------|--------|
| `auth_db`     | `auth_svc`       | users, market_groups, user_market_groups, cli_handlers, commands, command_groups, command_group_commands, user_command_groups — created & owned 100% by auth-svc Flyway (V1–V3) |
| `market_db`   | `prediction_svc` | all `*_prices`, `*_intraday_prices`, `*_predictions`, `sim_*`, `pipeline_*`, `training_*`, `sync_logs`, `stock_fundamentals`, `stock_splits`, `cron_schedules`, `macro_indicators`, `pipeline_crawl_counters` |
| `registry_db` | `service_mgt`    | service_instances |
| *(none)*      | api-svc          | **owns no tables** — market via gRPC(prediction-svc), auth via gRPC(auth-svc) |

`cli-svc`, `gateway-svc`, `web-svc` never touch the DB (unchanged).

## Workstream 1 — Credential hardening

- **auth-svc `SuperAdminSeeder`**: inject `SUPER_ADMIN_USERNAME` / `SUPER_ADMIN_PASSWORD` via
  `@Value`; fail-fast (throw on startup) when password blank or equals a known-weak literal.
  Remove hardcoded `chon` / `Ch1nch2n@`. Never log the password.
- **api-svc**: delete the dead `AdminUsername` / `AdminPassword` config fields, their env
  defaults, and `ADMIN_USERNAME` / `ADMIN_PASSWORD` from ConfigMap/compose (grep-verified: only
  the config struct referenced them — nothing seeds a user from them).
- **Helm**: every `values.yaml` secret default becomes `""`; secret templates wrap values in
  `{{ required "<name> is required" .Values.secrets.<x> }}`. Real values via
  `-f values-secret.yaml` (already git-ignored, C2 guard exists). Charts: db, auth-svc, api-svc,
  prediction-svc, service-mgt, minio, pgadmin, cli-svc.
- **compose**: add `.env.example` with `CHANGE_ME` placeholders; drop `:-123` / `:-admin123`
  inline defaults so a missing `.env` fails loudly instead of booting with weak creds.
- **tests**: read admin creds from an env fixture instead of hardcoding `admin/admin123`.

## Workstream 2 — Database-per-service

### 2a. DB infrastructure
- Init script (`deploy/db-init/`) creates 3 roles + 3 databases and loads the correct schema
  into each. `database.sql` splits into `market_db.sql` (+ registry schema); auth tables leave
  `database.sql` entirely (Flyway owns `auth_db`) — this removes the GORM↔Flyway migration-order
  fragility the grader flagged.
- k8s: `db-schema` ConfigMap becomes the 3-file init set; compose mounts the same into
  `/docker-entrypoint-initdb.d`.

### 2b. Connection rewire
- auth-svc → `jdbc:postgresql://db:5432/auth_db` as `auth_svc`.
- prediction-svc (+ `jobs_cli`, cron) → `market_db` as `prediction_svc`.
- service-mgt → `registry_db` as `service_mgt`.
- api-svc → **remove `repository.Init()` / GORM / all DB env.**
- backup CronJob / ofelia → `pg_dump` each of the 3 databases.

### 2c. gRPC query API (bulk)
- Add resource-oriented read RPCs to `prediction.proto`, parameterised by market to collapse the
  gold/nasdaq/crypto/sp500 4× duplication. Approx surface:
  `GetMarketLatest`, `GetMarketPrices`, `GetMarketChart`, `GetPredictionsLatest`,
  `GetPredictionsList`, `GetPredictionsChart`, `GetPredictionsLatestResults`,
  `GetDirectionAccuracy`, `GetTrainingAlgorithms`, `GetTrainingMetrics`,
  `GetMarketsPredictions`, `GetMarketsTraining`, `GetMonitoringOverview`, `GetMonitoringBots`,
  `GetPipelineReports`, `GetDashboardStats`, `GetSimulationLeaderboard`, `GetSimulationBots`,
  `GetSimulationTrades`, `GetSimulationChart`, `GetSimulationConfig`, `ListSchedules`,
  `UpdateSchedule`. (`GetTrainingStatus` already exists.)
- prediction-svc Python servicer builds the **DTOs** (business logic lives with the data),
  reusing the existing prediction-svc repository that already reads these tables.
- Regenerate Go + Python stubs. api-svc maps proto responses to the **existing** JSON DTOs so the
  frontend contract is unchanged.

### 2d. api-svc rewire
- 23 handlers switch from `store`/`repository` to gRPC calls (market) — auth handlers already use
  gRPC. Delete `pkg/store/postgres/*` market impls and the now-unused `pkg/store/repository`
  market interfaces. api-svc keeps only gRPC clients + config + server.

### 2e. Helm / compose / docs
- db chart: 3 databases + 3 roles + 3 role Secrets; per-service configmap/secret point at their
  own DB+role; api-svc chart drops DB env. NetworkPolicy allow-to-db unchanged (same instance).
- Docs: CLAUDE.md, docs/claude/{database,directory-structure}.md, README data-ownership + debug.

## Risk mitigation / demonstrability
- JSON API contract frozen → frontend untouched; port resource-by-resource with old-vs-new parity
  checks.
- Demo isolation: `\l` shows 3 databases; `\du` shows 3 least-priv roles; connecting as `api_svc`
  (if any) or as one service role and querying another DB/table → permission denied; api-svc has
  no DB credentials at all.
- Verify: `go build ./...` (api-svc), `python -m py_compile` (prediction-svc), `helm lint` per
  chart, `helm template` renders clean.
