# CLAUDE.md

## Tổng quan dự án

Hệ thống dự đoán giá tài sản tài chính. Thu thập dữ liệu từ 4 nguồn (Gold SJC/XAU, NASDAQ, Crypto BTC/ETH/SOL, S&P 500), chạy 11 thuật toán ML (Moving Average, EMA/MACD, LSTM PyTorch, GRU PyTorch, ARIMA-GARCH, EGARCH, SARIMA, LightGBM, XGBoost, Random Forest, Ensemble), và hiển thị kết quả qua web dashboard.

**Kiến trúc hiện tại: microservice (2 service + nginx)**
- **API Backend** (`api/cmd`) — Go HTTP trên `:8118`, phục vụ toàn bộ `/api/*`. Gọi Prediction Service qua gRPC để trigger crawler/training; đọc DB trực tiếp cho các query dữ liệu.
- **Prediction Service** (`prediction/`) — **Python** gRPC trên `:8119`. Xử lý crawling (Gold SJC/XAU, NASDAQ, Crypto BTC/ETH/SOL, S&P 500), 11 thuật toán ML, training, APScheduler cron jobs.
- **Nginx** — Reverse proxy trên `:80`, forward tất cả request về API Backend.


## Lệnh thường dùng

```bash
# Build API Backend (Go) — chạy từ trong thư mục api/
cd api && go build -o api-server ./cmd

# Start tất cả services (DB + Python Prediction + API + Nginx + Frontend + phpMyAdmin)
docker-compose up -d

# Import schema
mysql -u root -p go_stock_prediction < database.sql

# Regenerate proto Go stubs (cần protoc + plugins) — chạy từ thư mục api/
cd api && protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/prediction/prediction.proto

# Regenerate proto Python stubs (chạy trong Docker hoặc local với grpcio-tools)
cd prediction
python -m grpc_tools.protoc -Iapi/proto --python_out=src/proto --grpc_python_out=src/proto api/proto/prediction/prediction.proto

# Generate Swagger docs (cần swag CLI: go install github.com/swaggo/swag/v2/cmd/swag@latest) — chạy từ thư mục api/
cd api && swag init -g cmd/main.go -o docs/

# Chạy test Python prediction service
cd prediction && make test
# hoặc chạy trong container đang chạy:
docker exec prediction_service python -m pytest tests/ -v
```

## Cấu trúc thư mục quan trọng

### Go API Backend

