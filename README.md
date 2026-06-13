# go-stock-prediction

Hệ thống dự đoán giá tài sản tài chính (Gold SJC/XAU, NASDAQ, Crypto BTC/ETH/SOL, S&P 500), bao gồm crawl dữ liệu tự động, huấn luyện mô hình ML, và dashboard theo dõi.

## Tính năng

- **Thu thập dữ liệu:** Crawl giá vàng SJC/XAU/USD mỗi giờ; NASDAQ và S&P 500 mỗi giờ; Crypto BTC/ETH/SOL mỗi 2 giờ
- **Backup tự động:** mysqldump toàn bộ DB hàng ngày lúc 3 AM vào `BACKUP_DIR`
- **Dự đoán giá:** 11 thuật toán ML — Moving Average, EMA/MACD, LSTM, GRU, ARIMA-GARCH, EGARCH, SARIMA, LightGBM, XGBoost, Random Forest, Ensemble
- **Walk-forward Backtest:** Backtest lịch sử với cơ chế fold tự động (`/api/trigger/historical-backtest`)
- **Huấn luyện tự động:** Per-market training jobs mỗi Chủ nhật (staggered từ 3AM đến 7AM)
- **Reconcile hàng ngày:** 6 AM cập nhật `actual_price` và `direction_correct` vào các dự đoán đã qua `target_date`
- **Dashboard web:** React SPA — biểu đồ, kết quả dự đoán, giá vàng, NASDAQ, Crypto, S&P 500
- **REST API:** Endpoints đầy đủ cho mọi market, dự đoán, trigger thủ công

## Kiến trúc

```
Microservice: Go API Backend + Python Prediction Service + Nginx

api/cmd/main.go              API Backend (:8118) — HTTP, đọc DB trực tiếp, gọi Prediction Service qua gRPC
prediction/src/main.py       Prediction Service (:8119) — gRPC, crawling (Gold/NASDAQ/Crypto/SP500),
                             11 thuật toán ML, training, APScheduler cron jobs
nginx/nginx.conf             Reverse proxy :80 → api:8118

api/pkg/
  config/                    Quản lý cấu hình (.env)
  grpc/client/client.go      gRPC client singleton dùng trong API Backend
  server/                    HTTP server, routes, handlers (api_*.go)
  store/
    repository/              Interface repository pattern (DatabaseStore)
    mysql/                   Triển khai MySQL (GORM)
  models/
    models_db/               GORM struct (GoldPrice, NasdaqPrice, CryptoPrice, Sp500Price, predictions, TrainingLog, ...)
    models_api/              DTO cho API response

prediction/src/
  algorithms/                11 thuật toán ML (MA, EMA, LSTM, GRU, ARIMA-GARCH, EGARCH, SARIMA, LightGBM, XGBoost, RF, Ensemble)
  crawlers/                  Gold, NASDAQ, Crypto, S&P 500
  scheduler/                 APScheduler + DB-backed cron schedules
  orchestrator/              run_all_markets(), train_for_market(), reconcile_predictions()

api/proto/prediction/        gRPC service definitions + generated code
frontend/                    React SPA (Vite + TypeScript)
```

## Yêu cầu

- Go 1.23+ (API Backend)
- Python 3.12+ (Prediction Service)
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

# Chạy Prediction Service (Python) trước
cd prediction && pip install -e ".[ml]"
python -m src.main

