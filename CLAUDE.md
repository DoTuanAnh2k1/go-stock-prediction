# CLAUDE.md

## Tổng quan dự án

Hệ thống dự đoán giá cổ phiếu Việt Nam. Thu thập dữ liệu từ nhiều nguồn (VN30, Gold SJC/XAU, NASDAQ, Crypto, Fuel), chạy 6 thuật toán ML (Moving Average, EMA, LSTM PyTorch, ARIMA-GARCH, LightGBM, Ensemble), và hiển thị kết quả qua web dashboard.

**Kiến trúc hiện tại: microservice (2 service + nginx)**
- **API Backend** (`cmd/api`) — Go HTTP trên `:8118`, phục vụ toàn bộ `/api/*`. Gọi Prediction Service qua gRPC để trigger crawler/training; đọc DB trực tiếp cho các query dữ liệu.
- **Prediction Service** (`prediction/`) — **Python** gRPC trên `:8119`. Xử lý crawling (VN30, Gold, NASDAQ, Crypto, Fuel), 6 thuật toán ML, training, APScheduler cron jobs.
- **Nginx** — Reverse proxy trên `:80`, forward tất cả request về API Backend.


## Lệnh thường dùng

```bash
# Build API Backend (Go)
go build -o api-server ./cmd/api

# Start tất cả services (DB + Python Prediction + API + Nginx + Frontend + phpMyAdmin)
docker-compose up -d

# Import schema
mysql -u root -p go_stock_prediction < database.sql

# Regenerate proto Go stubs (cần protoc + plugins)
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/prediction/prediction.proto

# Regenerate proto Python stubs (chạy trong Docker hoặc local với grpcio-tools)
cd prediction
python -m grpc_tools.protoc -Iproto --python_out=src/proto --grpc_python_out=src/proto proto/prediction/prediction.proto

# Generate Swagger docs (cần swag CLI: go install github.com/swaggo/swag/v2/cmd/swag@latest)
swag init -g cmd/api/main.go -o docs/

# Chạy test Python prediction service
cd prediction && make test
# hoặc chạy trong container đang chạy:
docker exec prediction_service python -m pytest tests/ -v
```

## Cấu trúc thư mục quan trọng

### Go API Backend