```
api/cmd/main.go                         # API Backend entry point — config → timezone → logger → repository → grpcclient → server
api/pkg/config/                         # Load .env, trả về config struct toàn cục (bao gồm GRPCConfig)
api/pkg/grpc/client/client.go           # gRPC client singleton — API Backend dùng để gọi Python Prediction Service
api/proto/prediction/prediction.proto   # gRPC service definitions (shared với Python service)
api/proto/prediction/*.pb.go            # Generated Go protobuf stubs (không sửa tay)
api/docs/swagger.json                   # Generated Swagger spec (không sửa tay — chạy swag init)
api/docs/swagger.yaml                   # Generated Swagger YAML spec
api/docs/docs.go                        # Generated Go package cho swagger
nginx/nginx.conf                        # Nginx reverse proxy: :80 → api:8118
api/Dockerfile                          # Docker build cho API Backend (Go)
api/pkg/server/router.go                # Đăng ký tất cả routes (API)
api/pkg/server/api_*.go                 # Mỗi file = một nhóm API endpoint
api/pkg/server/api_direction_accuracy.go    # GET /api/predictions/direction-accuracy — direction accuracy per algorithm per market
api/pkg/server/api_training_algorithms.go   # GET /api/training/algorithms
api/pkg/server/api_training_metrics.go      # GET /api/training/metrics
api/pkg/server/api_gold.go                  # GET /api/gold/latest, /api/gold/prices, /api/gold/chart
api/pkg/server/api_gold_prediction.go       # GET /api/gold/predictions/latest, /api/gold/predictions/chart, /api/gold/predictions
api/pkg/server/api_nasdaq.go                # GET /api/nasdaq/latest, /api/nasdaq/prices, /api/nasdaq/chart
api/pkg/server/api_nasdaq_prediction.go     # GET /api/nasdaq/predictions/*
api/pkg/server/api_crypto.go                # GET /api/crypto/latest, /api/crypto/prices, /api/crypto/chart
api/pkg/server/api_crypto_prediction.go     # GET /api/crypto/predictions/*
api/pkg/server/api_sp500.go                 # GET /api/sp500/latest, /api/sp500/prices, /api/sp500/chart
api/pkg/server/api_sp500_prediction.go      # GET /api/sp500/predictions/*
api/pkg/server/api_markets.go               # GET /api/markets/{key}/predictions, GET /api/markets/{key}/training
api/pkg/server/api_training_status.go       # GET /api/training/status (gọi gRPC GetTrainingStatus)
api/pkg/server/api_dashboard_stats.go       # GET /api/dashboard/stats
api/pkg/server/api_trigger_train.go         # POST /api/trigger/train (→ gRPC TriggerTrain)
api/pkg/server/api_trigger_gold_crawler.go  # POST /api/trigger/gold-crawler (→ gRPC TriggerGoldCrawler)
api/pkg/server/api_trigger_gold_history.go  # POST /api/trigger/gold-history, POST /api/trigger/gold-predict (→ gRPC)
api/pkg/server/api_trigger_reconcile.go     # POST /api/trigger/reconcile (→ gRPC TriggerReconcile)
api/pkg/server/api_trigger_historical_backtest.go # POST /api/trigger/historical-backtest (→ gRPC TriggerHistoricalBacktest, trả 202, 409 nếu đang chạy)
api/pkg/server/api_trigger_gold_historical_backtest.go # POST /api/trigger/gold-historical-backtest (→ gRPC)
api/pkg/server/api_trigger_nasdaq_crawler.go  # POST /api/trigger/nasdaq-crawler, POST /api/trigger/nasdaq-predict (→ gRPC)
api/pkg/server/api_trigger_crypto_crawler.go  # POST /api/trigger/crypto-crawler, POST /api/trigger/crypto-predict (→ gRPC)
api/pkg/server/api_trigger_sp500.go          # POST /api/trigger/sp500-crawler (→ gRPC TriggerSP500Crawler), POST /api/trigger/sp500-predict (→ gRPC TriggerSP500Predict)
api/pkg/server/api_trigger_simulation.go     # POST /api/trigger/simulation-backtest, POST /api/trigger/simulation-live-step, POST /api/trigger/sim-reset (→ gRPC)
api/pkg/server/api_auth.go                  # POST /api/auth/login (DB + bcrypt), GET /api/auth/me — JWT authentication
api/pkg/server/api_users.go                 # GET/POST /api/users, DELETE /api/users/{id} — quản lý user (admin only)
api/pkg/server/api_schedules.go             # GET /api/schedules, PUT /api/schedules/{key} — quản lý lịch cron động
api/pkg/server/api_backup.go                # GET /api/backups, POST /api/trigger/backup, GET/DELETE /api/backups/{filename}
api/pkg/server/api_simulation.go            # GET /api/simulation/leaderboard, bots, trades, chart; PUT config; POST toggle/run
api/pkg/server/api_monitoring.go            # GET /api/monitoring/overview — monitoring overview: crawl freshness, prediction activity per algo per market, bot win/loss (JWT required, cache 30s)
api/pkg/server/middleware_jwt.go            # JWTMiddleware (non-blocking, inject claims vào context), getClaims(), requireAuth(), requireAdmin(), AuthRequired()
api/pkg/server/helper.go                    # requireGRPCClient(), ResponseError(), ResponseSuccess() và các helper
api/pkg/service/predict/registry/registry.go   # AlgorithmDef struct (metadata only — không có Factory), Register(), All()
api/pkg/service/predict/registry/algorithms.go # FILE DUY NHẤT cần sửa khi thêm/bỏ thuật toán trong metadata registry
api/pkg/store/repository/repository.go     # Interface DatabaseStore (composite) — bao gồm GetConfirmedPredictionsPage, DeletePredictionsBeforeDate, BulkCreatePredictions, CronScheduleStore, MonitoringStore
api/pkg/store/repository/direction_accuracy.go # DirectionAccuracyStore interface — GetDirectionAccuracy(market)
api/pkg/store/repository/user.go           # UserStore interface — CreateUser, GetUserByUsername, GetUserByID, GetAllUsers, DeleteUser, AdminExists
api/pkg/store/repository/monitoring.go     # MonitoringStore interface — GetMarketCrawlStats(market), GetMarketPredStats(market)
api/pkg/store/mysql/                        # Triển khai MySQL dùng GORM
api/pkg/store/mysql/user.go                 # MySQL implementation của UserStore
api/pkg/store/mysql/cron_schedule.go        # MySQL implementation của CronScheduleStore — GetAllCronSchedules, GetCronScheduleByKey, UpsertCronSchedule
api/pkg/store/mysql/direction_accuracy.go   # MySQL implementation của DirectionAccuracyStore — raw SQL query GROUP BY algorithm trên 4 prediction tables
api/pkg/store/mysql/monitoring.go           # MySQL implementation của MonitoringStore — raw SQL crawl freshness (daily+intraday tables) và per-algo pred counts per market
api/pkg/models/models_db/                   # GORM struct: GoldPrice, GoldPrediction, NasdaqPrice, Sp500Price, CryptoPrice, intraday prices, SyncLog, TrainingLog, TrainingMetrics, User, CronSchedule
api/pkg/models/models_db/cron_schedule.go   # CronSchedule GORM struct (JobKey, JobName, CronExpression, Enabled, UpdatedAt)
api/pkg/models/models_db/user.go            # User GORM struct (Username, PasswordHash, Role)
api/pkg/models/models_db/gold_price.go      # GoldPrice GORM struct
api/pkg/models/models_db/training_log.go    # TrainingLog GORM struct
api/pkg/models/models_db/training_metrics.go # TrainingMetrics struct
# Tất cả 4 prediction structs (GoldPrediction, NasdaqPrediction, Sp500Prediction, CryptoPrediction)
# đều có trường DirectionCorrect *bool (nullable, cột direction_correct trong DB)
api/pkg/models/models_api/                  # DTO cho JSON response — bao gồm DirectionAccuracyRow{Algorithm, Total, Correct}, MarketCrawlStats, AlgoPredStats
api/pkg/models/models_api/monitoring_dto.go # MarketCrawlStats (LastDailyAt, LastIntradayAt, DailyToday, IntradayToday) và AlgoPredStats (AlgorithmName, TodayCount, LastPredictAt)
api/pkg/models/models_config/config.go      # Config struct — bao gồm GRPCConfig (ServerPort, ClientTarget) và ServerConfig (AdminUsername, AdminPassword, JWTSecret)
api/pkg/utils/cron/                         # Hằng số cron schedule + wrapper (dùng làm giá trị mặc định khi seed DB)
api/pkg/testutil/                           # Test helpers (db, fixtures, http)
frontend/src/context/AuthContext.tsx    # AuthProvider + useAuth hook — quản lý JWT trong localStorage
frontend/src/context/LangContext.tsx    # LangProvider + useLanguage hook — VI/EN toggle, state lưu localStorage (vns_lang)
frontend/src/i18n.ts                    # Bảng dịch VI/EN cho toàn bộ UI shell (nav, topbar, sidebar, tweaks)
frontend/src/components/LoginModal.tsx  # Login modal component — gọi POST /api/auth/login
frontend/src/pages/Users.tsx            # Trang quản lý user — chỉ hiển thị với role admin
frontend/src/pages/Monitoring.tsx       # Trang Data Pipeline — gọi GET /api/monitoring/overview; hiển thị 4 market cards (crawl freshness, per-algo prediction stats) + bots summary table + bots full table (sortable); route /monitoring, sidebar "Giám sát dữ liệu" (VI) / "Data Pipeline" (EN)
```

### Python Prediction Service

