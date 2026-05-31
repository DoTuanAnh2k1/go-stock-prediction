# go-stock-prediction

Hệ thống dự đoán giá cổ phiếu thị trường chứng khoán Việt Nam (VN30), bao gồm crawl dữ liệu tự động, huấn luyện mô hình ML, và dashboard theo dõi.

## Tính năng

- **Thu thập dữ liệu:** Crawl giá cổ phiếu VN30 từ VietStock mỗi ngày lúc 12 PM; crawl giá vàng SJC/XAU/USD lúc 10 AM
- **Dự đoán giá:** 4 thuật toán ML chạy song song — Moving Average (VWMA), LSTM Neural Network, ARIMA-GARCH, Ensemble
- **Walk-forward Backtest:** Backtest lịch sử toàn bộ VN30 với cơ chế fold tự động (`/api/trigger/historical-backtest`)
- **Huấn luyện tự động:** Mỗi Chủ nhật lúc 9 AM, hệ thống tự train lại toàn bộ mô hình
- **Reconcile hàng ngày:** 6 AM cập nhật `actual_price` và `accuracy` vào các dự đoán đã qua `target_date`
- **Dashboard web:** React SPA — tổng quan thị trường, biểu đồ, kết quả dự đoán, giá vàng
- **REST API:** Endpoints đầy đủ cho dữ liệu cổ phiếu, dự đoán, trigger thủ công

## Kiến trúc

```
Microservice: 2 Go services + Nginx reverse proxy

cmd/api/main.go          API Backend (:8118) — HTTP, đọc DB trực tiếp, gọi Prediction Service qua gRPC
cmd/prediction/main.go   Prediction Service (:8119) — gRPC, crawling, thuật toán, training, cron jobs
nginx/nginx.conf         Reverse proxy :80 → api:8118

pkg/
  config/                Quản lý cấu hình (.env)
  grpc/
    server/server.go     gRPC server — implement PredictionServiceServer
    client/client.go     gRPC client singleton dùng trong API Backend
  server/                HTTP server, routes, handlers (api_*.go)
  service/
    crawler/             Thu thập dữ liệu từ VietStock (Gocolly) và giá vàng
    predict/             Thuật toán dự đoán (MA, LSTM, ARIMA-GARCH, Ensemble)
    predict/historical_backtest.go  Walk-forward backtesting
  store/
    repository/          Interface repository pattern (DatabaseStore)
    mysql/               Triển khai MySQL (GORM)
  models/
    models_db/           GORM struct (Stock, StockPrice, Prediction, GoldPrice, TrainingLog, ...)
    models_api/          DTO cho API response
  utils/cron/            Hằng số và wrapper cho robfig/cron/v3

proto/prediction/        gRPC service definitions + generated code
frontend/                React SPA (Vite + TypeScript)
cmd/crawdata/main.py     Script Python crawl dữ liệu lịch sử (vnstock API)
```

## Yêu cầu

- Go 1.23+
- MySQL 8.0+ (hoặc dùng Docker Compose)
- Docker + Docker Compose (khuyến nghị)
- File `.env` cấu hình (xem bên dưới)

## Cài đặt & Chạy

### Chạy bằng Docker Compose (khuyến nghị)

```bash
# Khởi động tất cả services (DB, Prediction, API, Nginx, Frontend, phpMyAdmin)
docker-compose up -d
```

Dashboard React: `http://localhost:36018`
API Backend: `http://localhost:80` (qua Nginx)
phpMyAdmin: `http://localhost:8081`

### Tạo file `.env`

```env
# HTTP server (API Backend)
SERVER_PORT=8118

# gRPC
GRPC_SERVER_PORT=8119
GRPC_TARGET=prediction:8119   # Docker internal; dùng localhost:8119 khi chạy local

# Auth
API_KEY=                       # Optional — bảo vệ một số trigger endpoint
ADMIN_USERNAME=admin           # Username đăng nhập dashboard (default: admin)
ADMIN_PASSWORD=admin123        # Password đăng nhập dashboard (default: admin123)
JWT_SECRET=change-me-in-production  # Secret ký JWT — bắt buộc đổi trong production

# Database
DB_DRIVER=mysql
MYSQL_HOST=db                  # Docker internal; dùng localhost khi chạy local
MYSQL_PORT=3306
MYSQL_USER=root
MYSQL_PASSWORD=123
MYSQL_DB_NAME=go_stock_prediction
MYSQL_DEBUG=false

# Logging
LOG_LEVEL=DEBUG
DB_LOG_LEVEL=DEBUG
```