```
cmd/api/main.go                         # API Backend entry point — config → timezone → logger → repository → grpcclient → server
pkg/config/                             # Load .env, trả về config struct toàn cục (bao gồm GRPCConfig)
pkg/grpc/client/client.go               # gRPC client singleton — API Backend dùng để gọi Python Prediction Service
proto/prediction/prediction.proto       # gRPC service definitions (shared với Python service)
proto/prediction/*.pb.go                # Generated Go protobuf stubs (không sửa tay)
docs/swagger.json                       # Generated Swagger spec (không sửa tay — chạy swag init)
docs/swagger.yaml                       # Generated Swagger YAML spec
docs/docs.go                            # Generated Go package cho swagger
nginx/nginx.conf                        # Nginx reverse proxy: :80 → api:8118
Dockerfile.api                          # Docker build cho API Backend (Go)
pkg/server/router.go                    # Đăng ký tất cả routes (API)
pkg/server/api_*.go                     # Mỗi file = một nhóm API endpoint
pkg/server/api_prediction_compare.go    # GET /api/predictions/compare/{symbol} và /api/predictions/error-distribution
pkg/server/api_training_algorithms.go   # GET /api/training/algorithms
pkg/server/api_training_metrics.go      # GET /api/training/metrics
pkg/server/api_gold.go                  # GET /api/gold/latest, /api/gold/prices, /api/gold/chart
pkg/server/api_gold_prediction.go       # GET /api/gold/predictions/latest, /api/gold/predictions/chart, /api/gold/predictions
pkg/server/api_training_status.go       # GET /api/training/status (gọi gRPC GetTrainingStatus)
pkg/server/api_stock_detail.go          # GET /api/stocks/{symbol}/detail
pkg/server/api_stock_actions.go         # POST /api/stocks/{symbol}/crawl, POST /api/stocks/{symbol}/predict
pkg/server/api_dashboard_stats.go       # GET /api/dashboard/stats
pkg/server/api_accuracy_trend.go        # GET /api/predictions/accuracy-trend
pkg/server/api_trigger_crawler.go       # POST /api/trigger/crawler (→ gRPC TriggerCrawler)
pkg/server/api_trigger_predict.go       # POST /api/trigger/predict (→ gRPC TriggerPredict)
pkg/server/api_trigger_train.go         # POST /api/trigger/train (→ gRPC TriggerTrain)
pkg/server/api_trigger_gold_crawler.go  # POST /api/trigger/gold-crawler (→ gRPC TriggerGoldCrawler)
pkg/server/api_trigger_gold_history.go  # POST /api/trigger/gold-history (→ gRPC TriggerGoldHistory)
pkg/server/api_trigger_reconcile.go     # POST /api/trigger/reconcile (→ gRPC TriggerReconcile)
pkg/server/api_trigger_stock_history.go # POST /api/trigger/stock-history (→ gRPC TriggerStockHistory)
pkg/server/api_trigger_historical_backtest.go # POST /api/trigger/historical-backtest (→ gRPC TriggerHistoricalBacktest, trả 202, 409 nếu đang chạy)
pkg/server/api_auth.go                  # POST /api/auth/login (DB + bcrypt), GET /api/auth/me — JWT authentication
pkg/server/api_users.go                 # GET/POST /api/users, DELETE /api/users/{id} — quản lý user (admin only)
pkg/server/api_schedules.go             # GET /api/schedules, PUT /api/schedules/{key} — quản lý lịch cron động
pkg/server/middleware_jwt.go            # JWTMiddleware (non-blocking, inject claims vào context), getClaims(), requireAuth(), requireAdmin(), AuthRequired()
pkg/server/helper.go                    # requireGRPCClient(), ResponseError(), ResponseSuccess() và các helper
pkg/service/predict/registry/registry.go   # AlgorithmDef struct (metadata only — không có Factory), Register(), All()
pkg/service/predict/registry/algorithms.go # FILE DUY NHẤT cần sửa khi thêm/bỏ thuật toán trong metadata registry
pkg/store/repository/repository.go     # Interface DatabaseStore (composite) — bao gồm GetConfirmedPredictionsPage, DeletePredictionsBeforeDate, BulkCreatePredictions, CronScheduleStore
pkg/store/repository/user.go           # UserStore interface — CreateUser, GetUserByUsername, GetUserByID, GetAllUsers, DeleteUser, AdminExists
pkg/store/mysql/                        # Triển khai MySQL dùng GORM
pkg/store/mysql/user.go                 # MySQL implementation của UserStore
pkg/store/mysql/cron_schedule.go        # MySQL implementation của CronScheduleStore — GetAllCronSchedules, GetCronScheduleByKey, UpsertCronSchedule
pkg/models/models_db/                   # GORM struct: Stock, StockPrice, Prediction, SyncLog, Exchange, GoldPrice, TrainingLog, TrainingMetrics, User, CronSchedule
pkg/models/models_db/cron_schedule.go   # CronSchedule GORM struct (JobKey, JobName, CronExpression, Enabled, UpdatedAt)
pkg/models/models_db/user.go            # User GORM struct (Username, PasswordHash, Role)
pkg/models/models_db/gold_price.go      # GoldPrice GORM struct
pkg/models/models_db/training_log.go    # TrainingLog GORM struct
pkg/models/models_db/training_metrics.go # TrainingMetrics struct
pkg/models/models_api/                  # DTO cho JSON response
pkg/models/models_config/config.go      # Config struct — bao gồm GRPCConfig (ServerPort, ClientTarget) và ServerConfig (AdminUsername, AdminPassword, JWTSecret)
pkg/utils/cron/                         # Hằng số cron schedule + wrapper (dùng làm giá trị mặc định khi seed DB)
web/templates/                          # HTML templates (Go's html/template)
web/static/js/                          # Frontend JS — AJAX gọi các /api/* endpoint
frontend/src/context/AuthContext.tsx    # AuthProvider + useAuth hook — quản lý JWT trong localStorage
frontend/src/components/LoginModal.tsx  # Login modal component — gọi POST /api/auth/login
frontend/src/pages/Users.tsx            # Trang quản lý user — chỉ hiển thị với role admin
```

### Python Prediction Service