```
prediction/
├── Dockerfile                          # Multi-stage build: proto-builder (grpcio-tools) → python:3.12-slim runtime
├── pyproject.toml                      # Python dependencies (torch, statsmodels, lightgbm, xgboost, grpcio, APScheduler, SQLAlchemy, pandas-ta; optuna trong [ml] extras)
├── Makefile                            # make test, make test-phase5, v.v.
├── src/
│   ├── main.py                         # Entry point — config → timezone → logger → DB → gRPC → scheduler → startup sync → signal wait
│   ├── config.py                       # Pydantic Settings (load env vars, mirror Go config)
│   ├── database/
│   │   ├── connection.py               # SQLAlchemy sync engine + session factory
│   │   ├── models.py                   # ORM models (mirror GORM structs: Stock, StockPrice, Prediction, GoldPrice, ...)
│   │   └── repository.py              # Repository pattern — tất cả query methods
│   ├── grpc_server/
│   │   └── server.py                   # gRPC servicer — implement tất cả RPC trong proto
│   ├── algorithms/
│   │   ├── base.py                     # Abstract PredictionAlgorithm interface; MARKET_MAX_CHANGE dict + get_max_change_pct() helper; _market_key attr set by registry
│   │   ├── registry.py                 # Algorithm registry (build_algorithms, get_algos_for_market, set_algos_for_market) — per-market instance cache; set _market_key trên mỗi instance
│   │   ├── features.py                 # Shared feature builder cho tree-based models: build_basic_features() (14 features) và build_enhanced_features() (~30 features); graceful fallback numpy-only khi pandas-ta vắng
│   │   ├── moving_average.py           # VWMA trend slope projection + RSI momentum scaling + StochRSI overlay (overbought/oversold); clamp market-aware; pandas-ta optional
│   │   ├── ema_macd.py                 # EMA slope projection + MACD momentum boost + Bollinger %B overlay (mean-reversion near band extremes); clamp market-aware; pandas-ta optional
│   │   ├── lstm.py                     # PyTorch LSTM (2 layers, hidden=64, seq=60, dropout=0.2)
│   │   ├── gru.py                      # PyTorch GRU (2 layers, hidden=64, seq=60, dropout=0.2)
│   │   ├── arima_garch.py              # statsmodels ARIMA(2,1,2) + arch GARCH(1,1)
│   │   ├── egarch.py                   # arch EGARCH(p=1, o=1, q=1) với HARX mean model
│   │   ├── sarima.py                   # statsmodels SARIMA(1,1,1)(1,0,1,5) — seasonal period 5 (trading week)
│   │   ├── lightgbm_model.py           # LightGBM enhanced ~30 features; Optuna hyperopt (≥200 points, 30 trials, timeout 120s); EMA fallback
│   │   ├── xgboost_model.py            # XGBoost enhanced ~30 features; Optuna hyperopt (≥200 points, 30 trials, timeout 120s); EMA fallback
│   │   ├── random_forest.py            # scikit-learn RandomForest enhanced ~30 features; n_estimators=200, max_depth=8; không có Optuna
│   │   └── ensemble.py                 # Equal-weight ensemble của tất cả 10 base models
│   ├── crawlers/
│   │   ├── base.py                     # Abstract BaseCrawler
│   │   ├── gold.py                     # Yahoo Finance XAU + BTMC API + Phú Quý
│   │   ├── nasdaq.py                   # Yahoo Finance — 15 NASDAQ symbols
│   │   ├── crypto.py                   # CoinGecko — BTC/ETH/SOL
│   │   └── sp500.py                    # Yahoo Finance — 16 S&P 500 symbols (SPY, QQQ, JPM, BAC, GS, JNJ, UNH, PFE, PG, KO, WMT, XOM, CVX, V, MA)
│   ├── scheduler/
│   │   ├── manager.py                  # APScheduler + DB-backed CronSchedule; poll mỗi 60s để phát hiện thay đổi
│   │   └── jobs.py                     # Định nghĩa tất cả jobs (crawlers + predictions + training + reconcile)
│   ├── orchestrator/
│   │   ├── runner.py                   # run_all_markets() — GOLD/NASDAQ100/CRYPTO/SP500; run_for_market(key)
│   │   └── training.py                 # reconcile_predictions() — tính direction_correct cho 4 markets (GOLD/NASDAQ/SP500/CRYPTO); train_for_market()
│   └── utils/
│       ├── logger.py                   # structlog config
│       ├── timezone.py                 # Asia/Ho_Chi_Minh helpers
│       ├── number_parser.py            # Vietnamese number format (1.234,56 → 1234.56)
│       └── market_calendar.py          # is_market_open(market_key, when) — GOLD/CRYPTO luôn True; NASDAQ/SP500 False vào cuối tuần + ngày lễ NYSE (US/Eastern); không phụ thuộc thư viện ngoài
├── tests/
│   ├── conftest.py                     # Fixtures: DB session, gRPC stub, test data
│   ├── unit/                           # Unit tests cho từng algorithm
│   └── integration/                    # End-to-end tests qua gRPC và HTTP API
└── proto/
    └── prediction/
        └── prediction_pb2*.py          # Generated Python stubs (không sửa tay — tái sinh trong Dockerfile stage 1)
```

## Luồng khởi động

### API Backend (api/cmd/main.go) — Go

1. `config.InitConfig()` — Load file `.env`
2. Set timezone — `Asia/Ho_Chi_Minh`
3. `logger.Init()` — Khởi tạo ZeroLog
4. `repository.Init()` — Kết nối MySQL (shared DB, dùng cho read queries)
5. `seedAdminUser()` — Tạo admin user từ env vars nếu chưa có admin nào trong DB
6. `grpcclient.Init(config.GetGRPCConfig().ClientTarget)` — Kết nối tới Python Prediction Service
7. `server.StartHTTPServer()` — HTTP server trên `:8118` (goroutine)
8. Chờ SIGTERM/SIGINT → `grpcclient.Close()` → shutdown

### Prediction Service (prediction/src/main.py) — Python

1. Load config — Pydantic Settings từ env vars
2. Set timezone — `TZ=Asia/Ho_Chi_Minh` + `time.tzset()`
3. `init_logger()` — Khởi tạo structlog
4. `init_db()` — SQLAlchemy engine + session factory
5. `start_grpc_server(port)` — gRPC server trên `:8119` (thread)
6. `init_scheduler(JOB_FUNCTIONS)` — APScheduler với DB-backed schedules, poll mỗi 60s
7. Startup data sync — không còn chạy crawler tự động khi khởi động (đã bỏ VN30 startup sync)
8. Chờ SIGTERM/SIGINT → `stop_grpc_server()` + `shutdown_scheduler()` → exit

## Biến môi trường (.env)

```
# HTTP server (API Backend)
SERVER_PORT=8118

# gRPC
GRPC_SERVER_PORT=8119          # Port Prediction Service lắng nghe
GRPC_TARGET=localhost:8119     # Địa chỉ API Backend dùng để kết nối Prediction Service (Docker: prediction:8119)

# Auth
API_KEY=                       # Optional — hiện tại không được dùng để bảo vệ trigger endpoints (đã chuyển sang JWT)
ADMIN_USERNAME=admin           # Username đăng nhập dashboard (default: admin)
ADMIN_PASSWORD=admin123        # Password đăng nhập dashboard (default: admin123)
JWT_SECRET=change-me-in-production  # Secret ký JWT — bắt buộc đổi trong production

# Database
DB_DRIVER=mysql
MYSQL_HOST=localhost
MYSQL_PORT=3306
MYSQL_USER=root
MYSQL_PASSWORD=123
MYSQL_DB_NAME=go_stock_prediction
MYSQL_DEBUG=false

# Logging
LOG_LEVEL=DEBUG
DB_LOG_LEVEL=DEBUG

# Backup
BACKUP_DIR=/backups              # Thư mục lưu file backup mysqldump (mount vào cả api và prediction containers)
```

## Conventions trong codebase

