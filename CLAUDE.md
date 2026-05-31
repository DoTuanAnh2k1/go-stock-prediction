# CLAUDE.md

## Tổng quan dự án

Hệ thống dự đoán giá cổ phiếu Việt Nam viết bằng Go. Thu thập dữ liệu từ VietStock, chạy 5 thuật toán ML (Moving Average, EMA, LSTM, ARIMA-GARCH, Ensemble), và hiển thị kết quả qua web dashboard.

**Kiến trúc hiện tại: microservice (2 service + nginx)**
- **API Backend** (`cmd/api`) — HTTP trên `:8118`, phục vụ toàn bộ `/api/*`. Gọi Prediction Service qua gRPC để trigger crawler/training; đọc DB trực tiếp cho các query dữ liệu.
- **Prediction Service** (`cmd/prediction`) — gRPC trên `:8119`. Xử lý crawling (stock + gold), thuật toán dự đoán, training, cron jobs.
- **Nginx** — Reverse proxy trên `:80`, forward tất cả request về API Backend.


## Lệnh thường dùng

```bash
# Build API Backend
go build -o api-server ./cmd/api

# Build Prediction Service
go build -o prediction-server ./cmd/prediction

# Start tất cả services (DB + Prediction + API + Nginx + Frontend + phpMyAdmin)
docker-compose up -d

# Import schema
mysql -u root -p go_stock_prediction < database.sql

# Crawl dữ liệu lịch sử bằng Python
python cmd/crawdata/main.py

# Regenerate proto (cần protoc + plugins)
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/prediction/prediction.proto
```

## Cấu trúc thư mục quan trọng

```
cmd/api/main.go                         # API Backend entry point — config → timezone → logger → repository → grpcclient → server
cmd/prediction/main.go                  # Prediction Service entry point — config → timezone → logger → repository → grpcserver → crawler → predict
pkg/config/                             # Load .env, trả về config struct toàn cục (bao gồm GRPCConfig)
pkg/grpc/server/server.go               # gRPC server — implement PredictionServiceServer, xử lý tất cả trigger RPC
pkg/grpc/client/client.go               # gRPC client singleton — API Backend dùng để gọi Prediction Service
proto/prediction/prediction.proto       # gRPC service definitions
proto/prediction/*.pb.go                # Generated protobuf code (không sửa tay)
nginx/nginx.conf                        # Nginx reverse proxy: :80 → api:8118
Dockerfile.api                          # Docker build cho API Backend
Dockerfile.prediction                   # Docker build cho Prediction Service
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
pkg/service/crawler/init.go             # Khởi tạo crawler và đăng ký cron job
pkg/service/crawler/crawler.go          # Logic scraping VietStock bằng Gocolly
pkg/service/crawler/gold_crawler.go     # Crawl giá vàng SJC (BTMC) và XAU/USD
pkg/service/predict/init.go             # Khởi tạo 4 thuật toán và đăng ký 3 cron jobs (train/predict/reconcile)
pkg/service/predict/historical_backtest.go # Walk-forward backtest: RunHistoricalBacktest(), walkForwardStock(), bulkInsertPredictions()
pkg/service/predict/moving_average/     # Thuật toán VWMA
pkg/service/predict/lstm_nn/            # Thuật toán LSTM Neural Network
pkg/service/predict/arima_garch/        # Thuật toán ARIMA-GARCH
pkg/service/predict/ensemble/algo.go    # Thuật toán Ensemble — trung bình có trọng số từ 4 thuật toán cơ sở
pkg/service/predict/ema/algo.go         # Thuật toán EMA — MACD(12,26,9) với Vietnamese session adjustment
pkg/service/predict/registry/registry.go   # AlgorithmDef struct, Register(), All(), Build() — hai-pass build (base trước, composite sau)
pkg/service/predict/registry/algorithms.go # FILE DUY NHẤT cần sửa khi thêm/bỏ thuật toán
pkg/service/predict/assettype/iface.go  # Interface AssetPredictionType: TypeKey(), TypeName(), Init(), RunPredictions()
pkg/service/predict/assettype/registry.go  # Register(), All(), Get(key) cho asset types
pkg/service/predict/assettype_stock.go  # Stock asset type — tự đăng ký qua init(), gọi predict.Init()
pkg/service/predict/gold/init.go        # Gold prediction service — đăng ký cron job
pkg/service/predict/gold/assettype_gold.go # Gold asset type — tự đăng ký qua init(), gọi goldpredict.Init()
pkg/service/market/iface.go             # Interface AssetMarket: MarketKey(), MarketName(), GetInstruments(), FetchPrices(), SavePrediction(), Crawl(). Điểm mở rộng chính để thêm market mới.
pkg/service/market/registry.go          # Register(), Get(key), All() cho asset markets — thread-safe, dùng sync.RWMutex
pkg/service/market/vn30/market.go       # VN30 market implementation — 30 cổ phiếu HOSE, tự đăng ký qua init()
pkg/service/market/gold/market.go       # Gold market implementation — 3 sản phẩm: XAU/spot, BTMC/sjc, BTMC/nhan_tron; tự đăng ký qua init()
pkg/service/predict/orchestrator/orchestrator.go # RunAllMarkets(ctx), RunForMarket(ctx, key) — chạy prediction qua tất cả registered markets
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
pkg/utils/cron/                         # Hằng số cron schedule + wrapper
web/templates/                          # HTML templates (Go's html/template)
web/static/js/                          # Frontend JS — AJAX gọi các /api/* endpoint
frontend/src/context/AuthContext.tsx    # AuthProvider + useAuth hook — quản lý JWT trong localStorage
frontend/src/components/LoginModal.tsx  # Login modal component — gọi POST /api/auth/login
frontend/src/pages/Users.tsx            # Trang quản lý user — chỉ hiển thị với role admin
```