```
prediction/
├── Dockerfile                          # Multi-stage build: proto-builder (grpcio-tools) → python:3.12-slim runtime
├── pyproject.toml                      # Python dependencies (torch, statsmodels, lightgbm, grpcio, APScheduler, SQLAlchemy...)
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
│   │   ├── base.py                     # Abstract PredictionAlgorithm interface
│   │   ├── registry.py                 # Algorithm registry (Register, All, build_algorithms)
│   │   ├── moving_average.py           # VWMA + RSI + Bollinger
│   │   ├── ema_macd.py                 # EMA/MACD(12,26,9) với signal line
│   │   ├── lstm.py                     # PyTorch LSTM (2 layers, hidden=64, seq=60, dropout=0.2)
│   │   ├── arima_garch.py              # statsmodels ARIMA(2,1,2) + arch GARCH(1,1)
│   │   ├── lightgbm_model.py           # LightGBM (lag returns 1-10, RSI, MA ratios, n_estimators=200)
│   │   └── ensemble.py                 # Equal-weight ensemble of all base models
│   ├── crawlers/
│   │   ├── base.py                     # Abstract BaseCrawler
│   │   ├── vn30.py                     # VNDirect API — 30 stocks HOSE
│   │   ├── gold.py                     # Yahoo Finance XAU + BTMC API + Phú Quý
│   │   ├── nasdaq.py                   # Yahoo Finance — 15 NASDAQ symbols
│   │   ├── crypto.py                   # CoinGecko — BTC/ETH
│   │   └── fuel.py                     # giaxanghomnay.com/api/pvdate + /api/chart
│   ├── scheduler/
│   │   ├── manager.py                  # APScheduler + DB-backed CronSchedule; poll mỗi 60s để phát hiện thay đổi
│   │   └── jobs.py                     # Định nghĩa tất cả jobs (crawlers + predictions + training + reconcile)
│   ├── orchestrator/
│   │   └── runner.py                   # RunAllMarkets(ctx), RunForMarket(ctx, key)
│   └── utils/
│       ├── logger.py                   # structlog config
│       ├── timezone.py                 # Asia/Ho_Chi_Minh helpers
│       └── number_parser.py            # Vietnamese number format (1.234,56 → 1234.56)
├── tests/
│   ├── conftest.py                     # Fixtures: DB session, gRPC stub, test data
│   ├── unit/                           # Unit tests cho từng algorithm
│   └── integration/                    # End-to-end tests qua gRPC và HTTP API
└── proto/
    └── prediction/
        └── prediction_pb2*.py          # Generated Python stubs (không sửa tay — tái sinh trong Dockerfile stage 1)
```

## Luồng khởi động