- **Repository pattern (Go):** Mọi truy cập DB từ API Backend phải qua interface `DatabaseStore` trong `api/pkg/store/repository/`. Không gọi GORM trực tiếp từ service layer.
- **Singleton:** `repository.GetSingleton()` trả về instance DB toàn cục đã init.
- **Cron constants:** Hằng số trong `api/pkg/utils/cron/` dùng làm giá trị mặc định trong `seedCronSchedules()` (Go, insert-only). Lịch chạy thực tế cho tất cả jobs Python-side được định nghĩa trong `DEFAULT_SCHEDULES` tại `prediction/src/scheduler/manager.py` và được upsert vào DB mỗi lần Python service khởi động.
- **Decimal:** Dùng `shopspring/decimal` trong Go API Backend cho mọi phép tính số thực liên quan đến giá — tránh float64. Python service dùng `Decimal` từ stdlib hoặc pandas float64 (được làm tròn trước khi lưu DB).
- **API handlers (Go):** Mỗi nhóm endpoint có file riêng `api_<topic>.go` trong `api/pkg/server/`.
- **Algorithms (Python):** Mỗi thuật toán implement abstract class `PredictionAlgorithm` trong `prediction/src/algorithms/base.py` với method `predict(prices, volumes) -> PredictionResult`. Đăng ký metadata tương ứng trong `api/pkg/service/predict/registry/algorithms.go` (Go) để `/api/training/algorithms` trả đúng danh sách. Tổng cộng 11 thuật toán: moving_average, ema, lstm_nn, gru_nn, arima_garch, egarch, sarima, lightgbm, xgboost, random_forest, ensemble.
- **Market-aware clamp:** Tất cả algorithms dùng `get_max_change_pct(self._market_key)` từ `base.py` để giới hạn thay đổi giá dự đoán. Giới hạn theo market: GOLD/SP500 ±15%, NASDAQ100 ±20%, CRYPTO ±50%. Registry set `_market_key` trên instance trước khi gọi `predict()`. Market key không xác định dùng `DEFAULT_MAX_CHANGE = 0.15`.
- **Direction accuracy:** Sau khi reconcile, trường `direction_correct` (nullable boolean) được lưu vào 4 prediction tables (`gold_predictions`, `nasdaq_predictions`, `sp500_predictions`, `crypto_predictions`). Giá trị `True` khi hướng dự đoán (tăng/giảm so với giá hiện tại) khớp với hướng thực tế; `NULL` khi chưa có giá thực tế. Query tổng hợp qua `DirectionAccuracyStore` (Go) hoặc `get_direction_accuracy()` (Python repository).
- **Logging:** Go API Backend dùng `api/pkg/logger` (zerolog). Python service dùng `structlog`.
- **gRPC triggers:** Tất cả trigger handler trong `api/pkg/server/api_trigger_*.go` đều gọi `requireGRPCClient(w)` trước. Hàm này trả về 503 nếu gRPC client chưa init. Tất cả trigger endpoints được wrap bằng `AuthRequired()` trong router — yêu cầu JWT hợp lệ.
- **Dynamic cron schedules:** Lịch cron được lưu trong bảng `cron_schedules`. Python Prediction Service poll DB mỗi 60 giây để phát hiện thay đổi và tự reschedule qua APScheduler — không cần restart. Nguồn sự thật là `DEFAULT_SCHEDULES` trong `prediction/src/scheduler/manager.py`; mỗi lần Python service khởi động, `upsert_cron_schedule()` chạy true upsert — ghi đè DB nếu giá trị code khác. Go `seedCronSchedules()` chỉ insert-if-not-exists (không update). Dùng `CronScheduleStore` interface (Go) để truy cập từ API Backend.
- **Proto regeneration:** Khi thay đổi `api/proto/prediction/prediction.proto`, cần tái sinh cả Go stubs (`protoc` chạy từ `api/`) lẫn Python stubs (lệnh `grpc_tools.protoc` trong Dockerfile stage 1). Không sửa tay các file generated.
- **Data ordering — QUAN TRỌNG:** DB trả `stock_prices` với `ORDER BY trading_date DESC` (mới nhất trước). Python algorithms cần đảo ngược về ASC trước khi build feature sequences. Repository (Python) trả DESC — tầng algorithm tự xử lý (tương tự pattern Go cũ với `reverseStockPrices()`).
- **JWT middleware — non-blocking:** `JWTMiddleware` trong `api/pkg/server/middleware_jwt.go` nằm trong middleware chain `CORS → RateLimit → JWT → mux`. Middleware này chỉ inject claims vào context nếu token hợp lệ — request không có token vẫn tiếp tục (unauthenticated). Các handler bảo vệ dùng `requireAuth(w, r)` hoặc `requireAdmin(w, r)` để enforce.
- **Admin seeder:** Khi startup, `seedAdminUser()` trong `api/cmd/main.go` kiểm tra `AdminExists()`. Nếu chưa có user nào với `role="admin"`, tạo một user mới từ `ADMIN_USERNAME`/`ADMIN_PASSWORD` env vars với bcrypt hash. Chạy một lần duy nhất — các lần sau bỏ qua nếu admin đã tồn tại.
- **User management:** `ADMIN_USERNAME`/`ADMIN_PASSWORD` trong `.env` chỉ dùng để seed lần đầu. Sau đó quản lý user hoàn toàn qua API `/api/users` (admin JWT required). Password lưu dưới dạng bcrypt hash — không lưu plaintext.
- **Swagger annotations:** Mỗi handler function trong `api/pkg/server/api_*.go` có swaggo annotations (`@Summary`, `@Tags`, `@Param`, `@Success`, `@Router`). Khi thêm handler mới, phải thêm annotations. Sau khi thêm/sửa annotations, chạy `cd api && swag init -g cmd/main.go -o docs/` để regenerate. Không sửa tay files trong `api/docs/`.
- **Shared feature builder (Python):** `prediction/src/algorithms/features.py` cung cấp hai hàm dùng chung cho LightGBM, XGBoost, RandomForest: `build_basic_features()` (14 features: lag returns 1-10, RSI, MA5/20 ratios, vol ratio) và `build_enhanced_features()` (~30 features: lag returns 1-10, MA5/10/20/50 ratios, multi-timeframe returns 5/10/20d, RSI, StochRSI %K/%D, Bollinger %B, MACD line/hist normalized, rolling volatility 5/10/20d, ROC(10), momentum 5/10, volume ratio). Khi `pandas-ta` có sẵn thì dùng pandas-ta; nếu không dùng numpy-only fallback hoàn toàn tương đương. Minimum data: `MIN_DATA_POINTS = 80`.
- **Optuna hyperparameter tuning:** LightGBM và XGBoost chạy Optuna Bayesian search khi data >= 200 points và optuna được cài (`[ml]` extras). Search tối đa 30 trials, timeout 120s; fallback về `_DEFAULT_PARAMS` nếu optuna không có hoặc data không đủ. Search space — LightGBM: learning_rate, num_leaves, min_data_in_leaf, n_estimators, subsample, colsample_bytree. XGBoost: n_estimators, learning_rate, max_depth, subsample, colsample_bytree, min_child_weight. RandomForest không dùng Optuna (fixed: n_estimators=200, max_depth=8, min_samples_leaf=5).
- **Market calendar — đóng cửa cuối tuần/lễ NYSE:** NASDAQ và SP500 không chạy crawl/predict/bot-trade vào Thứ 7, Chủ nhật và ngày lễ NYSE (New Year's Day, MLK Day, Presidents' Day, Good Friday, Memorial Day, Juneteenth, Independence Day, Labor Day, Thanksgiving, Christmas). Logic tập trung tại `prediction/src/utils/market_calendar.py` — hàm `is_market_open(market_key, when)`, không phụ thuộc thư viện ngoài, tự tính ngày lễ theo năm. Ba điểm guard trong Python service: (1) `scheduler/jobs.py::_run_pipeline` — skip toàn bộ pipeline nếu market đóng; (2) `orchestrator/runner.py::run_for_market` — return 0 predictions và bỏ qua sim live step; (3) `simulation/engine.py::run_live_step` — lọc bỏ bot thuộc market đóng trong job bot 8PM hàng ngày. GOLD và CRYPTO không bị ảnh hưởng — luôn trả `True`.
- **Timezone — ICT-at-rest:** Toàn bộ cột `datetime` trong DB lưu theo múi giờ `Asia/Ho_Chi_Minh` (ICT, UTC+7) dưới dạng wallclock — **không dùng UTC**. MySQL container chạy UTC nhưng không ảnh hưởng vì kiểu cột là `datetime` (lưu verbatim, không có timezone conversion). Hai quy tắc bắt buộc: (1) **Python** luôn dùng `datetime.now()` — container được set `TZ=Asia/Ho_Chi_Minh` nên trả naive ICT. **Tuyệt đối không dùng `datetime.utcnow()`** — sẽ ghi UTC vào DB, lệch 7 tiếng so với giá trị nghiệp vụ. (2) **Go** giữ `loc=Asia%2FHo_Chi_Minh` trong DSN (xem `api/pkg/store/mysql/mysql.go`) và set `time.Local = Asia/Ho_Chi_Minh` trong `api/cmd/main.go` — mọi `time.Now()` và so sánh thời gian trong API Backend đều theo ICT. Frontend không cần xử lý đặc biệt: `new Date(s).toLocaleString()` hiển thị đúng giờ local của trình duyệt. Các cột vốn đã ICT và không bị ảnh hưởng: `prediction_date`, `target_date`, `trade_date`, `snapshot_date`, `trading_date`, `indicator_date`, `start_date`, intraday `timestamp`.