## Luồng khởi động

### API Backend (cmd/api/main.go)

1. `config.InitConfig()` — Load file `.env`
2. Set timezone — `Asia/Ho_Chi_Minh`
3. `logger.Init()` — Khởi tạo ZeroLog
4. `repository.Init()` — Kết nối MySQL (shared DB, dùng cho read queries)
5. `seedAdminUser()` — Tạo admin user từ env vars nếu chưa có admin nào trong DB
6. `grpcclient.Init(config.GetGRPCConfig().ClientTarget)` — Kết nối tới Prediction Service
7. `server.StartHTTPServer()` — HTTP server trên `:8118` (goroutine)
8. Chờ SIGTERM/SIGINT → `grpcclient.Close()` → shutdown

### Prediction Service (cmd/prediction/main.go)

1. `config.InitConfig()` — Load file `.env`
2. Set timezone — `Asia/Ho_Chi_Minh`
3. `logger.Init()` — Khởi tạo ZeroLog
4. `repository.Init()` — Kết nối MySQL (shared DB)
5. `grpcserver.New().Start(config.GetGRPCConfig().ServerPort)` — gRPC server trên `:8119`
6. `crawler.Init()` — Đăng ký cron job (goroutine)
7. Startup data sync sau 5 giây delay — gọi `crawler.CronjobCrawler()` một lần
8. `predict.Init()` + `goldpredict.Init()` — Đăng ký cron jobs (goroutines)
9. Chờ SIGTERM/SIGINT → `grpcSrv.Stop()` (graceful) → shutdown

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
```

## Conventions trong codebase

- **Repository pattern:** Mọi truy cập DB phải qua interface `DatabaseStore` trong `pkg/store/repository/`. Không gọi GORM trực tiếp từ service layer.
- **Singleton:** `repository.GetSingleton()` trả về instance DB toàn cục đã init.
- **Cron constants:** Dùng hằng số trong `pkg/utils/cron/` thay vì hardcode cron string.
- **Decimal:** Dùng `shopspring/decimal` cho mọi phép tính số thực liên quan đến giá cổ phiếu — tránh float64.
- **API handlers:** Mỗi nhóm endpoint có file riêng `api_<topic>.go` trong `pkg/server/`.
- **Algorithms:** Mỗi thuật toán implement interface `PredictionAlgorithm` với method `Predict(ctx, StockData) → Prediction`.
- **Logging:** Dùng `pkg/logger` (zerolog), không dùng `fmt.Println` hay `log` stdlib.
- **gRPC triggers:** Tất cả trigger handler trong `pkg/server/api_trigger_*.go` và `pkg/server/api_stock_actions.go` đều gọi `requireGRPCClient(w)` trước. Hàm này trả về 503 nếu gRPC client chưa init (e.g. khi chạy unit test không có prediction service). Tất cả trigger endpoints được wrap bằng `AuthRequired()` trong router — yêu cầu JWT hợp lệ, không còn dùng `API_KEY` header.
- **Dynamic cron schedules:** Lịch cron được lưu trong bảng `cron_schedules`. Khi startup, `seedCronSchedules()` tạo các hàng mặc định nếu chưa tồn tại. Prediction Service poll DB mỗi phút để phát hiện thay đổi và tự reschedule — không cần restart. Dùng `CronScheduleStore` interface (`GetAllCronSchedules`, `GetCronScheduleByKey`, `UpsertCronSchedule`) để truy cập, không truy cập bảng trực tiếp.
- **Proto regeneration:** Khi thay đổi `proto/prediction/prediction.proto`, chạy lệnh `protoc` trong mục Lệnh thường dùng để tái sinh `*.pb.go`. Không sửa tay các file generated.
- **Data ordering — QUAN TRỌNG:** DB trả `stock_prices` với `ORDER BY trading_date DESC` (mới nhất trước). Tất cả thuật toán (MA/LSTM/ARIMA-GARCH/Ensemble) dùng `prices[len-1]` làm giá hiện tại — vì vậy **phải đảo ngược DESC → ASC** trước khi build `Historical []string`. Logic đảo ngược nằm trong `getStockTrainingData()`, `getStockPredictionData()`, và `walkForwardStock()` (dùng `reverseStockPrices()`). Không thêm query `ORDER BY ASC` trực tiếp — repository interface trả DESC, tầng service tự xử lý.
- **JWT middleware — non-blocking:** `JWTMiddleware` trong `pkg/server/middleware_jwt.go` nằm trong middleware chain `CORS → RateLimit → JWT → mux`. Middleware này chỉ inject claims vào context nếu token hợp lệ — request không có token vẫn tiếp tục (unauthenticated). Các handler bảo vệ dùng `requireAuth(w, r)` hoặc `requireAdmin(w, r)` để enforce.
- **Admin seeder:** Khi startup, `seedAdminUser()` trong `cmd/api/main.go` kiểm tra `AdminExists()`. Nếu chưa có user nào với `role="admin"`, tạo một user mới từ `ADMIN_USERNAME`/`ADMIN_PASSWORD` env vars với bcrypt hash. Chạy một lần duy nhất — các lần sau bỏ qua nếu admin đã tồn tại.
- **User management:** `ADMIN_USERNAME`/`ADMIN_PASSWORD` trong `.env` chỉ dùng để seed lần đầu. Sau đó quản lý user hoàn toàn qua API `/api/users` (admin JWT required). Password lưu dưới dạng bcrypt hash — không lưu plaintext.

## Hướng dẫn mở rộng (Extension Guide)

### 1. Thêm thuật toán dự đoán mới

**Chỉ cần chạm vào 2 nơi** — tạo package mới và thêm 1 entry trong file registry:

1. Tạo `pkg/service/predict/<tên_thuật_toán>/algo.go` — implement interface:
   ```go
   type PredictionAlgorithm interface {
       Predict(ctx context.Context, data *modelssvc.StockData) (*modelssvc.Prediction, error)
       GetName() string
       GetAccuracy() float64
   }
   ```
2. Thêm entry vào `pkg/service/predict/registry/algorithms.go` trong `init()`, phần base algorithms:
   ```go
   Register(AlgorithmDef{
       Key:         "ten_thuat_toan",
       DisplayName: "Tên Hiển Thị",
       Config:      map[string]interface{}{"param": value},
       Factory:     func() iface.PredictionAlgorithm { return tenthuattoan.New() },
   })
   ```
3. (Tuỳ chọn) Nếu muốn thuật toán mới tham gia Ensemble, thêm `bases["ten_thuat_toan"]` vào slice trong `CompositeFactory` của Ensemble trong cùng file.
4. Thuật toán tự động xuất hiện trong API `/api/training/algorithms` và được dùng cho cả stock lẫn gold prediction — không cần sửa thêm file nào khác.

**Lưu ý về `Build()`:** Hàm này dùng hai lần lặp (two-pass). Pass 1: khởi tạo tất cả base algorithm (các def có `IsComposite = false`). Pass 2: khởi tạo composite algorithm (Ensemble) và truyền vào map các base instances đã build. Điều này đảm bảo Ensemble luôn nhận được đúng instance đang dùng.

### 2. Thêm thị trường hoặc loại tài sản dự đoán mới

Kể từ khi orchestrator ra đời, "thêm sàn/chỉ số" và "thêm loại tài sản" đều quy về cùng một pattern: implement `AssetMarket` và đăng ký vào registry. Orchestrator tự động picks up market mới — không cần sửa cron hay prediction logic.

**Gold đã implement AssetMarket.** Để thêm crypto hoặc thị trường khác: xem hướng dẫn chi tiết tại `ADDING_NEW_MARKET.md`.

Tóm tắt các bước bắt buộc:
1. Tạo DB model + migration trong `pkg/models/models_db/`.
2. Thêm repository methods vào `pkg/store/repository/repository.go` và implement trong `pkg/store/mysql/`.
3. Tạo crawler trong `pkg/service/crawler/` và đăng ký cron job trong `crawler/init.go`.
4. Tạo `pkg/service/market/<tên>/market.go` — implement `AssetMarket`, gọi `market.Register()` trong `init()`.
5. Thêm blank import trong `cmd/prediction/main.go`:
   ```go
   _ "go-stock-prediction/pkg/service/market/<tên>"
   ```
   Orchestrator (`RunAllMarkets`) tự động chạy prediction cho market mới — không cần sửa thêm file nào.

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

Hằng số cron trong `pkg/utils/cron/` vẫn được dùng làm giá trị mặc định khi seed.

## Ports

| Service | Port | Protocol | Ghi chú |
|---------|------|----------|---------|
| Nginx | 80 | HTTP | Reverse proxy → API Backend |
| API Backend | 8118 | HTTP | Toàn bộ `/api/*` endpoints |
| Prediction Service | 8119 | gRPC | Internal only (không expose ra ngoài) |
| MySQL | 3306 | TCP | Docker |
| phpMyAdmin | 8081 | HTTP | Docker |
| Frontend | 36018 | HTTP | React app (Docker) |

## Docker Compose Services

| Service | Image/Dockerfile | Depends On | Ghi chú |
|---------|-----------------|------------|---------|
| `db` | `mysql:8.0` | — | Schema tự init từ `database.sql` |
| `prediction` | `Dockerfile.prediction` | `db` (healthy) | Prediction Service — gRPC :8119 (internal) |
| `api` | `Dockerfile.api` | `db` (healthy), `prediction` | API Backend — HTTP :8118 (internal) |
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
- `pkg/server/` — API handlers (mock pattern, không cần DB thật)
- `pkg/service/predict/moving_average/` — VWMA algorithm
- `pkg/service/predict/lstm_nn/` — LSTM algorithm
- `pkg/service/predict/arima_garch/` — ARIMA-GARCH algorithm
- `pkg/service/predict/ensemble/` — Ensemble algorithm
- `pkg/service/crawler/` — Gold price parsing, JSON mapping
- `pkg/testutil/` — Test helpers

**Convention test:**
- File test đặt cùng package: `algo.go` → `algo_test.go`
- Integration tests có guard: `if testing.Short() { t.Skip() }`
- API handler tests dùng `recover` pattern (không inject mock DB — xem `pkg/server/*_test.go`)