### Build và chạy local (không Docker)

```bash
# Import schema
mysql -u root -p go_stock_prediction < database.sql

# Build và chạy Prediction Service trước
go build -o prediction-server ./cmd/prediction
./prediction-server

# Build và chạy API Backend
go build -o api-server ./cmd/api
./api-server
```

### (Tuỳ chọn) Crawl dữ liệu lịch sử bằng Python

```bash
pip install vnstock mysql-connector-python pandas
python cmd/crawdata/main.py
```

Script dùng vnstock API để nhập dữ liệu giá lịch sử vào DB.

## Lần đầu khởi động (luồng bắt buộc)

Khi mới cài đặt, DB trống — thực hiện tuần tự:

```bash
# Lấy JWT token (admin được tạo tự động từ ADMIN_USERNAME/ADMIN_PASSWORD env vars)
TOKEN=$(curl -s -X POST http://localhost/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.token')
```

**Bước 1 — Crawl lịch sử (bắt buộc)**

```bash
python cmd/crawdata/main.py
# hoặc qua API (crawl 365 ngày gần nhất)
curl -X POST "http://localhost/api/trigger/stock-history" \
  -H "Authorization: Bearer $TOKEN" -d '{"days":365}'
```

**Bước 2 — Huấn luyện mô hình**

```bash
curl -X POST http://localhost/api/trigger/train -H "Authorization: Bearer $TOKEN"
```

**Bước 3 — Chạy walk-forward backtest (tạo dữ liệu `confirmed` để xem chart)**

```bash
curl -X POST "http://localhost/api/trigger/historical-backtest?train_window=30&step_size=6" \
  -H "Authorization: Bearer $TOKEN"
```

**Bước 4 — Chạy dự đoán hiện tại**

```bash
curl -X POST http://localhost/api/trigger/predict -H "Authorization: Bearer $TOKEN"
```

## Trigger thủ công bằng curl

Tất cả trigger endpoints yêu cầu JWT. Lấy token trước:

```bash
TOKEN=$(curl -s -X POST http://localhost/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.token')

curl -X POST http://localhost/api/trigger/crawler -H "Authorization: Bearer $TOKEN"             # crawl cổ phiếu
curl -X POST http://localhost/api/trigger/predict -H "Authorization: Bearer $TOKEN"             # chạy dự đoán
curl -X POST http://localhost/api/trigger/train -H "Authorization: Bearer $TOKEN"               # huấn luyện tất cả
curl -X POST http://localhost/api/trigger/gold-crawler -H "Authorization: Bearer $TOKEN"        # crawl giá vàng
curl -X POST http://localhost/api/trigger/reconcile -H "Authorization: Bearer $TOKEN"           # reconcile actual price

# Backtest toàn bộ lịch sử VN30 (async, trả 202 ngay)
curl -X POST "http://localhost/api/trigger/historical-backtest?train_window=30&step_size=6" \
  -H "Authorization: Bearer $TOKEN"
```

## API Endpoints

### Auth

| Method | Path | Mô tả |
|--------|------|-------|
| `POST` | `/api/auth/login` | Đăng nhập — body: `{"username":"","password":""}`, trả JWT 24h |
| `GET` | `/api/auth/me` | Xác minh token — header: `Authorization: Bearer <token>` |

### Predictions