### API Backend (cmd/api/main.go) — Go

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
7. Startup data sync sau 5 giây delay — chạy `VN30Crawler().crawl()` một lần (daemon thread)
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
BACKUP_DIR=/backups              # Thư mục lưu file backup mysqldump (default: /backups, được mount qua Docker volume)
```

## Conventions trong codebase

- **Repository pattern (Go):** Mọi truy cập DB từ API Backend phải qua interface `DatabaseStore` trong `pkg/store/repository/`. Không gọi GORM trực tiếp từ service layer.
- **Singleton:** `repository.GetSingleton()` trả về instance DB toàn cục đã init.
- **Cron constants:** Hằng số trong `pkg/utils/cron/` dùng làm giá trị mặc định khi `seedCronSchedules()`. Lịch chạy thực tế lưu trong bảng `cron_schedules` và có thể chỉnh sửa live.
- **Decimal:** Dùng `shopspring/decimal` trong Go API Backend cho mọi phép tính số thực liên quan đến giá — tránh float64. Python service dùng `Decimal` từ stdlib hoặc pandas float64 (được làm tròn trước khi lưu DB).
- **API handlers (Go):** Mỗi nhóm endpoint có file riêng `api_<topic>.go` trong `pkg/server/`.
- **Algorithms (Python):** Mỗi thuật toán implement abstract class `PredictionAlgorithm` trong `prediction/src/algorithms/base.py` với method `predict(data: StockData) -> Prediction`. Đăng ký metadata tương ứng trong `pkg/service/predict/registry/algorithms.go` (Go) để `/api/training/algorithms` trả đúng danh sách.
- **Logging:** Go API Backend dùng `pkg/logger` (zerolog). Python service dùng `structlog`.
- **gRPC triggers:** Tất cả trigger handler trong `pkg/server/api_trigger_*.go` và `pkg/server/api_stock_actions.go` đều gọi `requireGRPCClient(w)` trước. Hàm này trả về 503 nếu gRPC client chưa init. Tất cả trigger endpoints được wrap bằng `AuthRequired()` trong router — yêu cầu JWT hợp lệ.
- **Dynamic cron schedules:** Lịch cron được lưu trong bảng `cron_schedules`. Khi startup, `seedCronSchedules()` trong `pkg/server/api_schedules.go` (Go) tạo các hàng mặc định nếu chưa tồn tại. Python Prediction Service poll DB mỗi 60 giây để phát hiện thay đổi và tự reschedule qua APScheduler — không cần restart. Dùng `CronScheduleStore` interface (Go) để truy cập từ API Backend.
- **Proto regeneration:** Khi thay đổi `proto/prediction/prediction.proto`, cần tái sinh cả Go stubs (`protoc`) lẫn Python stubs (lệnh `grpc_tools.protoc` trong Dockerfile stage 1). Không sửa tay các file generated.
- **Data ordering — QUAN TRỌNG:** DB trả `stock_prices` với `ORDER BY trading_date DESC` (mới nhất trước). Python algorithms cần đảo ngược về ASC trước khi build feature sequences. Repository (Python) trả DESC — tầng algorithm tự xử lý (tương tự pattern Go cũ với `reverseStockPrices()`).
- **JWT middleware — non-blocking:** `JWTMiddleware` trong `pkg/server/middleware_jwt.go` nằm trong middleware chain `CORS → RateLimit → JWT → mux`. Middleware này chỉ inject claims vào context nếu token hợp lệ — request không có token vẫn tiếp tục (unauthenticated). Các handler bảo vệ dùng `requireAuth(w, r)` hoặc `requireAdmin(w, r)` để enforce.
- **Admin seeder:** Khi startup, `seedAdminUser()` trong `cmd/api/main.go` kiểm tra `AdminExists()`. Nếu chưa có user nào với `role="admin"`, tạo một user mới từ `ADMIN_USERNAME`/`ADMIN_PASSWORD` env vars với bcrypt hash. Chạy một lần duy nhất — các lần sau bỏ qua nếu admin đã tồn tại.
- **User management:** `ADMIN_USERNAME`/`ADMIN_PASSWORD` trong `.env` chỉ dùng để seed lần đầu. Sau đó quản lý user hoàn toàn qua API `/api/users` (admin JWT required). Password lưu dưới dạng bcrypt hash — không lưu plaintext.
- **Swagger annotations:** Mỗi handler function trong `pkg/server/api_*.go` có swaggo annotations (`@Summary`, `@Tags`, `@Param`, `@Success`, `@Router`). Khi thêm handler mới, phải thêm annotations. Sau khi thêm/sửa annotations, chạy `swag init -g cmd/api/main.go -o docs/` để regenerate. Không sửa tay files trong `docs/`.

## Hướng dẫn mở rộng (Extension Guide)

### 1. Thêm thuật toán dự đoán mới

**Cần chạm vào 2 nơi** — một file Python (implementation) và một file Go (metadata registry):

1. Tạo `prediction/src/algorithms/<tên>.py` — implement abstract class:
   ```python
   class MyAlgorithm(PredictionAlgorithm):
       def predict(self, data: StockData) -> Prediction:
           ...
       def get_name(self) -> str: return "my_algo"
       def get_accuracy(self) -> float: return 0.0
   ```
2. Đăng ký trong `prediction/src/algorithms/registry.py` — thêm vào `build_algorithms()`.
3. Thêm metadata vào `pkg/service/predict/registry/algorithms.go` trong `init()`:
   ```go
   Register(AlgorithmDef{
       Key:         "my_algo",
       DisplayName: "My Algorithm",
       Config:      map[string]interface{}{"param": value},
   })
   ```
4. (Tuỳ chọn) Nếu muốn tham gia Ensemble, thêm instance vào `EnsemblePredictor` trong `ensemble.py`.
5. Thuật toán tự động xuất hiện trong API `/api/training/algorithms` (metadata từ Go registry) và được dùng trong tất cả prediction workflows của Python service.

**Lưu ý về `build_algorithms()`:** Python registry dùng two-pass tương tự Go: base algorithms trước, Ensemble cuối (nhận các base instances). Đảm bảo Ensemble luôn nhận đúng instance đang dùng.

### 2. Thêm thị trường hoặc loại tài sản dự đoán mới

Orchestrator Python (`prediction/src/orchestrator/runner.py`) tự động picks up market mới thông qua Python crawler registry.

Các bước bắt buộc:
1. Tạo DB model + migration trong `pkg/models/models_db/` (Go GORM struct) — Python ORM model tương ứng trong `prediction/src/database/models.py`.
2. Thêm repository methods trong `prediction/src/database/repository.py` và (nếu cần) trong Go `pkg/store/repository/repository.go` + `pkg/store/mysql/`.
3. Tạo crawler trong `prediction/src/crawlers/<tên>.py` — implement `BaseCrawler`.
4. Đăng ký cron job trong `prediction/src/scheduler/jobs.py`.
5. Thêm prediction logic vào orchestrator hoặc tạo market-specific predict function.
6. Thêm trigger endpoints mới trong Go API Backend nếu cần (`pkg/server/api_trigger_<tên>.go` + proto RPC mới).

## Database

- **ORM:** GORM v2
- **Tables chính:** `exchanges`, `stocks`, `stock_prices`, `predictions`, `sync_logs`, `gold_prices`, `training_logs`, `users`, `cron_schedules`
- **Auto-migrate:** Chạy khi start app qua `models_db/migrations.go`
- **Schema đầy đủ:** `database.sql` ở root

## Cron schedules

Lịch cron được lưu trong bảng `cron_schedules` và có thể chỉnh sửa live qua API `/api/schedules` hoặc Settings page trên frontend — **không cần restart service**. Prediction Service poll DB mỗi phút để phát hiện thay đổi và rescheduling tự động.

Khi lần đầu startup, `seedCronSchedules()` trong `pkg/server/api_schedules.go` tạo các hàng mặc định nếu chưa tồn tại.

| Job Key (DB) | Schedule mặc định | Công việc |
|-------------|-------------------|-----------|
| `reconcile_daily` | `0 0 6 * * *` | Reconcile dự đoán với giá thực tế |
| `gold_crawler_daily` | `0 0 10 * * *` | Crawl giá vàng SJC và XAU/USD |
| `crawler_daily` | `0 0 12 * * *` | Crawl dữ liệu giá cổ phiếu (VN30) |
| `predict_daily` | `0 0 18 * * *` | Chạy dự đoán cho TẤT CẢ markets qua `orchestrator.RunAllMarkets()` |
| `train_weekly` | `0 0 9 * * SUN` | Huấn luyện mô hình |
| `db_backup_daily` | `0 0 2 * * *` | Backup database (mysqldump → `BACKUP_DIR`) — hiện chạy trong Python service |
| `backup_cleanup_daily` | `0 0 3 * * *` | Xóa backup cũ hơn 7 ngày trong `BACKUP_DIR` — hiện chạy trong Python service |

Hằng số cron trong `pkg/utils/cron/` vẫn được dùng làm giá trị mặc định khi seed Go-side. Python APScheduler đọc expression từ bảng `cron_schedules` và parse cùng format.

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
| `api` | `Dockerfile.api` | `db` (healthy), `prediction` | Go API Backend — HTTP :8118 (internal) |
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
| `GET` | `/api/predictions` | Danh sách dự đoán — query: `?status=confirmed` dùng `GetConfirmedPredictionsPage` (default from 6 tháng), mặc định from 30 ngày; response gồm `actual_price`, `accuracy`, `status` |
| `GET` | `/api/predictions/accuracy` | Độ chính xác theo thuật toán |
| `GET` | `/api/predictions/accuracy-trend` | Xu hướng độ chính xác theo thời gian |
| `GET` | `/api/predictions/compare/{symbol}` | Cặp giá dự đoán vs thực tế theo thời gian — query: `?days=30&algorithm=lstm_nn` |
| `GET` | `/api/predictions/error-distribution` | Scatter plot: predicted change % vs actual change % — query: `?algorithm=lstm_nn` |
| `GET` | `/api/predictions/{id}` | Chi tiết một dự đoán |

### Training

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/training/status` | Trạng thái huấn luyện hiện tại |
| `GET` | `/api/training/history` | Lịch sử các phiên huấn luyện |
| `GET` | `/api/training/algorithms` | Chi tiết từng thuật toán: config, accuracy, last_trained, training_time |
| `GET` | `/api/training/metrics` | Aggregate metrics: avg time, data quality, success rate |
| `GET` | `/api/training/{id}` | Chi tiết một phiên huấn luyện |