## Hướng dẫn mở rộng (Extension Guide)

### 1. Thêm thuật toán dự đoán mới

**Cần chạm vào 2 nơi** — một file Python (implementation) và một file Go (metadata registry):

1. Tạo `prediction/src/algorithms/<tên>.py` — implement abstract class:
   ```python
   from src.algorithms.base import PredictionAlgorithm, PredictionResult, get_max_change_pct

   class MyAlgorithm(PredictionAlgorithm):
       def predict(self, prices: list[float], volumes: list[float] | None = None) -> PredictionResult:
           current = prices[-1]
           predicted = ...  # tính toán
           # Bắt buộc: áp dụng market-aware clamp
           max_change = current * get_max_change_pct(self._market_key)
           predicted = max(current - max_change, min(current + max_change, predicted))
           return PredictionResult(predicted_price=predicted, confidence=0.5,
                                   current_price=current, algorithm_name=self.get_key())
       def get_name(self) -> str: return "My Algorithm"
       def get_key(self) -> str: return "my_algo"
   ```
2. Đăng ký trong `prediction/src/algorithms/registry.py` — thêm vào `build_algorithms()`.
3. Thêm metadata vào `api/pkg/service/predict/registry/algorithms.go` trong `init()`:
   ```go
   Register(AlgorithmDef{
       Key:         "my_algo",
       DisplayName: "My Algorithm",
       Config:      map[string]interface{}{"param": value},
   })
   ```
4. (Tuỳ chọn) Nếu muốn tham gia Ensemble, thêm instance vào `EnsemblePredictor` trong `ensemble.py`. Hiện tại Ensemble nhận đủ 10 base instances: `[ma, ema, lstm, arima, lgbm, sarima, egarch, gru, rf, xgb]`.
5. Nếu thuật toán là tree-based, có thể dùng `build_enhanced_features()` từ `features.py` thay vì tự implement feature engineering.
6. Thuật toán tự động xuất hiện trong API `/api/training/algorithms` (metadata từ Go registry) và được dùng trong tất cả prediction workflows của Python service.

**Lưu ý về `build_algorithms()`:** Python registry dùng two-pass: base algorithms trước, Ensemble cuối (nhận các base instances). Đảm bảo Ensemble luôn nhận đúng instance đang dùng.

### 2. Thêm thị trường hoặc loại tài sản dự đoán mới

Orchestrator Python (`prediction/src/orchestrator/runner.py`) tự động picks up market mới thông qua Python crawler registry.

Các bước bắt buộc:
1. Tạo DB model + migration trong `api/pkg/models/models_db/` (Go GORM struct) — Python ORM model tương ứng trong `prediction/src/database/models.py`.
2. Thêm repository methods trong `prediction/src/database/repository.py` và (nếu cần) trong Go `api/pkg/store/repository/repository.go` + `api/pkg/store/mysql/`.
3. Tạo crawler trong `prediction/src/crawlers/<tên>.py` — implement `BaseCrawler`.
4. Đăng ký cron job trong `prediction/src/scheduler/jobs.py`.
5. Thêm prediction logic vào orchestrator hoặc tạo market-specific predict function.
6. Thêm trigger endpoints mới trong Go API Backend nếu cần (`api/pkg/server/api_trigger_<tên>.go` + proto RPC mới).

## Database

- **ORM:** GORM v2
- **Tables chính:** `sync_logs`, `gold_prices`, `gold_predictions`, `nasdaq_prices`, `nasdaq_predictions`, `sp500_prices`, `sp500_predictions`, `crypto_prices`, `crypto_predictions`, `training_logs`, `training_metrics`, `users`, `cron_schedules`
- **Auto-migrate:** Chạy khi start app qua `api/pkg/models/models_db/migrations.go`
- **Schema đầy đủ:** `database.sql` ở root
- **direction_correct (nullable boolean):** Có mặt trong tất cả 4 prediction tables. Được set bởi `reconcile_predictions()` trong Python; `NULL` = chưa reconcile, `1` = hướng đúng, `0` = hướng sai. Dùng cho endpoint `/api/predictions/direction-accuracy`.

## Cron schedules

Lịch cron được lưu trong bảng `cron_schedules` và có thể chỉnh sửa live qua API `/api/schedules` hoặc Settings page trên frontend — **không cần restart service**. Prediction Service poll DB mỗi phút để phát hiện thay đổi và rescheduling tự động.

**Nguồn sự thật (source of truth) cho lịch mặc định là `DEFAULT_SCHEDULES` trong `prediction/src/scheduler/manager.py`.** Mỗi lần Python service khởi động, `upsert_cron_schedule()` trong `repository.py` chạy true upsert — cập nhật `cron_expression`, `job_name`, `enabled` trong DB nếu giá trị trong code khác với DB hiện tại. Giá trị do người dùng chỉnh sửa qua API sẽ bị ghi đè khi restart nếu khác với `DEFAULT_SCHEDULES`.

Go-side `seedCronSchedules()` trong `api/pkg/server/api_schedules.go` chỉ insert nếu row chưa tồn tại (không update) — chỉ dùng để seed các job key cũ (`crawler_daily`, `predict_daily`, `train_weekly`, `reconcile_daily`, `gold_crawler_daily`, `gold_predict_daily`, `simulation_daily`).