| Method | Path | Mô tả |
|--------|------|-------|
| `GET` | `/api/predictions` | Danh sách dự đoán — query: `?symbol=VCB&algorithm=lstm_nn&status=confirmed&from=YYYY-MM-DD&page=1&limit=20` |
| `GET` | `/api/predictions/accuracy` | Độ chính xác theo thuật toán |
| `GET` | `/api/predictions/accuracy-trend` | Xu hướng độ chính xác theo thời gian |
| `GET` | `/api/predictions/compare/{symbol}` | Dự đoán vs thực tế — query: `?days=90&algorithm=lstm_nn` |
| `GET` | `/api/predictions/error-distribution` | Scatter plot predicted change % vs actual change % — query: `?algorithm=lstm_nn` |
| `GET` | `/api/predictions/{id}` | Chi tiết một dự đoán |

### Training

| Method | Path | Mô tả |
|--------|------|-------|
| `GET` | `/api/training/status` | Trạng thái huấn luyện: `is_training`, `progress`, `phase` |
| `GET` | `/api/training/history` | Lịch sử các phiên huấn luyện |
| `GET` | `/api/training/algorithms` | Chi tiết từng thuật toán: config, accuracy, last_trained |
| `GET` | `/api/training/metrics` | Aggregate metrics: avg time, success rate |
| `GET` | `/api/training/{id}` | Chi tiết một phiên huấn luyện |

### Market & Stocks

| Method | Path | Mô tả |
|--------|------|-------|
| `GET` | `/api/market/overview` | Tổng quan thị trường — query: `?sector=ngan-hang&exchange=HOSE` |
| `GET` | `/api/stocks/{symbol}/current` | Giá hiện tại |
| `GET` | `/api/stocks/{symbol}/history` | Lịch sử giá |
| `GET` | `/api/stocks/{symbol}/chart` | Dữ liệu biểu đồ |
| `GET` | `/api/stocks/{symbol}/detail` | Chi tiết cổ phiếu |
| `GET` | `/api/stocks/watchlist` | Danh sách theo dõi |

### Gold

| Method | Path | Mô tả |
|--------|------|-------|
| `GET` | `/api/gold/latest` | Giá vàng mới nhất |
| `GET` | `/api/gold/prices` | Danh sách giá vàng |
| `GET` | `/api/gold/chart` | Biểu đồ lịch sử giá vàng |
| `GET` | `/api/gold/predictions/latest` | Dự đoán vàng mới nhất |
| `GET` | `/api/gold/predictions/chart` | Biểu đồ dự đoán vs thực tế (vàng) |
| `GET` | `/api/gold/predictions` | Danh sách dự đoán vàng |

### Schedules

| Method | Path | Mô tả |
|--------|------|-------|
| `GET` | `/api/schedules` | Danh sách lịch tác vụ cron — yêu cầu JWT |
| `PUT` | `/api/schedules/{key}` | Cập nhật lịch tác vụ — yêu cầu JWT; body: `{"cron_expression":"0 0 12 * * *","enabled":true}` |

### Dashboard, Algorithms & Triggers

Tất cả `POST /api/trigger/*` yêu cầu JWT (`Authorization: Bearer <token>`).

| Method | Path | Mô tả |
|--------|------|-------|
| `GET` | `/api/dashboard/stats` | Thống kê tổng quan |
| `GET` | `/api/algorithms/comparison` | So sánh các thuật toán |
| `GET` | `/api/algorithms/backtest` | Backtest thuật toán |
| `POST` | `/api/trigger/crawler` | Crawl VN30 (background) — yêu cầu JWT |
| `POST` | `/api/trigger/predict` | Chạy dự đoán (background) — yêu cầu JWT |
| `POST` | `/api/trigger/train` | Huấn luyện — body: `{"algorithm":"lstm_nn"}` (optional) — yêu cầu JWT |
| `POST` | `/api/trigger/gold-crawler` | Crawl giá vàng — yêu cầu JWT |
| `POST` | `/api/trigger/gold-history` | Import lịch sử XAU — yêu cầu JWT |
| `POST` | `/api/trigger/gold-predict` | Dự đoán vàng — yêu cầu JWT |
| `POST` | `/api/trigger/reconcile` | Reconcile actual prices — yêu cầu JWT |
| `POST` | `/api/trigger/stock-history` | Crawl lịch sử stock — body: `{"days":365}` — yêu cầu JWT |
| `POST` | `/api/trigger/historical-backtest` | Walk-forward backtest — query: `?train_window=30&step_size=6`; trả 202, 409 nếu đang chạy — yêu cầu JWT |
| `POST` | `/api/stocks/{symbol}/crawl` | Crawl một mã cổ phiếu |
| `POST` | `/api/stocks/{symbol}/predict` | Dự đoán một mã cổ phiếu |
| `GET` | `/health` | Health check chi tiết |
| `GET` | `/health/simple` | Health check đơn giản |

