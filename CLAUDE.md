# CLAUDE.md

## Tổng quan dự án

Hệ thống dự đoán giá cổ phiếu Việt Nam viết bằng Go. Thu thập dữ liệu từ VietStock, chạy 3 thuật toán ML (Moving Average, LSTM, ARIMA-GARCH), và hiển thị kết quả qua web dashboard.

## Lệnh thường dùng

```bash
# Chạy ứng dụng
go run ./cmd/app

# Build
go build -o go-stock-prediction ./cmd/app

# Start database (MySQL + phpMyAdmin)
docker-compose up -d

# Import schema
mysql -u root -p go_stock_prediction < database.sql

# Crawl dữ liệu lịch sử bằng Python
python cmd/crawdata/main.py
```

## Cấu trúc thư mục quan trọng

```
cmd/app/main.go                         # Entry point — khởi động theo thứ tự: config → logger → repository → server → crawler → predict
pkg/config/                             # Load .env, trả về config struct toàn cục
pkg/server/router.go                    # Đăng ký tất cả routes (web + API)
pkg/server/api_*.go                     # Mỗi file = một nhóm API endpoint
pkg/server/api_prediction_compare.go    # GET /api/predictions/compare/{symbol} và /api/predictions/error-distribution
pkg/server/api_training_algorithms.go   # GET /api/training/algorithms
pkg/server/api_training_metrics.go      # GET /api/training/metrics
pkg/server/api_gold.go                  # GET /api/gold/latest, /api/gold/prices, /api/gold/chart
pkg/server/api_stock_detail.go          # GET /api/stocks/{symbol}/detail
pkg/server/api_dashboard_stats.go       # GET /api/dashboard/stats
pkg/server/api_accuracy_trend.go        # GET /api/predictions/accuracy-trend
pkg/service/crawler/init.go             # Khởi tạo crawler và đăng ký cron job
pkg/service/crawler/crawler.go          # Logic scraping VietStock bằng Gocolly
pkg/service/crawler/gold_crawler.go     # Crawl giá vàng SJC (BTMC) và XAU/USD
pkg/service/predict/init.go             # Khởi tạo thuật toán và đăng ký cron jobs
pkg/service/predict/moving_average/     # Thuật toán VWMA
pkg/service/predict/lstm_nn/            # Thuật toán LSTM Neural Network
pkg/service/predict/arima_garch/        # Thuật toán ARIMA-GARCH
pkg/store/repository/repository.go     # Interface DatabaseStore (composite)
pkg/store/mysql/                        # Triển khai MySQL dùng GORM
pkg/models/models_db/                   # GORM struct: Stock, StockPrice, Prediction, SyncLog, Exchange, GoldPrice, TrainingLog, TrainingMetrics
pkg/models/models_db/gold_price.go      # GoldPrice GORM struct
pkg/models/models_db/training_log.go    # TrainingLog GORM struct
pkg/models/models_db/training_metrics.go # TrainingMetrics struct
pkg/models/models_api/                  # DTO cho JSON response
pkg/utils/cron/                         # Hằng số cron schedule + wrapper
web/templates/                          # HTML templates (Go's html/template)
web/static/js/                          # Frontend JS — AJAX gọi các /api/* endpoint
```

## Luồng khởi động (main.go)

1. `config.InitConfig()` — Load file `.env`
2. `logger.Init()` — Khởi tạo ZeroLog
3. `repository.Init()` — Kết nối MySQL, chạy auto-migrate
4. `server.StartHTTPServer()` — HTTP server trên `:31300` (goroutine)
5. `crawler.Init()` — Đăng ký cron job crawl 12 PM hằng ngày
6. `predict.Init()` — Đăng ký cron job dự đoán 6 PM và train Chủ nhật 9 AM
7. Chờ SIGTERM/SIGINT để shutdown

## Biến môi trường (.env)

```
SERVER_PORT=31300
DB_DRIVER=mysql
MYSQL_HOST=localhost
MYSQL_PORT=3306
MYSQL_USER=root
MYSQL_PASSWORD=123
MYSQL_DB_NAME=go_stock_prediction
MYSQL_DEBUG=false
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

## Database

- **ORM:** GORM v2
- **Tables chính:** `exchanges`, `stocks`, `stock_prices`, `predictions`, `sync_logs`, `gold_prices`, `training_logs`
- **Auto-migrate:** Chạy khi start app qua `models_db/migrations.go`
- **Schema đầy đủ:** `database.sql` ở root

## Cron schedules

| Hằng số | Schedule | Công việc |
|---------|----------|-----------|
| `Daily10AM` | `0 0 10 * * *` | Crawl giá vàng SJC và XAU/USD |
| `Daily12PM` | `0 0 12 * * *` | Crawl dữ liệu giá cổ phiếu |
| `Daily6PM` | `0 0 18 * * *` | Chạy dự đoán |
| `WeeklySundayAM` | `0 0 9 * * SUN` | Huấn luyện mô hình |

## Ports

- **App:** 31300
- **MySQL:** 3306 (Docker)
- **phpMyAdmin:** 8080 (Docker)

## API Endpoints

### Predictions

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/predictions` | Danh sách dự đoán — response gồm `actual_price`, `accuracy`, `status` (pending/confirmed/wrong) |
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

### Dashboard & Algorithms

| Method | Path | Ghi chú |
|--------|------|---------|
| `GET` | `/api/dashboard/stats` | Thống kê tổng quan dashboard |
| `GET` | `/api/algorithms/comparison` | So sánh các thuật toán |
| `GET` | `/api/algorithms/backtest` | Backtest thuật toán |

## Trigger thủ công qua API

```bash
# Chạy crawler ngay
curl -X POST http://localhost:31300/api/trigger/crawler

# Chạy dự đoán ngay
curl -X POST http://localhost:31300/api/trigger/predict

# Huấn luyện mô hình ngay
curl -X POST http://localhost:31300/api/trigger/train

# Crawl vàng ngay
curl -X POST http://localhost:31300/api/trigger/gold-crawler
```

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
- `pkg/service/crawler/` — Gold price parsing, JSON mapping
- `pkg/testutil/` — Test helpers

**Convention test:**
- File test đặt cùng package: `algo.go` → `algo_test.go`
- Integration tests có guard: `if testing.Short() { t.Skip() }`
- API handler tests dùng `recover` pattern (không inject mock DB — xem `pkg/server/*_test.go`)