### Market & Stocks

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/market/overview` | Tổng quan thị trường — query: `?sector=ngan-hang&exchange=HOSE` |
| `GET` | `/api/stocks/{symbol}/current` | Giá hiện tại |
| `GET` | `/api/stocks/{symbol}/history` | Lịch sử giá |
| `GET` | `/api/stocks/{symbol}/chart` | Dữ liệu biểu đồ |
| `GET` | `/api/stocks/{symbol}/detail` | Chi tiết cổ phiếu |
| `GET` | `/api/stocks/watchlist` | Danh sách theo dõi |

### Gold

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/gold/latest` | Giá vàng mới nhất |
| `GET` | `/api/gold/prices` | Danh sách giá vàng |
| `GET` | `/api/gold/chart` | Dữ liệu biểu đồ vàng |
| `GET` | `/api/gold/predictions/latest` | Dự đoán vàng mới nhất |
| `GET` | `/api/gold/predictions/chart` | Biểu đồ dự đoán vs thực tế (vàng) |
| `GET` | `/api/gold/predictions` | Danh sách dự đoán vàng |

### Dashboard & Algorithms

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/dashboard/stats` | Thống kê tổng quan dashboard |
| `GET` | `/api/algorithms/comparison` | So sánh các thuật toán |
| `GET` | `/api/algorithms/backtest` | Backtest thuật toán |

### Schedules

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/schedules` | Danh sách lịch tác vụ — yêu cầu JWT; trả `[{"job_key":"...","job_name":"...","cron_expression":"...","enabled":true,"updated_at":"..."}]` |
| `PUT` | `/api/schedules/{key}` | Cập nhật lịch tác vụ — yêu cầu JWT; body: `{"cron_expression":"0 0 12 * * *","enabled":true}`; validate cron expression trước khi lưu |

