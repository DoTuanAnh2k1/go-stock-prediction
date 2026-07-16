# Cấu trúc thư mục chi tiết

## Go API Backend

```
api-svc/cmd/main.go                         # API Backend entry point — config → timezone → logger → repository → grpcclient → backup_scheduler → server
api-svc/pkg/config/                         # Load .env, trả về config struct toàn cục (bao gồm GRPCConfig, BackupSchedulerEnabled)
api-svc/pkg/telemetry/                      # OTel tracing (tracing.go), Prometheus metrics (metrics.go), sampler (sampler.go) — factor 14
api-svc/pkg/grpc/client/client.go           # gRPC client singleton — API Backend dùng để gọi Python Prediction Service
api-svc/pkg/grpc/authclient/client.go       # gRPC client singleton — API Backend dùng để gọi Java Auth Service
api-svc/proto/prediction/prediction.proto   # gRPC service definitions (shared với Python service)
api-svc/proto/prediction/*.pb.go            # Generated Go protobuf stubs (không sửa tay)
api-svc/proto/auth/auth.proto               # gRPC service definitions cho Auth Service (18 RPCs)
api-svc/proto/auth/*.pb.go                  # Generated Go protobuf stubs cho auth (không sửa tay)
api-svc/docs/swagger.json                   # Generated Swagger spec (không sửa tay — chạy swag init)
deploy/api-svc.Dockerfile                   # Docker build cho API Backend (Go)
api-svc/pkg/server/router.go                # Đăng ký tất cả routes (API)
api-svc/pkg/server/api_*.go                 # Mỗi file = một nhóm API endpoint
api-svc/pkg/server/api_direction_accuracy.go    # GET /api/predictions/direction-accuracy
api-svc/pkg/server/api_training_algorithms.go   # GET /api/training/algorithms
api-svc/pkg/server/api_training_metrics.go      # GET /api/training/metrics
api-svc/pkg/server/api_gold.go                  # GET /api/gold/latest, /api/gold/prices, /api/gold/chart
api-svc/pkg/server/api_gold_prediction.go       # GET /api/gold/predictions/*
api-svc/pkg/server/api_nasdaq.go                # GET /api/nasdaq/latest, /api/nasdaq/prices, /api/nasdaq/chart
api-svc/pkg/server/api_nasdaq_prediction.go     # GET /api/nasdaq/predictions/*
api-svc/pkg/server/api_crypto.go                # GET /api/crypto/latest, /api/crypto/prices, /api/crypto/chart
api-svc/pkg/server/api_crypto_prediction.go     # GET /api/crypto/predictions/*
api-svc/pkg/server/api_sp500.go                 # GET /api/sp500/latest, /api/sp500/prices, /api/sp500/chart
api-svc/pkg/server/api_sp500_prediction.go      # GET /api/sp500/predictions/*
api-svc/pkg/server/api_markets.go               # GET /api/markets/{key}/predictions, GET /api/markets/{key}/training
api-svc/pkg/server/api_training_status.go       # GET /api/training/status (gọi gRPC GetTrainingStatus)
api-svc/pkg/server/api_dashboard_stats.go       # GET /api/dashboard/stats
api-svc/pkg/server/api_trigger_train.go         # POST /api/trigger/train (→ gRPC TriggerTrain)
api-svc/pkg/server/api_trigger_gold_crawler.go  # POST /api/trigger/gold-crawler
api-svc/pkg/server/api_trigger_gold_history.go  # POST /api/trigger/gold-history, POST /api/trigger/gold-predict
api-svc/pkg/server/api_trigger_reconcile.go     # POST /api/trigger/reconcile
api-svc/pkg/server/api_trigger_historical_backtest.go # POST /api/trigger/historical-backtest (202, 409 nếu đang chạy)
api-svc/pkg/server/api_trigger_gold_historical_backtest.go # POST /api/trigger/gold-historical-backtest
api-svc/pkg/server/api_trigger_nasdaq_crawler.go  # POST /api/trigger/nasdaq-crawler, POST /api/trigger/nasdaq-predict
api-svc/pkg/server/api_trigger_crypto_crawler.go  # POST /api/trigger/crypto-crawler, POST /api/trigger/crypto-predict
api-svc/pkg/server/api_trigger_crypto_history.go  # POST /api/trigger/crypto-history (backfill 180 ngày, background, 202)
api-svc/pkg/server/api_trigger_sp500.go           # POST /api/trigger/sp500-crawler, POST /api/trigger/sp500-predict
api-svc/pkg/server/api_trigger_simulation.go      # POST /api/trigger/simulation-backtest, simulation-live-step, sim-reset
api-svc/pkg/server/api_auth.go                    # POST /api/x/grant (login, X-Token header), GET /api/auth/me, PUT /api/auth/password
api-svc/pkg/server/api_users.go                   # CRUD /api/users — quản lý user (admin only)
api-svc/pkg/server/api_market_groups.go           # CRUD /api/market-groups (admin only)
api-svc/pkg/server/api_schedules.go               # GET /api/schedules, PUT /api/schedules/{key}
api-svc/pkg/server/api_backup.go                  # GET/POST/DELETE /api/backups; runBackup(ctx) dùng chung
api-svc/pkg/server/backup_scheduler.go            # StartBackupScheduler(store) — poll DB 60s, seed daily_backup; guard BACKUP_SCHEDULER_ENABLED
api-svc/pkg/server/api_simulation.go              # GET /api/simulation/leaderboard, bots, trades, chart; PUT config; POST toggle/run
api-svc/pkg/server/api_monitoring.go              # GET /api/monitoring/overview, /api/monitoring/bots
api-svc/pkg/server/api_pipeline_reports.go        # GET /api/pipeline-reports
api-svc/pkg/server/api_pipeline_stream.go         # GET /api/pipeline/stream — SSE stream progress predict; auth ?token= query param; proxy gRPC Stream{Gold,Nasdaq,Crypto,SP500}Predict
api-svc/pkg/server/api_docs.go                    # GET /api/docs (list *.md), GET /api/docs/raw?path= (content) — serve markdown cho tab Tài liệu; admin only; đọc từ DOCS_ROOT; chống path traversal
api-svc/pkg/server/middleware_jwt.go              # JWTMiddleware (non-blocking), AuthRequired(), MarketRequired("KEY"), AdminRequired()
api-svc/pkg/server/middleware_accesslog.go        # AccessLogMiddleware — log một dòng/request; statusRecorder forward http.Flusher (bắt buộc cho SSE)
api-svc/pkg/server/helper.go                      # requireGRPCClient(), ResponseError(), ResponseSuccess()
api-svc/pkg/service/predict/registry/registry.go   # AlgorithmDef struct, Register(), All()
api-svc/pkg/service/predict/registry/algorithms.go # FILE DUY NHẤT cần sửa khi thêm/bỏ thuật toán trong metadata registry
api-svc/pkg/store/repository/repository.go        # Interface DatabaseStore (composite)
api-svc/pkg/store/repository/direction_accuracy.go # DirectionAccuracyStore interface
api-svc/pkg/store/repository/user.go              # UserStore interface
api-svc/pkg/store/repository/monitoring.go        # MonitoringStore interface
api-svc/pkg/store/repository/pipeline_report.go   # PipelineReportStore interface
api-svc/pkg/store/postgres/                        # Triển khai PostgreSQL/TimescaleDB dùng GORM
api-svc/pkg/models/models_db/                      # GORM structs: GoldPrice, GoldPrediction, NasdaqPrice, Sp500Price, CryptoPrice, ...
api-svc/pkg/models/models_db/cron_schedule.go      # CronSchedule GORM struct
api-svc/pkg/models/models_db/user.go               # User GORM struct (Username, PasswordHash, Role, FullName, Email, Phone nullable)
api-svc/pkg/models/models_db/pipeline_report.go    # PipelineReport GORM struct (ID, PipelineKey, Market, Status, Steps JSONB, ...)
# Tất cả 4 prediction structs đều có DirectionCorrect *bool (nullable, cột direction_correct)
api-svc/pkg/models/models_api/monitoring_dto.go    # MarketCrawlStats, AlgoPredStats DTOs
api-svc/pkg/models/models_config/config.go         # Config struct (GRPCConfig, ServerConfig)
api-svc/pkg/utils/cron/                            # Hằng số cron schedule (dùng làm default khi seed DB)
api-svc/pkg/testutil/                              # Test helpers (db, fixtures, http)
web-svc/src/context/AuthContext.tsx    # AuthProvider + useAuth hook — quản lý JWT; canAccessMarket(key)
web-svc/src/context/LangContext.tsx    # LangProvider + useLanguage hook — VI/EN toggle
web-svc/src/i18n.ts                    # Bảng dịch VI/EN
web-svc/src/components/LoginModal.tsx  # Login modal — gọi POST /api/x/grant (X-Token header)
web-svc/src/pages/Users.tsx            # Trang quản lý user (admin only)
web-svc/src/pages/MarketGroups.tsx     # Trang quản lý market groups (admin only)
web-svc/src/pages/Monitoring.tsx       # Data Pipeline — bảng bots dùng server-side paging /api/monitoring/bots
web-svc/src/pages/Settings.tsx         # Cài đặt — tab Lịch cron + tab Báo cáo
web-svc/src/pages/Docs.tsx             # Tab Tài liệu — reading room render markdown (react-markdown + highlight.js); gọi GET /api/docs + GET /api/docs/raw; admin-only
```