# Build và chạy API Backend (Go) — từ thư mục api/
cd api && go build -o api-server ./cmd
./api-server
```

## Lần đầu khởi động (luồng bắt buộc)

Khi mới cài đặt, DB trống — thực hiện tuần tự:

```bash
# Lấy JWT token (admin được tạo tự động từ ADMIN_USERNAME/ADMIN_PASSWORD env vars)
TOKEN=$(curl -s -X POST http://localhost/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.token')
```

**Bước 1 — Crawl dữ liệu ban đầu**

```bash
# Crawl Gold, NASDAQ, Crypto, S&P 500 ngay
curl -X POST http://localhost/api/trigger/gold-crawler -H "Authorization: Bearer $TOKEN"
curl -X POST http://localhost/api/trigger/nasdaq-crawler -H "Authorization: Bearer $TOKEN"
curl -X POST http://localhost/api/trigger/crypto-crawler -H "Authorization: Bearer $TOKEN"
curl -X POST http://localhost/api/trigger/sp500-crawler -H "Authorization: Bearer $TOKEN"
```

**Bước 2 — Huấn luyện mô hình**

```bash
curl -X POST http://localhost/api/trigger/train -H "Authorization: Bearer $TOKEN"
```

**Bước 3 — Chạy walk-forward backtest (tạo dữ liệu `confirmed` để xem chart)**

```bash
curl -X POST "http://localhost/api/trigger/historical-backtest?train_window=30&step_size=6&market_key=ALL" \
  -H "Authorization: Bearer $TOKEN"
```

## Trigger thủ công bằng curl

Tất cả trigger endpoints yêu cầu JWT. Lấy token trước:

```bash
TOKEN=$(curl -s -X POST http://localhost/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.token')

curl -X POST http://localhost/api/trigger/train -H "Authorization: Bearer $TOKEN"               # huấn luyện tất cả
curl -X POST http://localhost/api/trigger/gold-crawler -H "Authorization: Bearer $TOKEN"        # crawl giá vàng
curl -X POST http://localhost/api/trigger/nasdaq-crawler -H "Authorization: Bearer $TOKEN"      # crawl NASDAQ
curl -X POST http://localhost/api/trigger/crypto-crawler -H "Authorization: Bearer $TOKEN"      # crawl Crypto
curl -X POST http://localhost/api/trigger/sp500-crawler -H "Authorization: Bearer $TOKEN"       # crawl S&P 500
curl -X POST http://localhost/api/trigger/reconcile -H "Authorization: Bearer $TOKEN"           # reconcile actual price

# Backtest tất cả markets (async, trả 202 ngay)
curl -X POST "http://localhost/api/trigger/historical-backtest?train_window=30&step_size=6&market_key=ALL" \
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
| `GET` | `/api/predictions/direction-accuracy` | Direction accuracy per algorithm — query: `?market=GOLD\|NASDAQ\|SP500\|CRYPTO` |

### Training

| Method | Path | Mô tả |
|--------|------|-------|
| `GET` | `/api/training/status` | Trạng thái huấn luyện: `is_training`, `progress`, `phase` |
| `GET` | `/api/training/history` | Lịch sử các phiên huấn luyện |
| `GET` | `/api/training/algorithms` | Chi tiết từng thuật toán: config, accuracy, last_trained |
| `GET` | `/api/training/metrics` | Aggregate metrics: avg time, success rate |
| `GET` | `/api/training/{id}` | Chi tiết một phiên huấn luyện |

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

### Dashboard & Triggers

Tất cả `POST /api/trigger/*` yêu cầu JWT (`Authorization: Bearer <token>`).

| Method | Path | Mô tả |
|--------|------|-------|
| `GET` | `/api/dashboard/stats` | Thống kê tổng quan |
| `GET` | `/api/training/status` | Trạng thái huấn luyện |
| `GET` | `/api/training/algorithms` | Chi tiết từng thuật toán |
| `POST` | `/api/trigger/train` | Huấn luyện — body: `{"algorithm":"lstm_nn"}` (optional) — yêu cầu JWT |
| `POST` | `/api/trigger/gold-crawler` | Crawl giá vàng — yêu cầu JWT |
| `POST` | `/api/trigger/gold-history` | Import lịch sử XAU — yêu cầu JWT |
| `POST` | `/api/trigger/gold-predict` | Dự đoán vàng — yêu cầu JWT |
| `POST` | `/api/trigger/nasdaq-crawler` | Crawl NASDAQ — yêu cầu JWT |
| `POST` | `/api/trigger/nasdaq-predict` | Dự đoán NASDAQ — yêu cầu JWT |
| `POST` | `/api/trigger/crypto-crawler` | Crawl Crypto — yêu cầu JWT |
| `POST` | `/api/trigger/crypto-predict` | Dự đoán Crypto — yêu cầu JWT |
| `POST` | `/api/trigger/sp500-crawler` | Crawl S&P 500 — yêu cầu JWT |
| `POST` | `/api/trigger/sp500-predict` | Dự đoán S&P 500 — yêu cầu JWT |
| `POST` | `/api/trigger/reconcile` | Reconcile actual prices — yêu cầu JWT |
| `POST` | `/api/trigger/historical-backtest` | Walk-forward backtest — query: `?train_window=30&step_size=6&market_key=ALL`; trả 202, 409 nếu đang chạy — yêu cầu JWT |
| `GET` | `/health` | Health check chi tiết |
| `GET` | `/health/simple` | Health check đơn giản |

## Lịch chạy tự động

Lịch được lưu trong bảng DB `cron_schedules` và có thể chỉnh sửa live qua `PUT /api/schedules/{key}` hoặc Settings page trên frontend — không cần restart service.

| Job Key | Thời gian mặc định | Công việc |
|---------|-------------------|-----------|
| `daily_reconcile` | Mỗi ngày 6:00 AM | Reconcile dự đoán với giá thực tế |
| `crawler_gold` | Mỗi giờ (phút 0) | Pipeline Gold: crawl → train mỗi 10 lần → predict |
| `crawler_nasdaq` | Mỗi giờ (phút 15) | Pipeline NASDAQ: crawl → train mỗi 10 lần → predict |
| `crawler_sp500` | Mỗi giờ (phút 0 và 30) | Pipeline S&P 500: crawl → train mỗi 10 lần → predict |
| `crawler_crypto` | Mỗi 2 giờ | Pipeline Crypto: crawl → train mỗi 10 lần → predict |
| `train_gold` | Chủ nhật 3:00 AM | Huấn luyện lại mô hình Gold |
| `train_nasdaq` | Chủ nhật 4:00 AM | Huấn luyện lại mô hình NASDAQ |
| `train_crypto` | Chủ nhật 5:00 AM | Huấn luyện lại mô hình Crypto |
| `train_sp500` | Chủ nhật 7:00 AM | Huấn luyện lại mô hình S&P 500 |
| `simulation_daily` | Mỗi ngày 8:00 PM | Bot trading hàng ngày |
| `daily_backup` | Mỗi ngày 3:00 AM | Backup database bằng mysqldump vào `BACKUP_DIR` |

## Các thuật toán dự đoán (11 thuật toán)

| Key | Tên | Mô tả ngắn |
|-----|-----|-----------|
| `moving_average` | Moving Average | VWMA trend slope + RSI momentum scaling + StochRSI overlay |
| `ema` | EMA/MACD | EMA slope + MACD momentum boost + Bollinger %B mean-reversion |
| `lstm_nn` | LSTM | PyTorch LSTM (2 layers, hidden=64, seq=60, dropout=0.2) |
| `gru_nn` | GRU | PyTorch GRU (2 layers, hidden=64, seq=60, dropout=0.2) |
| `arima_garch` | ARIMA-GARCH | statsmodels ARIMA(2,1,2) + arch GARCH(1,1) |
| `egarch` | EGARCH | arch EGARCH(p=1, o=1, q=1) với HARX mean model |
| `sarima` | SARIMA | statsmodels SARIMA(1,1,1)(1,0,1,5) — seasonal period 5 |
| `lightgbm` | LightGBM | ~30 features; Optuna hyperopt (30 trials, timeout 120s) |
| `xgboost` | XGBoost | ~30 features; Optuna hyperopt (30 trials, timeout 120s) |
| `random_forest` | Random Forest | n_estimators=200, max_depth=8, min_samples_leaf=5 |
| `ensemble` | Ensemble | Equal-weight average của 10 base models |

## Walk-forward Historical Backtest

`POST /api/trigger/historical-backtest?train_window=30&step_size=6&market_key=ALL`

- Chạy walk-forward backtesting trên một hoặc tất cả markets (GOLD, NASDAQ100, CRYPTO, SP500)
- Fold 1: train trên `train_window` ngày đầu, predict `step_size` ngày tiếp theo
- Mỗi fold expand thêm `step_size` ngày; lặp đến hết lịch sử
- Mỗi fold dùng context cố định (không rolling) — simulate "dự đoán trước khi biết kết quả"
- Predictions lưu kèm `actual_price` và `accuracy` ngay (vì backtesting biết lịch sử)
- Xóa predictions `target_date < today` trước khi insert — idempotent khi gọi lại
- Batch insert 200 rows/lần

## Test

```bash
# Go API Backend — chạy từ thư mục api/
cd api && go test ./...
cd api && make test-coverage   # tạo coverage.html

# Python Prediction Service
cd prediction && make test
# hoặc trong container:
docker exec prediction_service python -m pytest tests/ -v
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
| HTTP server (Go) | `net/http` (stdlib) |
| gRPC | `google.golang.org/grpc` + protobuf |
| ORM (Go) | GORM v2 + MySQL driver |
| Logging (Go) | ZeroLog |
| Số thực tài chính (Go) | `shopspring/decimal` |
| Config (Go) | `joho/godotenv` |
| ML models (Python) | PyTorch, statsmodels, arch, scikit-learn, LightGBM, XGBoost |
| Hyperopt (Python) | Optuna (Bayesian search) |
| Cron jobs (Python) | APScheduler |
| Data (Python) | pandas, pandas-ta, SQLAlchemy |
| Crawling (Python) | yfinance, requests |
| Frontend | React + TypeScript + Vite |
