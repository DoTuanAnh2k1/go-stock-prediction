# Tài liệu Cài đặt (Implementation) — go-stock-prediction

> **Loại tài liệu:** Implementation Guide (tầng mã nguồn, cho đội phát triển)
> **Phạm vi:** Toàn hệ thống — 6 service chính + hạ tầng
> **Ngày:** 2026-07-31
> **Tài liệu song sinh:** [DESIGN.md](DESIGN.md) — thiết kế kiến trúc mà tài liệu này *hiện thực hóa*.

Tài liệu này mô tả **CÁCH CÀI ĐẶT** thiết kế trong [DESIGN.md](DESIGN.md) ở tầng mã nguồn: tech stack chi tiết, ánh xạ thành phần thiết kế → file thật, các pattern/entrypoint, cách build/chạy/test/triển khai. Đường dẫn file được ghi kèm để tra cứu.

---

## Mục lục

1. [Tech stack chi tiết](#1-tech-stack-chi-tiết)
2. [Ánh xạ thiết kế → mã nguồn](#2-ánh-xạ-thiết-kế--mã-nguồn)
3. [Cài đặt từng service](#3-cài-đặt-từng-service)
4. [Cài đặt tầng dữ liệu](#4-cài-đặt-tầng-dữ-liệu)
5. [Cài đặt ML pipeline](#5-cài-đặt-ml-pipeline)
6. [Cài đặt các vấn đề xuyên suốt](#6-cài-đặt-các-vấn-đề-xuyên-suốt)
7. [Build & chạy](#7-build--chạy)
8. [Triển khai Kubernetes](#8-triển-khai-kubernetes)
9. [Kiểm thử](#9-kiểm-thử)
10. [Vận hành](#10-vận-hành)

---

## 1. Tech stack chi tiết

### 1.1 Bảng phiên bản (từ file manifest thật)

| Service | Runtime | Thư viện chính (version) | Manifest |
|---|---|---|---|
| **gateway-svc** | Rust edition 2021 | axum 0.8.6, axum-server 0.7 (tls-rustls), tokio 1.48, hyper 1.7, reqwest 0.12.24 (rustls-tls/stream/http2), rustls 0.23 (ring), governor 0.6, tonic 0.12, kube 0.95 | `gateway-svc/Cargo.toml` |
| **api-svc** | Go 1.25.0 | `net/http` chuẩn, golang-jwt/jwt v5.3.1, gorm 1.31.1 + driver/postgres 1.6.0 (pgx v5.6.0), grpc 1.81.1, zerolog 1.35.1, shopspring/decimal 1.4.0, robfig/cron v3.0.1, swaggo/swag v2 rc5 | `api-svc/go.mod` |
| **auth-svc** | Java 21 | Spring Boot 3.3.6, grpc 1.63.0, grpc-server-spring-boot-starter 3.1.0, protobuf-java 3.25.3, java-jwt (auth0) 4.4.0, Flyway 10.x (+flyway-database-postgresql), Lombok | `auth-svc/pom.xml` |
| **prediction-svc** | Python ≥3.12 | grpcio ≥1.70, SQLAlchemy 2.0.40, psycopg2-binary, pydantic-settings 2.9.1, structlog 25.1.0, APScheduler 3.11.0, numpy ≥2.2.6, pandas ≥2.3.2, pandas-ta, scikit-learn 1.6.1 | `prediction-svc/pyproject.toml` |
| ↳ extras `[ml]` | | torch ≥2.5, statsmodels ≥0.14, arch ≥7.0, lightgbm ≥4.5, xgboost ≥2.0, optuna ≥3.6 | idem |
| ↳ extras `[observability]`/`[s3]` | | opentelemetry-*, prometheus-client, boto3 | idem |
| **cli-svc** | Go 1.26.4 | wish 1.4.7, bubbletea 1.3.10, bubbles 1.0.0, lipgloss 1.1.0, go-pretty/v6 6.8.1, termenv 0.16.0, prometheus 1.23.2 | `cli-svc/go.mod` |
| **service-mgt** | Go 1.25.0 | grpc 1.81.1, gorm 1.31.1, google/uuid 1.6.0, zerolog 1.35.1, glebarez/sqlite (test) | `service-mgt/go.mod` |
| **web-svc** | Node 22 build → nginx | react 18.3.1, react-router-dom 6.28, vite 6.0.1, typescript 5.6.2, jwt-decode 4.0, react-markdown 10.1, mermaid 11.16, terser 5.48 | `web-svc/package.json` |

### 1.2 Nguyên tắc chung khi cài đặt

- **Import ML lazy** (Python): `import torch` / `import lightgbm` nằm *trong method* → service khởi động không cần `[ml]`; thiếu lib thì thuật toán fallback/raise.
- **Fail-safe toàn cục:** registry/tracing/model-store bắt lỗi và no-op, không crash service.
- **Cùng image chạy 2 môi trường:** compose (không collector, in-app scheduler tắt) và k8s (có collector, CronJob) — chỉ khác ENV.

---

## 2. Ánh xạ thiết kế → mã nguồn

| Thành phần thiết kế (DESIGN.md) | Vị trí mã nguồn |
|---|---|
| Gateway routing + proxy | `gateway-svc/src/{router,routes,proxy,middleware}/` + `config.yaml` |
| API Backend entrypoint | `api-svc/cmd/main.go` |
| API routes | `api-svc/pkg/server/router.go` + `api_*.go` (mỗi file 1 nhóm endpoint) |
| Repository pattern (Go) | `api-svc/pkg/store/repository/` (interface) + `pkg/store/postgres/` (GORM impl) |
| gRPC clients (Go) | `api-svc/pkg/grpc/{client,authclient}/client.go` |
| Auth gRPC service | `auth-svc/.../grpc/AuthGrpcServiceImpl.java` + `service/*` |
| Flyway migrations | `auth-svc/src/main/resources/db/migration/V{1,2,3}*.sql` |
| Prediction gRPC servicer | `prediction-svc/src/grpc_server/server.py` |
| Repository (Python) | `prediction-svc/src/database/repository.py` |
| 13 thuật toán | `prediction-svc/src/algorithms/*.py` |
| Metadata registry (Go, hiển thị) | `api-svc/pkg/service/predict/registry/algorithms.go` |
| Simulation & bots | `prediction-svc/src/simulation/*.py` |
| Orchestrator (runner/train/split) | `prediction-svc/src/orchestrator/*.py` |
| Scheduler + jobs + CLI | `prediction-svc/src/scheduler/*.py` + `src/jobs_cli.py` |
| Crawlers | `prediction-svc/src/crawlers/*.py` |
| CLI TUI | `cli-svc/main.go` + `internal/{server,shell,handlers,client,render}/` |
| Service registry | `service-mgt/{main.go,internal/,client/}` |
| Web SPA | `web-svc/src/{context,pages,components,api}/` |
| Schema | `database.sql` (root) |
| Proto canonical | `api-svc/proto/{prediction,auth}/*.proto` |
| Deploy | `deploy/docker-compose.yaml`, `deploy/*.Dockerfile`, `deploy/helm/` |

---

## 3. Cài đặt từng service

### 3.1 gateway-svc (Rust)

**Entrypoint** `gateway-svc/src/main.rs`: cài `rustls::crypto::ring` (bắt buộc rustls 0.23+, panic nếu quên) → init telemetry/metrics → `AppConfig::load` (`config.yaml` rồi `/etc/gateway/config.yaml`, override env `GATEWAY_`) → khởi tạo blue/green → dựng `PathRouter` + `ProxyClient` → serve HTTP + HTTPS đồng thời (`tokio::join!`).

**Router** (`src/router/mod.rs`): `PathRouter::from_config` sort route giảm dần theo `prefix.len()`; `route(path)` = action đầu tiên `starts_with`. Block/None → 404.

**ProxyClient** (`src/proxy/client.rs`):
- Đọc trọn body (`to_bytes(body, usize::MAX)`) để cho phép retry.
- **SSE:** request `Accept: text/event-stream` → timeout 24h; response `Content-Type: text/event-stream` → `Body::from_stream(response.bytes_stream())`.
- Drop header `host/connection/content-length/transfer-encoding/traceparent/tracestate` khi forward; lỗi upstream → 502 JSON.

**Middleware** (`src/middleware/`, thứ tự trong `routes/mod.rs`): logging → request_id (UUID v4) → rate_limit (governor GCRA per-IP, 60/1s) → security_headers (chỉ route bật).

**Admin :9100** (`spawn_admin_server`): listener riêng, `/metrics` + `/healthz` — không có trong route table.

**TLS** (`docker-entrypoint.sh`): `openssl req -x509 -nodes -days 3650 -newkey rsa:2048` nếu chưa có cert; `chown 1001` + `chmod 600`; `setpriv` drop privilege.

### 3.2 api-svc (Go)

**Entrypoint** `api-svc/cmd/main.go` (thứ tự init xem DESIGN.md §5.2). Fail-fast: panic khi load timezone lỗi hoặc `repository.Init` lỗi.

**Router** `pkg/server/router.go`: `http.ServeMux` chuẩn (Go 1.22+ method+pattern). `addHandler()` đăng ký tập trung; dùng `r.Pattern` làm nhãn Prometheus (tránh nổ cardinality với `{id}`).

**Middleware** (`server.go`, ngoài→trong): `otelhttp.NewHandler` → `CORSMiddleware` → `JWTMiddleware` → `AccessLogMiddleware` → mux.
- `JWTMiddleware` (`middleware_jwt.go`): parse Bearer, verify HMAC bằng `cfg.JWTSecret`, inject `jwt.MapClaims`; **không token vẫn đi tiếp**.
- Gate: `AuthRequired`/`AdminRequired` (admin **và** super_admin)/`MarketRequired` (super_admin bypass, còn lại kiểm `accessible_markets`). `callerFromClaims` → `authpb.CallerMeta` forward RBAC.

**Handler nhóm** (`api_*.go`): auth (`api_auth.go` — `decodeGrantToken` dùng `strings.Cut` trên `:` đầu, map `Unauthenticated`→401), market data, predictions, triggers, users/RBAC, schedules, simulation/monitoring, `api_pipeline_stream.go`, `api_docs.go`, `api_backup.go`.

**gRPC clients** (`pkg/grpc/`): cả hai dùng `insecure` credentials + `telemetry.GRPCClientDialOption()` (nối trace). `authclient` → auth :8120; `grpcclient` → prediction :8119 (gồm 4 `Stream*Predict`).

**SSE** (`api_pipeline_stream.go`): auth `?token=`, set `text/event-stream` + `X-Accel-Buffering: no`, assert `http.Flusher`, vòng `stream.Recv()` → `data:` + `Flush()`. `statusRecorder.Flush()` (`middleware_accesslog.go`) forward Flusher.

**Docs viewer** (`api_docs.go`): `getDocsRoot()` từ `DOCS_ROOT` (default `/repo`); phòng thủ 3 lớp path-traversal (`..`, `.md`, containment sau `filepath.Clean`); 413 nếu >2MB.

**BackupScheduler** (`backup_scheduler.go`): chỉ chạy khi `BACKUP_SCHEDULER_ENABLED=true`; seed `daily_backup` (insert-if-not-exists), poll DB 60s reschedule; `runBackup` (pg_dump→gzip, giữ 10 file).

### 3.3 auth-svc (Java)

**Entrypoint** `AuthServiceApplication` (`@SpringBootApplication` + `@EnableScheduling`). App name `auth-service`.

**Phân tầng:** `AuthGrpcServiceImpl` (`@GrpcService`, delegate) → `service/{JwtService, UserService, MarketGroupService, CommandRbacService}` → `repository/*` (Spring Data JPA) → `entity/*`.

**JWT** (`JwtService`): `Algorithm.HMAC256(secret)`; claims `sub`, `username`, `role`, `user_id`, `accessible_markets`, `iat`, `exp` (= now + `jwt.expiration-ms` default 86400000).

**Authorize** (`UserService.requireAdminOrSuperAdmin`): mọi RPC quản trị đòi admin/super_admin; guard super_admin lặp lại (chỉ super_admin đụng super_admin; admin chỉ reset password `user`). `getAccessibleMarkets`: super_admin → all; còn lại UNION qua market group.

**Command RBAC** (`CommandRbacService`): `getUserCommands` — super_admin/admin bypass `findAllEnabledWithEnabledHandler()`; user = `findEnabledCommandsByUserId()` (native SQL DISTINCT JOIN). `upsertHandlers` so `req.secret` với env `INTERNAL_SECRET` (sai/rỗng → `PERMISSION_DENIED`).

**Seeder** `SuperAdminSeeder` (`ApplicationRunner`): seed `chon`/`Ch1nch2n@` role super_admin nếu chưa tồn tại (idempotent theo username+role+deleted_at IS NULL).

**Config** (`application.yml`): `server.port=${MANAGEMENT_PORT:9464}` (Actuator/Tomcat), gRPC `:8120`; `hibernate.jdbc.time_zone: Asia/Ho_Chi_Minh`; `spring.output.ansi.enabled: always`; JPA `ddl-auto: validate`, Flyway `baseline-on-migrate: true`.

### 3.4 prediction-svc (Python)

**Entrypoint** `src/main.py` (8 bước, xem DESIGN.md §5.4). TZ set trước import (`os.environ.setdefault("TZ",...)` + `time.tzset()`).

**gRPC servicer** (`src/grpc_server/server.py`): `PredictionServicer`; predict/crawl nền qua `threading.Thread(daemon=True)`; backtest guard `_backtest_running` (Event); ThreadPool `max_workers=10`. Interceptor: OTel + `_MetricsInterceptor` (`record_rpc`).

**DB** (`src/database/connection.py`): `postgresql+psycopg2`, `pool_pre_ping=True`, `pool_size=20`, `max_overflow=40`, `pool_recycle=3600`, `connect_args={"options":"-c TimeZone=Asia/Ho_Chi_Minh"}`. `session_scope()` contextmanager auto commit/rollback/close.

**Scheduler** (`src/scheduler/manager.py`): `DEFAULT_SCHEDULES` (cron 6-field Go format; `parse_6field_cron` strip giây + map weekday Go→APScheduler). Watcher `CronTrigger(minute="*")` poll DB 60s. **Chỉ enabled=True:** train_gold/nasdaq/crypto/sp500/meta, crawler_fundamentals, train_transformer — các pipeline crawler enabled=False ("moved to k8s CronJob").

**jobs_cli** (`src/jobs_cli.py`): `python -m src.jobs_cli <job_key|gold|nasdaq|crypto|sp500>`; `--list`; init tối thiểu (config→TZ→logger→`init_db()`, không gRPC/scheduler); exit 0/1/2. Tái dùng `JOB_FUNCTIONS` (18 job).

**Orchestrator:**
- `runner.py` `run_for_market(key)`: lấy dir_acc → `_apply_ensemble_weights`; `transformer_nn` dùng `_transformer_intraday_prices` (hourly ≥110 bar); ghi prediction; `_trigger_sim_step`. Per-symbol pass `ThreadPoolExecutor` gated `per_symbol_enabled`.
- `training.py` `train_for_market`: `_collect_series_for_market` → `build_algorithms` → `algo.train_batch_labeled`; ghi `training_logs`/`sync_logs`; trạng thái global dưới `_lock`. `reconcile_predictions(only_market)` chấm khi `target_date <= now`, `actual=prices[-1]`; `direction_verdict` → None khi frozen.
- `splits.py` `_apply_one_split`: chỉ chỉnh intraday, boundary phát hiện từ data (`prev/cur >= ratio*0.75`), idempotent (`applied_at`).

**Crawlers** (`src/crawlers/`): `base.py` (abstract), `gold.py` (Yahoo XAU + BTMC + Phú Quý), `nasdaq.py`/`sp500.py` (Yahoo + `events=split`), `crypto.py` (**Binance klines**, `_COIN_TO_BINANCE`), `fundamentals.py` (yfinance), `sanity.py` (`check_update` + `batch_outlier_mask`).

**market_calendar.py:** `_nyse_holidays` tính động; `is_market_open` (guard ngày) + `is_intraday_open` (guard giờ, hẹp hơn). CRYPTO luôn True, GOLD weekday, NYSE có giờ phiên.

### 3.5 cli-svc (Go)

**Entrypoint** `cli-svc/main.go`: OTel tracing + metrics :9464 → `server.LoadConfig` → `server.New` → `ListenAndServe`.

**SSH server** (`internal/server/server.go`): `wish.NewServer` (WithHostKeyPath tự tạo, WithPasswordAuth, WithMiddleware bubbletea). `passwordHandler` → `client.Login` (15s) → lưu ctxKey{JWT,Role,UserID}. `upsertCatalog()` goroutine (skip nếu secret rỗng, retry 10× cách 3s, chỉ 2xx = OK).

**Client** (`internal/client/client.go`): interface `HTTPClient{Login, GetMyCommands, Do, PostInternal}`; body/response giới hạn 8MB; timeout 30s; `otelhttp` transport. `decodeJWTClaims` (`jwt.go`) **không verify chữ ký**.

**Shell** (`internal/shell/`):
- `run.go` `Run()`: `Parse` → `reg.Resolve` → `allowed.Allows` → `h.Validate` → `h.Execute` → render. Enforcement TRƯỚC khi gọi API.
- `enforce.go` `NewAllowedSet(role, cmds)`: super_admin bypass; else union từ `GET /me/commands`; rỗng → từ chối.
- `model.go`/`update.go`/`view.go`: bubbletea; `loadCommands()` async; inline mode (`tea.Println`); completer kube-prompt (dropdown sau Tab).
- `parse.go`/`grep.go`: grammar `<verb> <category> <name> [arg val]`; pipe grep hỗ trợ `-i/-A/-B/-C`.

**Handler catalog** (`internal/handlers/defaults.go`): 19 handler; verb map get/set/update/delete → GET/POST/PUT/DELETE.

### 3.6 service-mgt (Go)

**Entrypoint** `service-mgt/main.go`: TZ ICT → zerolog → `config.Load` → telemetry + metrics :9464 → `gorm.Open` → `store.NewGorm` → `registry.New` → `WarmStart` → `StartBackground` → listen gRPC :8121 + `grpc_health_v1.Health`.

**Registry core** (`internal/registry/registry.go`): struct `Core` với `byName`/`byID` index + `sync.RWMutex`. `Register` write-through DB trước; `Heartbeat` gia hạn **chỉ cache** (write-through chỉ khi hồi sinh DOWN→UP); `Tick(now)` reaper (quá expireAt → DOWN; quá +grace → Delete); `flush()` gom `dirtySeen` 1 transaction; `WarmStart` đặt `expireAt=now`. `StartBackground`: 2 ticker (reaper 1s, flusher 30s).

**Store** (`internal/store/`): interface `{Upsert, UpdateStatus, Delete, FlushLastSeen, LoadAll}`; `Upsert` = GORM `OnConflict{Columns:instance_id}`.

**Client SDK** (`client/client.go`): `Start()` dial non-blocking + `register` best-effort + `loop()`; `Resolve(name, staticFallback)` cache-first (5s), lỗi → static fallback; `Stop()` deregister. TTL/grace hard-coded trong `config.go` (không đọc env).

### 3.7 web-svc (React)

**Entrypoint** `src/main.tsx`: `BrowserRouter > DataProvider > App`; `App` bọc `AuthProvider > LangProvider > AppInner`.

**Context:**
- `AuthContext.tsx`: `login()` → `POST /api/x/grant` (X-Token `btoa("user:pass")`); JWT trong localStorage `vns_token`; `jwt-decode` → `accessible_markets`; `canAccessMarket()`.
- `DataContext.tsx`: `loadAll()` khi mount, `status: loading|live|demo`, `enrich()` async.
- `LangContext.tsx` + `i18n.ts`: VI/EN cây phẳng.

**API client** (`src/api/index.ts`): `fetch` thuần; `fetchJSON` tự gắn `Authorization: Bearer`; base `const BASE = '/api'` (phụ thuộc gateway). Algo metadata enrich runtime từ `/api/training/algorithms`.

**Charts** (`src/components/charts.tsx`): SVG thủ công (Sparkline/LineChart/BarChart/Candlestick/Donut...), viewBox scale, palette OKLCH.

**Docs** (`src/pages/Docs.tsx`): TOC từ regex heading, IntersectionObserver scroll-spy; `MermaidBlock` (lazy `import('mermaid')`, pan/zoom) + `VizBlock`.

**SSE** (`Settings.tsx`): `new EventSource('/api/pipeline/stream?market=...&token=JWT')`.

**Build** (`vite.config.ts`): dev proxy `/api → :31300`; prod terser `drop_console` + mangle.

---

## 4. Cài đặt tầng dữ liệu

### 4.1 Schema (`database.sql`)

- **Composite PK hypertable:** DROP PK `id` gốc → `ADD PRIMARY KEY (id, <time_col>)` cho 13 bảng → `create_hypertable`. Đây là lý do tắt AutoMigrate GORM.
- **Cột additive idempotent** (`ADD COLUMN IF NOT EXISTS`): `snapshot_at`, `trade_at`, `trailing_stop`, OHLC crypto/gold, `symbol` — an toàn chạy lại (schema chỉ mount lần init đầu).
- **Unique index tách khỏi DML:** `CREATE UNIQUE INDEX uq_sim_snap_session_at (session_id, snapshot_at, snapshot_date)` phải ở block riêng (Timescale rollback nếu gộp với DELETE/UPDATE).
- **Compression + continuous aggregate + materialized view** (xem DESIGN.md §6.4).

### 4.2 ORM mirror

- **GORM (Go)** `api-svc/pkg/models/models_db/*.go`: struct mirror schema; 4 prediction struct có `DirectionCorrect *bool`; `User` struct **cố tình thiếu** profile fields (Go thin proxy). Volume crypto = `Volume24h` (cột `volume24h`).
- **SQLAlchemy (Python)** `prediction-svc/src/database/models.py`: mirror; khai tên cột tường minh khi không snake_case chuẩn (`Column("volume24h", ...)`).
- **Repository access:** Go qua `DatabaseStore` (`GetSingleton()`); Python qua tập hàm `repository.py` (`session_scope()`). Không gọi ORM trực tiếp từ service layer.

### 4.3 Flyway (auth-svc, RBAC)

| Migration | Nội dung |
|---|---|
| V1 | `market_groups`, `market_group_markets` PK `(group_id, market_key)`, `user_market_groups` PK `(user_id, group_id)` (FK CASCADE, không FK sang `users`) |
| V2 | `ALTER users ADD full_name/email/phone` (nullable) |
| V3 | `cli_handlers` (PK `handler_key`, `arg_schema` JSONB), `commands` (FK→cli_handlers, `args` JSONB, `name` unique), `command_groups`, `command_group_commands`, `user_command_groups` |

---

## 5. Cài đặt ML pipeline

### 5.1 Thêm thuật toán mới (quy trình chuẩn)

1. Tạo `prediction-svc/src/algorithms/<tên>.py` — implement `PredictionAlgorithm`, bắt buộc dùng `get_max_change_pct(self._market_key)` để clamp.
2. Đăng ký trong `registry.py` → `build_algorithms()` (two-pass: base trước, Ensemble cuối).
3. Thêm metadata vào `api-svc/pkg/service/predict/registry/algorithms.go` → `init()` (chỉ hiển thị, không chạy predict).
4. (Tùy chọn) Thêm vào Ensemble `ensemble.py` (hiện 10 base; `rl_dqn`/`transformer_nn` KHÔNG vào).

### 5.2 Feature builder (`features.py`)

`build_basic_features()` (14) / `build_enhanced_features()` (30): lag returns, MA ratio, RSI, StochRSI, BB %B, MACD, rolling std, ROC, momentum, vol ratio. Pandas-ta nếu có, numpy fallback (`_numpy_rsi`, `_numpy_macd`). Target = next-return log; hàng cuối = inference row (drop `[:-1]` khi train). `MIN_DATA_POINTS=80`.

### 5.3 transformer_nn (`transformer_model.py`)

- Kiến trúc: SEQ_LEN=96 log-returns → 12 patch (PATCH_LEN=8) → embed D_MODEL=64 + pos emb → TransformerEncoder 2 layer pre-norm (N_HEADS=4, FF=128) → mean-pool → concat fundamentals (FUND_DIM=11→16) → dual head (`head_ret` regression + `head_dir` P(up)). Loss = Huber + 0.3·BCE.
- **Direction label = `ret > 0`** (`_direction_targets`): `y_std > -r_mean/r_std` — fix bug bias.
- Fundamentals: 11 fields (`FUNDAMENTAL_FIELDS`, repository.py:1444); thiếu → impute 0. Symbol từ `_context_symbol`/`_symbol_key`.
- `skip_write` khi `|p_up-0.5| < 0.02`. Cold-start → quick-train 15 epoch (không lưu). Legacy single-head checkpoint vẫn load.

### 5.4 rl_dqn (`rl_dqn.py`)

- `_DuelingQNet`: LayerNorm → 128 → 64 → V(s)+A(s,a)−mean(A). Actions 3 (hold/buy/sell). Observation = 30 enhanced features (scaled) + 3 position features.
- Training: PER (α=0.6,β=0.4) + 3-step return + Double-DQN + Polyak (τ=0.005) + Huber. Reward vol-normalized + directional shaping (±0.15·dir·ret/σ), clip ±10. Walk-forward validation (giữ best-epoch theo greedy RAW reward). `_make_windows` tail-align (fix reward lệch 20 bước). `_scale_feature_matrix` (tag `feat_scale`).
- Legacy plain MLP checkpoint vẫn load (tag `arch`).

### 5.5 Optuna (lgbm/xgb)

Chỉ khi `n_data_points ≥ 200`, `len(X_val) ≥ 5`, `is_optuna_enabled()` (`runtime_flags.py`). Max 30 trials, timeout 120s, minimize MAE. RandomForest không dùng.

### 5.6 Simulation (`simulation/`)

- `bot.py` `TradingBot.step()`: SL/TP trước → rẽ nhánh `base_key.split("__")[0]` → `_step_{threshold,rl,meta,conviction}`. Hằng số: `_RL_BUY_CONF_FLOOR=0.38`, `_CONVICTION_BUY_FLOOR=0.52`/`_SELL_CEIL=0.48`. Market normalize `_SIM_TO_REGISTRY_MARKET` (NASDAQ↔NASDAQ100).
- `meta_stack.py` `MetaStackModel`: `FEATURE_DIM=26` (`[g_1..12 | w_1..12 | sigma | mom]`); LightGBM `class_weight=balanced` + walk-forward 70/10/20 + sigmoid Platt calibration; `_fallback_vote` reliability-weighted; checkpoint pickle qua `model_store`.
- `portfolio.py` `buy(position_pct=...)` cho sizing conviction; trailing peak stateless (`MAX(price)` intraday).
- `seeder.py`: 4 khối (Standard `_v1–_v10`, Trailing `_v11`/`_v12` pooled-only, RL 1/market, Meta 1/market); per-symbol gated `PER_SYMBOL_ENABLED`.
- `engine.py`: `StepDataCache` (fetch predictions/prices 1 lần share ThreadPool); `_restore_portfolio_state` split-aware; `_upsert_snapshot` on-conflict theo giờ; worker `min(8,...)` backtest, `min(16,...)` live-step.

---

## 6. Cài đặt các vấn đề xuyên suốt

### 6.1 Proto codegen

```bash
# prediction.proto → Go stubs (từ api-svc/)
cd api-svc && protoc --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  proto/prediction/prediction.proto

# prediction.proto → Python stubs (trong Dockerfile stage proto-builder)
python -m grpc_tools.protoc -Iapi-svc/proto \
  --python_out=prediction-svc/src/proto --grpc_python_out=prediction-svc/src/proto \
  api-svc/proto/prediction/prediction.proto

# auth.proto: sửa CANONICAL tại api-svc/proto/auth/auth.proto, rồi:
make proto-auth        # sync bản Java (cp byte-identical) + regen Go stubs
make proto-auth-check  # CI guard: fail nếu 2 bản drift
```

`prediction.proto` = **24 RPC** (4 server-streaming `Stream*Predict` → `PipelineLogEvent{level,msg,progress,done,error}`). `auth.proto` = **35 RPC** (mọi request mang `CallerMeta{caller_id, caller_role}`).

### 6.2 Telemetry (tracing)

- **api-svc** (`pkg/telemetry/tracing.go`): `InitTracing("api-svc")` tại `main.go`; luôn cài W3C propagator; `otlptracegrpc` WithInsecure; no-op khi `OTEL_EXPORTER_OTLP_ENDPOINT` rỗng. `GRPCClientDialOption()` = `otelgrpc.NewClientHandler` cho 2 client.
- **prediction-svc** (`src/telemetry/tracing.py`): `init_tracing()` tại `main.py:62`; `get_grpc_server_interceptor()` extract W3C từ metadata → child span.
- **auth-svc:** OTel Java agent `-javaagent:` qua `JAVA_TOOL_OPTIONS` (không code SDK).
- **Endpoint mặc định (k8s):** `http://otel-collector.observability:4317`.

### 6.3 Metrics

| Service | Endpoint | Metric mẫu |
|---|---|---|
| api-svc | `:8118/metrics` | `http_requests_total{method,path,status}` (label = route pattern) |
| gateway-svc | `:9100/metrics` | `gateway_requests_total`, `gateway_request_duration_seconds` |
| prediction-svc | `:9464/metrics` | `prediction_grpc_requests_total{method}`, `_latency_seconds` |
| auth-svc | `:9464/actuator/prometheus` | Micrometer |
| service-mgt / cli-svc | `:9464/metrics` | default collectors |

Prometheus scrape qua pod annotation `prometheus.io/scrape|port` (trỏ THẲNG app, không qua nginx).

### 6.4 Logging

api-svc zerolog (`AccessLogMiddleware`, `statusRecorder.Flush()`); prediction-svc structlog `ConsoleRenderer(colors=True)`; auth-svc logback `%clr` + `GrpcLoggingInterceptor`; gateway tracing compact+ansi. Định dạng qua `LOG_FORMAT` (console/json).

### 6.5 Model Store (`storage/model_store.py`)

`ensure_local(path)` (s3: download nếu cache miss) / `upload_if_remote(path)` (local: no-op) / `save_bytes`/`load_bytes`; singleton `get_store()`; backend `MODEL_STORE_BACKEND` (local|s3); `_s3_key()` strip prefix `RL_MODEL_DIR`. Fail-safe: lỗi S3 → log + dùng local. Wire vào `rl_dqn`, `transformer_model`, `meta_stack`.

---

## 7. Build & chạy

### 7.1 Compose (local)

```bash
# Toàn bộ stack
docker compose --env-file .env -f deploy/docker-compose.yaml up -d
# hoặc
make up            # make down / make reset / make rebuild SVC=<svc> / make restart SVC=<svc>

# Build riêng api-svc (Go)
cd api-svc && go build -o api-server ./cmd

# Version stamping (bake vào image)
#   GIT_SHA=git rev-parse --short HEAD, BUILD_TIME, GIT_DIRTY → build args
make versions      # curl /api/version qua gateway (fallback grep log)

# Theo dõi CPU/RAM
make stats         # snapshot   |   make stats-watch (refresh 3s)
```

Schema tự init từ `database.sql` khi container `db` lần đầu lên. Import thủ công:
```bash
psql -U postgres -d go_stock_prediction -f database.sql
```

### 7.2 Regenerate Swagger

```bash
cd api-svc && swag init -g cmd/main.go -o docs/ --parseInternal --parseDependency
# BẮT BUỘC 2 flag (swag resolve type proto authpb.UserResponse); không sửa tay api-svc/docs/
```

### 7.3 Truy cập nhanh

```bash
# Login lấy JWT
TOKEN=$(curl -s -X POST http://localhost/api/x/grant \
  -H "Content-Type: application/json" \
  -H "X-Token: $(printf '%s' 'admin:admin123' | base64)" \
  -d '{"request":""}' | jq -r '.token')

curl -X POST http://localhost/api/trigger/gold-crawler -H "Authorization: Bearer $TOKEN"
```

- Web: `http://localhost/` (qua gateway) — super_admin `chon`/`Ch1nch2n@`.
- Swagger: `http://localhost:8118/swagger/`.
- CLI: `ssh -p 2345 <user>@localhost`.

---

## 8. Triển khai Kubernetes

### 8.1 Lần đầu

```bash
kubectl create namespace stock
# db-schema ConfigMap KHÔNG do Helm quản lý — tạo thủ công TRƯỚC
kubectl create configmap db-schema -n stock --from-file=01-schema.sql=database.sql

helm install stock deploy/helm/stock -n stock
helm install observability deploy/helm/observability -n observability --create-namespace
```

### 8.2 Cập nhật

```bash
helm upgrade stock deploy/helm/stock -n stock --set global.imageTag=v2
helm upgrade observability deploy/helm/observability -n observability
```

### 8.3 Toggle & override

| Mục đích | Cờ |
|---|---|
| Blue/green (mặc định **true**) | `--set global.bluegreen.enabled=false` |
| pgAdmin | `--set global.pgadmin.enabled=false` |
| Manual Job | `--set global.manualJob.enabled=true` |
| PDB / HPA / Quota / RBAC / Ingress / NetworkPolicy | `--set global.{pdb,hpa,quota,rbac,ingress,networkPolicy}.enabled=...` |
| Image tag chung | `--set global.imageTag=v2` |
| Replicas per-subchart | `--set <svc>.replicas=N` (vd `web-svc.replicas=4`) |

### 8.4 Ambassador & CronJob

- Ambassador pattern (5 backend) inline trong `charts/<svc>/templates/deployment.yaml`, dùng named template từ library `common` (`_ambassador.tpl`). Verify không đổi hành vi: `helm template stock deploy/helm/stock` diff với baseline (và `--set global.bluegreen.enabled=false`).
- CronJob (`charts/cronjobs/`) chạy `python -m src.jobs_cli <job_key>` (mirror `DEFAULT_SCHEDULES`); `daily_backup` = `cronjob-backup.yaml` (pg_dump → PVC `backup-data`).
- **prediction-svc replicas=2** an toàn vì `MODEL_STORE_BACKEND=s3` (`/models` = emptyDir cache, checkpoint ở MinIO).
- **`dnsConfig ndots:1`** ở prediction-svc + mọi CronJob (fix ISP hijack NXDOMAIN → crawl saved=0).

### 8.5 Gotcha triển khai

- **bluegreen bắt buộc true:** gateway route cứng `/api → api-svc-<color>` → `false` gây 502.
- **PDB minAvailable:1 trên db/minio** (1 replica) chặn drain node → scale=0 hoặc `kubectl drain --force`.
- **gRPC health probe = tcpSocket** (Python không đăng ký `grpc.health.v1`); `startupProbe failureThreshold:30 period 5s` (~150s cho torch import).

---

## 9. Kiểm thử

### 9.1 Go

```bash
cd api-svc && go test ./...
cd api-svc && make test-unit
cd api-svc && make test-coverage   # coverage.html
cd api-svc && make test-db-up       # PostgreSQL port 5433
cd api-svc && make test-db-down
```

Convention: test cùng package (`algo.go`→`algo_test.go`); integration `if testing.Short() { t.Skip() }`. service-mgt test dùng `glebarez/sqlite` (in-memory, no cgo) + `fakeStore` kiểm vòng đời lease.

### 9.2 Python

```bash
cd prediction-svc && make test
docker exec prediction-svc python -m pytest tests/ -v
```

Convention: `tests/unit/test_<module>.py` (không DB); `tests/integration/test_<phase>.py` (cần Docker stack); `pytest.importorskip("torch")` cho optional deps.

---

## 10. Vận hành

### 10.1 Biến môi trường chính (`.env`)

| Biến | Mặc định | Ghi chú |
|---|---|---|
| `SERVER_PORT` / `GRPC_SERVER_PORT` | 8118 / 8119 | api HTTP / prediction gRPC |
| `GRPC_TARGET` / `AUTH_GRPC_TARGET` | `prediction-svc:8119` / `auth-svc:8120` | target gRPC |
| `JWT_SECRET` | change-me | **phải giống** giữa api-svc và auth-svc |
| `INTERNAL_SECRET` | change-me | upsert catalog (**trong body**, không header) |
| `SCHEDULER_ENABLED` / `BACKUP_SCHEDULER_ENABLED` | false | tắt in-app scheduler (CronJob/ofelia chạy thay) |
| `SERVICE_MGT_ENABLED` | false | registry; static fallback khi tắt |
| `MODEL_STORE_BACKEND` | local \| s3 | s3 = MinIO bucket `models` (k8s) |
| `PER_SYMBOL_ENABLED` | false | bật bot/model per-symbol |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | (rỗng compose) | rỗng → tracing tắt |
| `CRAWL_MAX_TICK_DEVIATION_<MARKET>` | GOLD 0.30 / NASDAQ·SP500 0.40 / CRYPTO 0.80 | sanity guard |
| `LOG_LEVEL` / `LOG_FORMAT` | INFO / console | console=ANSI, json=aggregator |
| `DOCS_ROOT` | /repo | docs viewer (bind-mount `../:/repo:ro`) |

### 10.2 Ports

| Service | Port | Giao thức | Metrics |
|---|---|---|---|
| Gateway | 80/443 | HTTP/HTTPS | :9100 |
| API Backend | 8118 | HTTP | :8118/metrics + Swagger |
| Prediction | 8119 | gRPC | :9464 |
| Auth | 8120 | gRPC | :9464/actuator/prometheus |
| Service-mgt | 8121 | gRPC | :9464 |
| TimescaleDB | 5432 | TCP | — |
| pgAdmin | 8081 | HTTP (127.0.0.1) | — |
| Frontend | 3000 | HTTP | — |
| CLI Service | 2345 | SSH | :9464 |
| MinIO (k8s) | 9000/9001 | HTTP | — |
| OTel Collector / Tempo / Prometheus / Grafana (k8s) | 4317 / 3200 / 9090 / 3000 | — | — |

### 10.3 Trigger thủ công (admin JWT)

```bash
curl -X POST http://localhost/api/trigger/{gold,nasdaq,crypto,sp500}-crawler -H "Authorization: Bearer $TOKEN"
curl -X POST http://localhost/api/trigger/reconcile -H "Authorization: Bearer $TOKEN"
curl -X POST http://localhost/api/trigger/train -H "Authorization: Bearer $TOKEN" -d '{"algorithm":"meta_stack"}'
curl -X POST "http://localhost/api/trigger/historical-backtest?train_window=30&step_size=6&market_key=ALL" -H "Authorization: Bearer $TOKEN"

# Cập nhật lịch cron (khi APScheduler in-app bật)
curl -X PUT http://localhost/api/schedules/crawler_gold \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"cron_expression":"0 0 2 * * *","enabled":true}'
```

### 10.4 Xử lý sự cố thường gặp

| Triệu chứng | Nguyên nhân gốc | Xử lý |
|---|---|---|
| Mọi request 401 dù có token | `JWT_SECRET` lệch giữa api-svc và auth-svc | Đồng bộ `JWT_SECRET` |
| SSE `/api/pipeline/stream` treo/vỡ | Thiếu `Flush()` xuyên middleware hoặc gateway buffer | Kiểm `statusRecorder.Flush()` + `proxy_buffering off` |
| Crawl k8s saved=0 / timeout | ISP hijack NXDOMAIN với ndots:5 | `dnsConfig ndots:1` |
| Direction accuracy cả lô = 0% | Chấm khi `actual==current` (frozen) | Đã có `direction_verdict`→None (giữ pending) |
| Split thật bị coi là tick rác | Guard "chặn nhảy giá" chặn nhầm | Phân biệt bằng persistence (`check_update` 2 nhịp) |
| gateway 502 sau helm upgrade | `global.bluegreen.enabled=false` | Bật lại true (route cứng theo màu) |
| Leaderboard hiện 0 dù có trades | `kpi_updated_at NULL` trong `sim_sessions` | Backfill `update_session_kpis` |
| pod backend không drain được | PDB minAvailable:1 trên 1-replica db/minio | scale=0 hoặc `drain --force` |

---

*Tài liệu này bám mã nguồn tại 2026-07-31. Thiết kế kiến trúc tổng thể xem [DESIGN.md](DESIGN.md).*