## Python Prediction Service

```
prediction-svc/
├── deploy/prediction-svc.Dockerfile    # Multi-stage: proto-builder → python:3.12-slim; build context = repo root
├── pyproject.toml                      # Dependencies: torch, statsmodels, lightgbm, xgboost, grpcio, APScheduler, SQLAlchemy, pandas-ta; optuna [ml]
├── src/
│   ├── main.py                         # Entry point — config → timezone → logger → DB → gRPC → scheduler (skip nếu SCHEDULER_ENABLED=false) → registry_client → signal wait
│   ├── config.py                       # Pydantic Settings (env vars; service_mgt_enabled, registry_grpc_target)
│   ├── registry_client.py              # RegistryClient — register/heartbeat daemon thread; re-register khi NOT_FOUND
│   ├── database/
│   │   ├── connection.py               # SQLAlchemy engine + session factory
│   │   ├── models.py                   # ORM models (mirror GORM structs)
│   │   └── repository.py              # Repository pattern — tất cả query methods
│   ├── grpc_server/server.py           # gRPC servicer — implement tất cả RPC trong proto
│   ├── algorithms/
│   │   ├── base.py                     # Abstract PredictionAlgorithm; MARKET_MAX_CHANGE dict; get_max_change_pct()
│   │   ├── registry.py                 # Algorithm registry per-market; set _market_key trên instance
│   │   ├── features.py                 # build_basic_features() (14) và build_enhanced_features() (~30); numpy fallback
│   │   ├── moving_average.py           # VWMA + RSI + StochRSI; clamp market-aware
│   │   ├── ema_macd.py                 # EMA + MACD + Bollinger %B; clamp market-aware
│   │   ├── lstm.py                     # PyTorch LSTM (2 layers, hidden=64, seq=60, dropout=0.2)
│   │   ├── gru.py                      # PyTorch GRU (2 layers, hidden=64, seq=60, dropout=0.2)
│   │   ├── arima_garch.py              # statsmodels ARIMA(2,1,2) + arch GARCH(1,1)
│   │   ├── egarch.py                   # arch EGARCH(p=1, o=1, q=1) với HARX mean model
│   │   ├── sarima.py                   # statsmodels SARIMA(1,1,1)(1,0,1,5) — seasonal period 5
│   │   ├── lightgbm_model.py           # LightGBM ~30 features; Optuna (≥200 pts, 30 trials, 120s)
│   │   ├── xgboost_model.py            # XGBoost ~30 features; Optuna (≥200 pts, 30 trials, 120s)
│   │   ├── random_forest.py            # RandomForest n_estimators=200, max_depth=8; không Optuna
│   │   ├── ensemble.py                 # Accuracy-weighted ensemble 10 base models; fallback equal-weight
│   │   ├── transformer_model.py        # Transformer #13 (PatchTST-lite): patch attention trên log-returns + static context từ stock_fundamentals; checkpoint transformer_{market}.pt; train_batch_labeled; KHÔNG trong Ensemble
│   │   └── rl_dqn.py                   # Dueling Double-DQN v3 (LayerNorm→128→64→V/A heads; PER + 3-step return + Polyak; reward vol-normalized + directional shaping; walk-forward val chọn epoch tốt nhất; feature scaling tĩnh); checkpoint rl_dqn_{market}.pt tag arch/feat_scale — checkpoint MLP cũ vẫn load; KHÔNG trong Ensemble
│   ├── crawlers/
│   │   ├── base.py                     # Abstract BaseCrawler
│   │   ├── gold.py                     # Yahoo Finance XAU + BTMC API + Phú Quý
│   │   ├── nasdaq.py                   # Yahoo Finance — 15 NASDAQ symbols
│   │   ├── crypto.py                   # CoinGecko — BTC/ETH/SOL
│   │   ├── sp500.py                    # Yahoo Finance — 16 S&P 500 symbols
│   │   ├── fundamentals.py             # yfinance — báo cáo tài chính NASDAQ+SP500 → stock_fundamentals (tuần)
│   │   └── sanity.py                   # Crawl sanity guard: check_update() persistence-confirmed daily gate + batch_outlier_mask() bilateral intraday spike filter
│   ├── storage/
│   │   └── model_store.py              # ModelStore abstraction: backend local|s3; ensure_local()/upload_if_remote(); singleton get_store(); wire vào rl_dqn/transformer/meta_stack
│   ├── telemetry/
│   │   ├── tracing.py                  # OTel SDK tracing setup — OTLP/gRPC exporter → otel-collector.observability:4317
│   │   └── metrics.py                  # Prometheus client — HTTP metrics server :9464
│   ├── scheduler/
│   │   ├── manager.py                  # APScheduler + DB-backed CronSchedule; poll 60s; DEFAULT_SCHEDULES; guard SCHEDULER_ENABLED
│   │   └── jobs.py                     # Định nghĩa tất cả jobs (_run_pipeline, train, reconcile); JOB_FUNCTIONS dict (dùng bởi jobs_cli.py)
│   ├── jobs_cli.py                     # CLI entrypoint cho k8s Job/CronJob: `python -m src.jobs_cli <job_key|gold|nasdaq|crypto|sp500>`; --list; exit 0/1/2; KHÔNG gRPC KHÔNG scheduler
│   ├── orchestrator/
│   │   ├── runner.py                   # run_for_market(key); target = now+1h; is_intraday_open guard NASDAQ/SP500
│   │   ├── training.py                 # reconcile_predictions(only_market=None); train_for_market(); train_meta_for_market(); train_meta_all()
│   │   └── splits.py                   # apply_pending_splits(); _apply_one_split() — phát hiện boundary từ data, chỉnh intraday NASDAQ/SP500; idempotent
│   ├── simulation/
│   │   ├── bot.py                      # Bot step logic: is_rl → _step_rl, is_meta → _step_meta, còn lại → _step_threshold
│   │   ├── meta_stack.py               # MetaStackModel: LightGBM classifier + calibration isotonic → P(up); fallback reliability-weighted vote; KHÔNG thuộc algorithms/
│   │   ├── portfolio.py                # Portfolio; buy() có kwarg position_pct tùy chọn (sizing theo conviction, dùng bởi meta_stack)
│   │   └── seeder.py                   # Seed bots: algo pooled + per-symbol + rl_dqn + meta_stack (+ meta_stack__ps)
│   └── utils/
│       ├── logger.py                   # structlog config
│       ├── timezone.py                 # Asia/Ho_Chi_Minh helpers
│       ├── number_parser.py            # Vietnamese number format (1.234,56 → 1234.56)
│       └── market_calendar.py          # is_market_open() + is_intraday_open() — không phụ thuộc thư viện ngoài
├── tests/
│   ├── conftest.py                     # Fixtures: DB session, gRPC stub, test data
│   ├── unit/                           # Unit tests cho từng algorithm
│   └── integration/                    # End-to-end tests qua gRPC và HTTP API
└── proto/prediction/prediction_pb2*.py # Generated Python stubs (không sửa tay)
```