### Trigger

Tất cả trigger endpoint đều yêu cầu JWT authentication (`Authorization: Bearer <token>`), được enforce qua `AuthRequired()` middleware wrapper trong router.

| Method | Path | Ghi chú |
|--------|------|---------|
| `POST` | `/api/trigger/crawler` | Crawl VN30 (background) — yêu cầu JWT |
| `POST` | `/api/trigger/predict` | Chạy dự đoán (background) — yêu cầu JWT |
| `POST` | `/api/trigger/train` | Huấn luyện toàn bộ hoặc một thuật toán — body JSON `{"algorithm":"lstm_nn"}` (optional) — yêu cầu JWT |
| `POST` | `/api/trigger/gold-crawler` | Crawl giá vàng đồng bộ — yêu cầu JWT |
| `POST` | `/api/trigger/gold-history` | Import lịch sử XAU (background) — yêu cầu JWT |
| `POST` | `/api/trigger/gold-predict` | Dự đoán vàng (background) — yêu cầu JWT |
| `POST` | `/api/trigger/reconcile` | Reconcile dự đoán với giá thực tế — yêu cầu JWT |
| `POST` | `/api/trigger/stock-history` | Crawl lịch sử stock (background) — body JSON `{"days":365}` — yêu cầu JWT |
| `POST` | `/api/trigger/historical-backtest` | Walk-forward backtest (background) — query: `?train_window=30&step_size=6`; trả 202; 409 nếu đang chạy — yêu cầu JWT |

## Trigger thủ công qua API

Tất cả trigger endpoint đều gọi gRPC sang Prediction Service. Tất cả đều yêu cầu JWT (`Authorization: Bearer <token>`). Lấy token qua `POST /api/auth/login` trước.