### Danh sách jobs hiện tại

| Job Key (DB) | Schedule mặc định | Enabled | Công việc |
|-------------|-------------------|---------|-----------|
| `daily_reconcile` | `0 0 6 * * *` | bật | Reconcile dự đoán với giá thực tế |
| `crawler_gold` | `0 0 * * * *` | bật | Pipeline Gold: crawl → train mỗi 10 lần → predict (mỗi giờ phút 0) |
| `crawler_nasdaq` | `0 15 * * * 1-5` | bật | Pipeline NASDAQ: crawl → train mỗi 10 lần → predict (mỗi giờ phút 15, chỉ T2-T6) |
| `crawler_sp500` | `0 0,30 * * * 1-5` | bật | Pipeline S&P 500: crawl → train mỗi 10 lần → predict (mỗi giờ phút 0 và 30, chỉ T2-T6) |
| `crawler_crypto` | `0 0 */2 * * *` | bật | Pipeline Crypto: crawl → train mỗi 10 lần → predict (mỗi 2 giờ) |
| `train_gold` | `0 0 3 * * 0` | bật | Training Gold (Chủ nhật 3AM) |
| `train_nasdaq` | `0 0 4 * * 0` | bật | Training NASDAQ (Chủ nhật 4AM) |
| `train_crypto` | `0 0 5 * * 0` | bật | Training Crypto (Chủ nhật 5AM) |
| `train_sp500` | `0 0 7 * * 0` | bật | Training S&P 500 (Chủ nhật 7AM) |
| `gold_predict` | `0 0 11 * * *` | **tắt** | Dự đoán vàng riêng lẻ — disabled vì đã chạy trong pipeline `crawler_gold` |
| `predict_nasdaq` | `0 30 23 * * 1-5` | **tắt** | Dự đoán NASDAQ riêng lẻ — disabled vì đã chạy trong pipeline `crawler_nasdaq` |
| `predict_crypto` | `0 0 */6 * * *` | **tắt** | Dự đoán Crypto riêng lẻ — disabled vì đã chạy trong pipeline `crawler_crypto` |
| `predict_sp500` | `0 0 13 * * 1-5` | **tắt** | Dự đoán S&P 500 riêng lẻ — disabled vì đã chạy trong pipeline `crawler_sp500` |
| `weekly_training` | `0 0 9 * * 0` | **tắt** | Huấn luyện toàn bộ tất cả markets — disabled (thay bằng per-market training jobs) |
| `daily_prediction` | `0 0 */1 * * *` | **tắt** | Dự đoán tất cả markets — disabled (thay bằng pipeline trong từng crawler job) |
| `simulation_daily` | `0 0 20 * * *` | bật | Bot trading hàng ngày (8PM) |
| `daily_backup` | `0 0 3 * * *` | bật | Backup MySQL database hàng ngày lúc 3AM |

### Pipeline logic

Bốn markets chạy pipeline (`crawler_gold`, `crawler_nasdaq`, `crawler_sp500`, `crawler_crypto`) đều dùng hàm `_run_pipeline()` trong `jobs.py`:
1. Crawl dữ liệu mới
2. Tăng counter per-market; mỗi 10 lần crawl → trigger `train_for_market()`
3. Chạy `run_for_market()` để sinh dự đoán mới

## Ports

| Service | Port | Protocol | Ghi chú |
|---------|------|----------|---------|
| Nginx | 80 | HTTP | Reverse proxy → API Backend |
| API Backend | 8118 | HTTP | Toàn bộ `/api/*` endpoints; Swagger UI tại `http://localhost:8118/swagger/` |
| Prediction Service | 8119 | gRPC | Internal only (không expose ra ngoài) |
| MySQL | 3306 | TCP | Docker |
| phpMyAdmin | 8081 | HTTP | Docker |
| Frontend | 36018 | HTTP | React app (Docker) |

## Docker Compose Services

| Service | Image/Dockerfile | Depends On | Ghi chú |
|---------|-----------------|------------|---------|
| `db` | `mysql:8.0` | — | Schema tự init từ `database.sql` |
| `prediction` | `prediction/Dockerfile` | `db` (healthy) | **Python** Prediction Service — gRPC :8119 (internal); mount volume `backup_data:/backups` |
| `api` | `api/Dockerfile` | `db` (healthy), `prediction` | Go API Backend — HTTP :8118 (internal) |
| `nginx` | `nginx:alpine` | `api` | Reverse proxy — expose :80 |
| `frontend` | `frontend/Dockerfile` | `api` | React app — expose :36018 |
| `phpmyadmin` | `phpmyadmin/phpmyadmin` | `db` | Admin UI — expose :8081 |

## API Endpoints

### Auth

| Method | Path | Ghi chú |
|--------|------|---------|
| `POST` | `/api/auth/login` | Đăng nhập — body: `{"username":"","password":""}`, query DB + bcrypt verify, trả JWT 24h với claims `sub`, `role`, `user_id`; response: `{"token":"...","user":{"username":"...","role":"..."}}` |
| `GET` | `/api/auth/me` | Xác minh token — header: `Authorization: Bearer <token>`, trả `{"username":"...","role":"..."}` hoặc 401 |

### User Management

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/users` | Danh sách tất cả users — yêu cầu admin JWT; trả `[{"id":1,"username":"...","role":"...","created_at":"..."}]` |
| `POST` | `/api/users` | Tạo user mới — yêu cầu admin JWT; body: `{"username":"","password":"","role":"user\|admin"}`; role mặc định `"user"` nếu không hợp lệ; trả 409 nếu username đã tồn tại |
| `DELETE` | `/api/users/{id}` | Xóa user theo ID — yêu cầu admin JWT; trả 400 nếu tự xóa chính mình |

### Predictions

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/predictions/direction-accuracy` | Direction accuracy per algorithm — query: `?market=GOLD\|NASDAQ\|SP500\|CRYPTO`; trả `{market, algorithms:[{algorithm, direction_accuracy, total, correct}]}`; chỉ đếm rows có `direction_correct IS NOT NULL` |

### Training

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/training/status` | Trạng thái huấn luyện hiện tại |
| `GET` | `/api/training/history` | Lịch sử các phiên huấn luyện |
| `GET` | `/api/training/algorithms` | Chi tiết từng thuật toán: config, accuracy, last_trained, training_time |
| `GET` | `/api/training/metrics` | Aggregate metrics: avg time, data quality, success rate |
| `GET` | `/api/training/{id}` | Chi tiết một phiên huấn luyện |

### Markets (per-market paginated)

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/markets/{key}/predictions` | Danh sách dự đoán phân trang cho market — key: `gold`, `nasdaq`, `crypto`, `sp500` |
| `GET` | `/api/markets/{key}/training` | Danh sách training sessions cho market |