**Tài liệu chiến thuật bot:** thư mục `tactics/` (bên dưới `docs/`) chứa tài liệu CHIẾN THUẬT giao dịch của bot — phân biệt với `algos/` là tài liệu thuật toán dự đoán. `tactics/README.md` giải thích phân tầng; `tactics/meta-stacking.md` mô tả toán học bản supervised meta-stack; `tactics/meta-rl.md` mô tả phương án RL meta (doc-only, chưa code).

## Java Auth Service

```
auth-svc/                               # Spring Boot 3 Java Auth Service — gRPC :8120 (internal)
├── deploy/auth-svc.Dockerfile
├── src/main/java/
│   └── ...                             # AuthGrpcServiceImpl (17+ RPCs), SuperAdminSeeder, MarketGroupService,
│                                       # GrpcLoggingInterceptor (@GrpcGlobalServerInterceptor) — log 1 dòng/RPC
│                                       # RegistryClient — @EventListener register, @Scheduled(10s) heartbeat, @PreDestroy deregister
├── src/main/proto/
│   ├── auth.proto                      # KHÔNG SỬA TAY — generated bởi `make proto-auth-sync` (copy từ api-svc/proto/auth/auth.proto)
│   └── registry.proto                  # Copy của service-mgt/proto/registry/registry.proto
└── src/main/resources/
    ├── application.yml                 # spring.output.ansi.enabled: always; cấu hình service-mgt
    ├── logback-spring.xml              # Single-line có màu (%clr); INFO+ root logger
    └── db/migration/                   # V1__create_auth_tables.sql, V2__add_user_profile_fields.sql, V3__create_command_rbac.sql
```