## Lịch chạy tự động

Lịch được lưu trong bảng DB `cron_schedules` và có thể chỉnh sửa live qua `PUT /api/schedules/{key}` hoặc Settings page trên frontend — không cần restart service.

| Job Key | Thời gian mặc định | Công việc |
|---------|-------------------|-----------|
| `reconcile_daily` | Mỗi ngày 6:00 AM | Reconcile dự đoán với giá thực tế |
| `gold_crawler_daily` | Mỗi ngày 10:00 AM | Crawl giá vàng SJC và XAU/USD |
| `crawler_daily` | Mỗi ngày 12:00 PM | Crawl giá cổ phiếu từ VietStock |
| `predict_daily` | Mỗi ngày 6:00 PM | Chạy dự đoán giá cho ngày giao dịch tiếp theo |
| `train_weekly` | Chủ nhật 9:00 AM | Huấn luyện lại toàn bộ mô hình |

## Các thuật toán dự đoán

### Moving Average (VWMA)
- Volume-Weighted Moving Average
- Short period: 5 ngày, Long period: 20 ngày
- Tích hợp điều chỉnh phiên giao dịch Việt Nam

### LSTM Neural Network
- 2 lớp LSTM, 50 hidden units
- Sequence length: 60 ngày (~3 tháng)
- 8 features: giá, khối lượng, chỉ báo kỹ thuật
- Chuẩn hóa dữ liệu bằng MinMaxScaler

### ARIMA-GARCH
- ARIMA để dự đoán xu hướng giá
- GARCH để mô hình hoá biến động (volatility)

### Ensemble
- Kết hợp kết quả từ 3 thuật toán cơ sở (MA, LSTM, ARIMA-GARCH)
- Trung bình có trọng số dựa trên accuracy từng thuật toán

## Walk-forward Historical Backtest

`POST /api/trigger/historical-backtest?train_window=30&step_size=6`

- Chạy walk-forward backtesting trên toàn bộ VN30
- Fold 1: train trên `train_window` ngày đầu, predict `step_size` ngày tiếp theo
- Mỗi fold expand thêm `step_size` ngày; lặp đến hết lịch sử
- Mỗi fold dùng context cố định (không rolling) — simulate "dự đoán trước khi biết kết quả"
- Predictions lưu kèm `actual_price` và `accuracy` ngay (vì backtesting biết lịch sử)
- Xóa predictions `target_date < today` trước khi insert — idempotent khi gọi lại
- Batch insert 200 rows/lần

## Test

```bash
# Chạy tất cả test
go test ./...

# Chỉ unit test
go test -short ./...

# Coverage
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out -o coverage.html
```

## Ports

| Service | Port | Ghi chú |
|---------|------|---------|
| Nginx | 80 | Reverse proxy → API Backend |
| API Backend | 8118 | HTTP (internal) |
| Prediction Service | 8119 | gRPC (internal) |
| Frontend | 36018 | React app |
| MySQL | 3306 | Docker |
| phpMyAdmin | 8081 | Admin UI |

## Công nghệ sử dụng

| Lĩnh vực | Thư viện |
|----------|---------|
| HTTP server | `net/http` (stdlib) |
| gRPC | `google.golang.org/grpc` + protobuf |
| ORM | GORM v2 + MySQL driver |
| Web scraping | Gocolly v2 |
| Cron jobs | `robfig/cron/v3` |
| Logging | ZeroLog |
| Số thực tài chính | `shopspring/decimal` |
| Config | `joho/godotenv` |
| Frontend | React + TypeScript + Vite |