```bash
# Lấy JWT token
TOKEN=$(curl -s -X POST http://localhost:8118/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.token')

# Chạy crawler stock ngay
curl -X POST http://localhost:8118/api/trigger/crawler -H "Authorization: Bearer $TOKEN"

# Chạy dự đoán ngay
curl -X POST http://localhost:8118/api/trigger/predict -H "Authorization: Bearer $TOKEN"

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

# Crawl lịch sử cổ phiếu (mặc định 365 ngày)
curl -X POST http://localhost:8118/api/trigger/stock-history -H "Authorization: Bearer $TOKEN" -d '{"days":365}'

# Chạy historical backtest (query params, không phải body)
curl -X POST "http://localhost:8118/api/trigger/historical-backtest?train_window=30&step_size=6" -H "Authorization: Bearer $TOKEN"

# Crawl và dự đoán một mã cụ thể
curl -X POST http://localhost:8118/api/stocks/VCB/crawl
curl -X POST http://localhost:8118/api/stocks/VCB/predict

# Xem lịch cron
curl http://localhost:8118/api/schedules -H "Authorization: Bearer $TOKEN"

# Cập nhật lịch cron (ví dụ: đổi giờ crawl stock sang 1 PM)
curl -X PUT http://localhost:8118/api/schedules/crawler_daily \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"cron_expression":"0 0 13 * * *","enabled":true}'
```

## gRPC Service Contract

Định nghĩa trong `proto/prediction/prediction.proto`. Prediction Service implement tất cả RPC; API Backend gọi chúng qua gRPC client.

| RPC | Request | Response | Ghi chú |
|-----|---------|----------|---------|
| `TriggerCrawler` | `Empty` | `TriggerResponse` | Crawl VN30 trong background |
| `TriggerGoldCrawler` | `Empty` | `TriggerResponse` | Crawl giá vàng đồng bộ |
| `TriggerPredict` | `Empty` | `TriggerResponse` | Weekly training + daily prediction đồng bộ |
| `TriggerTrain` | `TriggerTrainRequest` | `TriggerTrainResponse` | Train một hoặc tất cả thuật toán — field `algorithm` optional |
| `TriggerReconcile` | `Empty` | `TriggerResponse` | Reconcile dự đoán với giá thực tế đồng bộ |
| `TriggerStockHistory` | `StockHistoryRequest` | `TriggerResponse` | Crawl lịch sử stock trong background — field `days` |
| `TriggerGoldHistory` | `Empty` | `TriggerResponse` | Import lịch sử XAU trong background |
| `TriggerGoldPredict` | `Empty` | `TriggerResponse` | Dự đoán vàng trong background |
| `TriggerHistoricalBacktest` | `BacktestRequest` | `TriggerResponse` | Walk-forward backtest trong background — concurrency guard |
| `TriggerStockCrawl` | `StockRequest` | `StockCrawlResponse` | Crawl và lưu một mã cổ phiếu đồng bộ |
| `TriggerStockPredict` | `StockRequest` | `StockPredictResponse` | Dự đoán tất cả thuật toán cho một mã đồng bộ |
| `GetTrainingStatus` | `Empty` | `TrainingStatusResponse` | Trạng thái training: `is_training`, `progress`, `phase`, `total/done algorithms` |

## Test

```bash
# Chạy tất cả test
make test
# hoặc
go test ./...

# Chỉ unit test (bỏ qua integration)
make test-unit

# Xem coverage
make test-coverage   # tạo coverage.html

# Khởi tạo test DB (MySQL port 3307)
make test-db-up
make test-db-down
```

**Test coverage hiện tại:**

Go API Backend:
- `pkg/server/` — API handlers (mock pattern, không cần DB thật)
- `pkg/testutil/` — Test helpers

Python Prediction Service (`prediction/tests/`):
- `tests/unit/` — Unit tests cho 6 algorithms (moving_average, ema_macd, lstm, arima_garch, lightgbm_model, ensemble)
- `tests/integration/` — End-to-end: gRPC contract, crawlers, prediction pipeline, training, schedules, performance benchmarks, service resilience
- Phase 5 final: **71 passed, 0 failed** (`make test-phase5`)

**Convention test (Go):**
- File test đặt cùng package: `algo.go` → `algo_test.go`
- Integration tests có guard: `if testing.Short() { t.Skip() }`
- API handler tests dùng `recover` pattern (không inject mock DB — xem `pkg/server/*_test.go`)

**Convention test (Python):**
- Unit tests: `tests/unit/test_<module>.py` — không cần DB, dùng mock data
- Integration tests: `tests/integration/test_<phase>.py` — cần Docker stack chạy
- Optional-dep tests: dùng `pytest.importorskip("torch")` để skip gracefully nếu thiếu thư viện
- Chạy trong container: `docker exec prediction_service python -m pytest tests/ -v`