**Lưu ý:** Maven artifactId và Spring app name vẫn là `auth-service` — chỉ đường dẫn thư mục thay đổi thành `auth-svc/`.

## Rust Gateway Service

```
gateway-svc/                            # Rust Axum — expose :80 (HTTP) và :443 (HTTPS)
├── deploy/gateway-svc.Dockerfile       # builder stage cài protobuf-compiler
├── Cargo.toml                          # axum 0.8, axum-server (tls-rustls), reqwest, tokio, tonic, prost
├── build.rs                            # tonic-build từ gateway-svc/proto/registry.proto
├── proto/registry.proto                # Copy của service-mgt registry.proto
├── config.yaml                         # Route rules, TLS config, HTTP pool settings
├── docker-entrypoint.sh                # Tạo self-signed cert nếu chưa có
└── src/
    ├── main.rs                         # load config → discover registry → spawn heartbeat → serve HTTP+HTTPS
    ├── config/mod.rs                   # AppConfig (serde_yaml) — RouteConfig, TlsConfig, HttpConfig
    ├── registry/mod.rs                 # discover() lúc boot; spawn_register() heartbeat; flag-gated SERVICE_MGT_ENABLED
    ├── router/mod.rs                   # PathRouter — longest-prefix match; Block | Proxy
    ├── routes/proxy.rs                 # proxy_handler — đọc PathRouter, gọi ProxyClient hoặc trả 404
    ├── proxy/client.rs                 # ProxyClient (reqwest) — forward request, stream response
    └── middleware/                     # logging, request_id (UUID v4), security_headers
```