### Gold

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/gold/latest` | Giá vàng mới nhất |
| `GET` | `/api/gold/prices` | Danh sách giá vàng |
| `GET` | `/api/gold/chart` | Dữ liệu biểu đồ vàng |
| `GET` | `/api/gold/predictions/latest-results` | Kết quả dự đoán mới nhất |
| `GET` | `/api/gold/predictions/latest` | Dự đoán vàng mới nhất |
| `GET` | `/api/gold/predictions/chart` | Biểu đồ dự đoán vs thực tế (vàng) |
| `GET` | `/api/gold/predictions` | Danh sách dự đoán vàng |

### NASDAQ

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/nasdaq/latest` | Giá NASDAQ mới nhất |
| `GET` | `/api/nasdaq/prices` | Danh sách giá NASDAQ |
| `GET` | `/api/nasdaq/chart` | Dữ liệu biểu đồ NASDAQ |
| `GET` | `/api/nasdaq/predictions/latest` | Dự đoán NASDAQ mới nhất |
| `GET` | `/api/nasdaq/predictions/chart` | Biểu đồ dự đoán vs thực tế (NASDAQ) |
| `GET` | `/api/nasdaq/predictions/latest-results` | Kết quả dự đoán mới nhất |
| `GET` | `/api/nasdaq/predictions` | Danh sách dự đoán NASDAQ |

### Crypto

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/crypto/latest` | Giá Crypto mới nhất |
| `GET` | `/api/crypto/prices` | Danh sách giá Crypto |
| `GET` | `/api/crypto/chart` | Dữ liệu biểu đồ Crypto |
| `GET` | `/api/crypto/predictions/latest` | Dự đoán Crypto mới nhất |
| `GET` | `/api/crypto/predictions/chart` | Biểu đồ dự đoán vs thực tế (Crypto) |
| `GET` | `/api/crypto/predictions/latest-results` | Kết quả dự đoán mới nhất |
| `GET` | `/api/crypto/predictions` | Danh sách dự đoán Crypto |

### S&P 500

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/sp500/latest` | Giá S&P 500 mới nhất |
| `GET` | `/api/sp500/prices` | Danh sách giá S&P 500 |
| `GET` | `/api/sp500/chart` | Dữ liệu biểu đồ S&P 500 |
| `GET` | `/api/sp500/predictions/latest` | Dự đoán S&P 500 mới nhất |
| `GET` | `/api/sp500/predictions/chart` | Biểu đồ dự đoán vs thực tế (S&P 500) |
| `GET` | `/api/sp500/predictions/latest-results` | Kết quả dự đoán mới nhất |
| `GET` | `/api/sp500/predictions` | Danh sách dự đoán S&P 500 |

### Dashboard

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/dashboard/stats` | Thống kê tổng quan dashboard |

### Monitoring

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/monitoring/overview` | Pipeline monitoring overview — yêu cầu JWT; trả `{generated_at, markets:[{market, crawl:{last_crawl_at, staleness, stale, daily_today, intraday_today}, predictions:{last_predict_at, staleness, today_total, expected_algos, missing_today, algorithms:[{algorithm, today_count, direction_accuracy, reconciled, correct}]}}], bots:{summary:{total_bots, active_bots, by_market:[...]}, table:[{bot_id, market, algorithm, trades, wins, losses, breakeven, win_rate, total_pnl, return_pct, profit_factor}]}}`; cache 30s; staleness "stale" khi last crawl > 3h hoặc không có data hôm nay |

### Schedules

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/schedules` | Danh sách lịch tác vụ — yêu cầu JWT; trả `[{"job_key":"...","job_name":"...","cron_expression":"...","enabled":true,"updated_at":"..."}]` |
| `PUT` | `/api/schedules/{key}` | Cập nhật lịch tác vụ — yêu cầu JWT; body: `{"cron_expression":"0 0 12 * * *","enabled":true}`; validate cron expression trước khi lưu |

### Backup

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/backups` | Danh sách file backup — yêu cầu JWT; trả `[{filename, size, size_human, created_at}]` |
| `POST` | `/api/trigger/backup` | Tạo backup ngay (mysqldump → gzip) — yêu cầu admin JWT; trả `{filename, size, message}`; giữ 10 backup gần nhất |
| `GET` | `/api/backups/{filename}` | Tải xuống file backup — yêu cầu JWT; stream file .sql.gz |
| `DELETE` | `/api/backups/{filename}` | Xóa file backup — yêu cầu admin JWT |

### Trigger

Tất cả trigger endpoint đều yêu cầu JWT authentication (`Authorization: Bearer <token>`), được enforce qua `AuthRequired()` middleware wrapper trong router.

| Method | Path | Ghi chú |
|--------|------|---------|
| `POST` | `/api/trigger/train` | Huấn luyện toàn bộ hoặc một thuật toán — body JSON `{"algorithm":"lstm_nn"}` (optional) — yêu cầu JWT |
| `POST` | `/api/trigger/gold-crawler` | Crawl giá vàng đồng bộ — yêu cầu JWT |
| `POST` | `/api/trigger/gold-history` | Import lịch sử XAU (background) — yêu cầu JWT |
| `POST` | `/api/trigger/gold-predict` | Dự đoán vàng (background) — yêu cầu JWT |
| `POST` | `/api/trigger/reconcile` | Reconcile dự đoán với giá thực tế — yêu cầu JWT |
| `POST` | `/api/trigger/historical-backtest` | Walk-forward backtest (background) — query: `?train_window=30&step_size=6&market_key=GOLD\|NASDAQ100\|CRYPTO\|SP500\|ALL`; trả 202; 409 nếu đang chạy — yêu cầu JWT |
| `POST` | `/api/trigger/gold-historical-backtest` | Walk-forward backtest chỉ Gold (background) — yêu cầu JWT |
| `POST` | `/api/trigger/nasdaq-crawler` | Crawl NASDAQ (background) — yêu cầu JWT |
| `POST` | `/api/trigger/nasdaq-predict` | Dự đoán NASDAQ (background) — yêu cầu JWT |
| `POST` | `/api/trigger/crypto-crawler` | Crawl Crypto (background) — yêu cầu JWT |
| `POST` | `/api/trigger/crypto-predict` | Dự đoán Crypto (background) — yêu cầu JWT |
| `POST` | `/api/trigger/sp500-crawler` | Crawl S&P 500 (background) — yêu cầu JWT |
| `POST` | `/api/trigger/sp500-predict` | Dự đoán S&P 500 (background) — yêu cầu JWT |
| `POST` | `/api/trigger/simulation-backtest` | Chạy simulation backtest — yêu cầu JWT |
| `POST` | `/api/trigger/simulation-live-step` | Chạy một bước live simulation — yêu cầu JWT |
| `POST` | `/api/trigger/sim-reset` | Reset tất cả active bots — đóng live session cũ, tạo session mới — yêu cầu JWT |

## Trigger thủ công qua API

Tất cả trigger endpoint đều gọi gRPC sang Prediction Service. Tất cả đều yêu cầu JWT (`Authorization: Bearer <token>`). Lấy token qua `POST /api/auth/login` trước.