**Route table (config.yaml, longest-prefix wins):**

| Prefix | Action | Backend | Security Headers |
|--------|--------|---------|-----------------|
| `/swagger` | block (404) | — | — |
| `/api` | proxy | `http://api-svc:8118` | bật |
| `/health` | proxy | `http://api-svc:8118` | tắt |
| `/` | proxy | `http://web-svc:3000` | tắt |

Gateway-local: `GET /healthz`, `GET /readyz` (không proxy).

## CLI Service

```
cli-svc/
├── main.go                      # wish.Server :2345; upsert handler catalog khi boot
└── internal/
    ├── server/                  # password-auth → POST /api/x/grant; boot handler-upsert
    ├── shell/                   # bubbletea Model: model.go, update.go, view.go, completer.go
    ├── handlers/                # Handler catalog registry
    ├── client/                  # HTTP → gateway (/api), gắn JWT từ ssh.Context
    └── render/                  # go-pretty/lipgloss table helpers
```

**Handler catalog** (verb: `get`→GET, `set`→POST, `update`→PUT, `delete`→DELETE):

| handler_key | verb | API call | args |
|---|---|---|---|
| `market.latest` | get | GET /api/{market}/latest | market |
| `market.prices` | get | GET /api/{market}/prices | market, limit |
| `market.predictions` | get | GET /api/{market}/predictions/latest | market |
| `direction.accuracy` | get | GET /api/predictions/direction-accuracy | market |
| `monitoring.overview` | get | GET /api/monitoring/overview | — |
| `schedules.list` | get | GET /api/schedules | — |
| `pipeline.reports` | get | GET /api/pipeline-reports | pipeline, limit |
| `training.status` | get | GET /api/training/status | — |
| `users.list` | get | GET /api/users | — |
| `backups.list` | get | GET /api/backups | — |
| `trigger.train` | set | POST /api/trigger/train | algorithm? |
| `trigger.crawler` | set | POST /api/trigger/{market}-crawler | market |
| `trigger.predict` | set | POST /api/trigger/{market}-predict | market |
| `trigger.reconcile` | set | POST /api/trigger/reconcile | — |
| `trigger.backup` | set | POST /api/trigger/backup | — |
| `schedule.update` | update | PUT /api/schedules/{key} | key, cron_expression, enabled |
| `user.update` | update | PUT /api/users/{id} | id, role?, full_name?, email?, phone? |
| `backup.delete` | delete | DELETE /api/backups/{filename} | filename |
| `user.delete` | delete | DELETE /api/users/{id} | id |

market ∈ {gold, nasdaq, crypto, sp500}.

**Flow phiên SSH:** connect → password-auth → lưu `{jwt, role, user_id}` → shell khởi tạo → gọi `GET /api/me/commands` → build allowed-set → user gõ → resolve (handler_key + args) → kiểm tra allowed-set → chạy handler → render table.

## Service Management

```
service-mgt/                                # Go Service Registry — gRPC :8121 (internal only)
├── main.go
├── proto/registry/registry.proto          # Register/Heartbeat/Deregister/Discover/ListServices
├── client/client.go                        # Go client SDK: Register/Heartbeat goroutine/Discover; static fallback
└── internal/
    ├── config/                             # Load env vars (SERVER_PORT, DB, TTL defaults)
    ├── store/                              # Postgres store — bảng service_instances
    ├── registry/                           # Lease TTL; reaper (1s); flusher (30s)
    └── grpcserver/                         # gRPC servicer
```

**Lưu ý build context:** `api-svc` import `service-mgt` qua `replace => ../service-mgt` trong `go.mod` → build context Docker = repo root để COPY cả hai. `cli-svc` KHÔNG import service-mgt — build context vẫn là `../cli-svc`.

## Deploy folder

```
deploy/
├── docker-compose.yaml          # Toàn bộ stack; thêm service ofelia (container-native cron); SCHEDULER_ENABLED=false + BACKUP_SCHEDULER_ENABLED=false trên các service liên quan
├── docker-compose.test.yml      # Test stack
├── api-svc.Dockerfile           # Go API Backend (build context = repo root)
├── auth-svc.Dockerfile          # Java Auth Service
├── prediction-svc.Dockerfile    # Python Prediction (build context = repo root)
├── web-svc.Dockerfile           # React Frontend
├── gateway-svc.Dockerfile       # Rust Gateway
├── cli-svc.Dockerfile           # Go CLI (build context = ../cli-svc riêng)
├── service-mgt.Dockerfile       # Go Service Registry (build context = ../service-mgt)
└── k8s/
    ├── observability/           # ns observability: namespace.yaml, otel-collector.yaml, prometheus.yaml, tempo.yaml, grafana.yaml
    ├── minio/                   # ns stock: minio.yaml (Deployment+PVC+Service, bucket models)
    ├── pipeline/                # CronJob: cronjob-gold/nasdaq/crypto/sp500.yaml (crawler); cronjob-train.yaml (train_*+train_meta); cronjob-weekly.yaml (fundamentals+train_transformer+daily_reconcile); cronjob-backup.yaml (pg_dump+PVC backup-data); cronjob-simulation.yaml
    ├── api-svc/                 # Deployment, Service, ConfigMap (api-config)
    ├── auth-svc/                # Deployment, Service, ConfigMap
    ├── prediction-svc/          # Deployment (replicas=2), Service, ConfigMap (prediction-config: SCHEDULER_ENABLED=false, MODEL_STORE_BACKEND=s3)
    ├── gateway-svc/             # Deployment, Service (LoadBalancer :80/:443)
    ├── cli-svc/                 # Deployment, Service
    ├── service-mgt/             # Deployment, Service
    ├── web-svc/                 # Deployment, Service
    ├── db/                      # StatefulSet TimescaleDB
    └── pgadmin/                 # Deployment pgAdmin
```