```bash
# Lấy JWT token
TOKEN=$(curl -s -X POST http://localhost:8118/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.token')

# Huấn luyện mô hình ngay (tất cả thuật toán)
curl -X POST http://localhost:8118/api/trigger/train -H "Authorization: Bearer $TOKEN"

# Huấn luyện một thuật toán cụ thể
curl -X POST http://localhost:8118/api/trigger/train -H "Authorization: Bearer $TOKEN" -d '{"algorithm":"lstm_nn"}'

# Crawl vàng ngay
curl -X POST http://localhost:8118/api/trigger/gold-crawler -H "Authorization: Bearer $TOKEN"

# Import lịch sử vàng (XAU)
curl -X POST http://localhost:8118/api/trigger/gold-history -H "Authorization: Bearer $TOKEN"

# Chạy dự đoán vàng ngay
curl -X POST http://localhost:8118/api/trigger/gold-predict -H "Authorization: Bearer $TOKEN"

# Reconcile dự đoán với giá thực tế
curl -X POST http://localhost:8118/api/trigger/reconcile -H "Authorization: Bearer $TOKEN"

# Chạy historical backtest (query params, không phải body)
curl -X POST "http://localhost:8118/api/trigger/historical-backtest?train_window=30&step_size=6" -H "Authorization: Bearer $TOKEN"

# Crawl NASDAQ ngay
curl -X POST http://localhost:8118/api/trigger/nasdaq-crawler -H "Authorization: Bearer $TOKEN"

# Crawl Crypto ngay
curl -X POST http://localhost:8118/api/trigger/crypto-crawler -H "Authorization: Bearer $TOKEN"

# Crawl S&P 500 ngay
curl -X POST http://localhost:8118/api/trigger/sp500-crawler -H "Authorization: Bearer $TOKEN"

# Chạy dự đoán S&P 500 ngay
curl -X POST http://localhost:8118/api/trigger/sp500-predict -H "Authorization: Bearer $TOKEN"

# Backtest tất cả markets
curl -X POST "http://localhost:8118/api/trigger/historical-backtest?train_window=30&step_size=6&market_key=ALL" -H "Authorization: Bearer $TOKEN"

# Xem lịch cron
curl http://localhost:8118/api/schedules -H "Authorization: Bearer $TOKEN"

# Cập nhật lịch cron (ví dụ: đổi giờ crawl gold sang 2AM)
curl -X PUT http://localhost:8118/api/schedules/crawler_gold \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"cron_expression":"0 0 2 * * *","enabled":true}'
```

## gRPC Service Contract

Định nghĩa trong `api/proto/prediction/prediction.proto`. Prediction Service implement tất cả RPC; API Backend gọi chúng qua gRPC client.

| RPC | Request | Response | Ghi chú |
|-----|---------|----------|---------|
| `TriggerGoldCrawler` | `Empty` | `TriggerResponse` | Crawl giá vàng đồng bộ |
| `TriggerTrain` | `TriggerTrainRequest` | `TriggerTrainResponse` | Train một hoặc tất cả thuật toán — field `algorithm` optional |
| `TriggerReconcile` | `Empty` | `TriggerResponse` | Reconcile dự đoán với giá thực tế đồng bộ |
| `TriggerGoldHistory` | `Empty` | `TriggerResponse` | Import lịch sử XAU trong background |
| `TriggerGoldPredict` | `Empty` | `TriggerResponse` | Dự đoán vàng trong background |
| `TriggerHistoricalBacktest` | `BacktestRequest` | `TriggerResponse` | Walk-forward backtest trong background — concurrency guard; `market_key`: `""` hoặc `"GOLD"`, `"NASDAQ100"`, `"CRYPTO"`, `"SP500"`, `"ALL"` |
| `TriggerNasdaqCrawler` | `Empty` | `TriggerResponse` | Crawl NASDAQ trong background |
| `TriggerNasdaqPredict` | `Empty` | `TriggerResponse` | Dự đoán NASDAQ trong background |
| `TriggerCryptoCrawler` | `Empty` | `TriggerResponse` | Crawl Crypto trong background |
| `TriggerCryptoPredict` | `Empty` | `TriggerResponse` | Dự đoán Crypto trong background |
| `TriggerSP500Crawler` | `Empty` | `TriggerResponse` | Crawl S&P 500 trong background |
| `TriggerSP500Predict` | `Empty` | `TriggerResponse` | Dự đoán S&P 500 trong background |
| `TriggerSimulationBacktest` | `SimulationRequest` | `TriggerResponse` | Chạy simulation backtest trong background |
| `TriggerSimulationLiveStep` | `Empty` | `TriggerResponse` | Chạy một bước live simulation |
| `ResetSimBots` | `Empty` | `TriggerResponse` | Đóng live sessions cũ, tạo fresh live session cho tất cả active bots |
| `GetTrainingStatus` | `Empty` | `TrainingStatusResponse` | Trạng thái training: `is_training`, `progress`, `phase`, `total/done algorithms` |

## Test

```bash
# Chạy tất cả test — chạy từ trong thư mục api/
cd api && make test
# hoặc
cd api && go test ./...

# Chỉ unit test (bỏ qua integration)
cd api && make test-unit

# Xem coverage
cd api && make test-coverage   # tạo coverage.html

# Khởi tạo test DB (MySQL port 3307)
cd api && make test-db-up
cd api && make test-db-down
```

**Test coverage hiện tại:**

Go API Backend:
- `api/pkg/server/` — API handlers (mock pattern, không cần DB thật)
- `api/pkg/testutil/` — Test helpers

Python Prediction Service (`prediction/tests/`):
- `tests/unit/` — Unit tests cho algorithms (moving_average, ema_macd, lstm, gru, arima_garch, egarch, sarima, lightgbm_model, xgboost_model, random_forest, ensemble, features, simulation_metrics, simulation_portfolio, simulation_signal) và `test_market_calendar.py` (25 tests — weekday/weekend/holiday logic cho NASDAQ/SP500/GOLD/CRYPTO)
- `tests/integration/` — End-to-end: gRPC contract, crawlers, prediction pipeline, training, schedules, performance benchmarks, service resilience
- Phase 5 final: **71 passed, 0 failed** (`make test-phase5`)

**Convention test (Go):**
- File test đặt cùng package: `algo.go` → `algo_test.go`
- Integration tests có guard: `if testing.Short() { t.Skip() }`
- API handler tests dùng `recover` pattern (không inject mock DB — xem `api/pkg/server/*_test.go`)

**Convention test (Python):**
- Unit tests: `tests/unit/test_<module>.py` — không cần DB, dùng mock data
- Integration tests: `tests/integration/test_<phase>.py` — cần Docker stack chạy
- Optional-dep tests: dùng `pytest.importorskip("torch")` để skip gracefully nếu thiếu thư viện
- Chạy trong container: `docker exec prediction_service python -m pytest tests/ -v`
