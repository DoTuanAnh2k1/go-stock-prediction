# go-stock-prediction

Hệ thống dự đoán giá cổ phiếu thị trường chứng khoán Việt Nam (VN30/VN100), bao gồm crawl dữ liệu tự động, huấn luyện mô hình ML, và dashboard theo dõi.

## Tính năng

- **Thu thập dữ liệu:** Crawl giá cổ phiếu VN30/VN100 từ VietStock mỗi ngày lúc 12 PM
- **Dự đoán giá:** 3 thuật toán ML chạy song song — Moving Average, LSTM Neural Network, ARIMA-GARCH
- **Huấn luyện tự động:** Mỗi Chủ nhật lúc 9 AM, hệ thống tự train lại toàn bộ mô hình
- **Dashboard web:** Giao diện xem tổng quan thị trường, biểu đồ, và kết quả dự đoán
- **REST API:** Endpoints đầy đủ cho dữ liệu cổ phiếu, dự đoán, và trigger thủ công

## Kiến trúc

```
cmd/app/main.go          Entry point
pkg/
  config/                Quản lý cấu hình (.env)
  server/                HTTP server, routes, handlers
  service/
    crawler/             Thu thập dữ liệu từ VietStock (Gocolly)
    predict/             Dự đoán giá (MA, LSTM, ARIMA-GARCH)
    db/                  Business logic tầng service
  store/
    repository/          Interface repository pattern
    mysql/               Triển khai MySQL (GORM)
  models/
    models_db/           GORM struct (Stock, StockPrice, Prediction, ...)
    models_api/          DTO cho API response
  utils/                 Cron, JWT, bcrypt, env
web/
  templates/             HTML templates (Go template)
  static/css,js/         Frontend assets
cmd/crawdata/main.py     Script Python crawl dữ liệu lịch sử (vnstock API)
```

## Yêu cầu

- Go 1.23+
- MySQL 5.7+ (hoặc dùng Docker Compose)
- File `.env` cấu hình (xem bên dưới)

## Cài đặt & Chạy

### 1. Khởi động database

```bash
docker-compose up -d
```

Docker Compose sẽ khởi chạy MySQL (port 3306) và phpMyAdmin (port 8080).

### 2. Tạo file `.env`

```env
SERVER_NAME=go-stock-prediction
SERVER_HOST=0.0.0.0
SERVER_PORT=31300

DB_DRIVER=mysql
MYSQL_HOST=localhost
MYSQL_PORT=3306
MYSQL_USER=root
MYSQL_PASSWORD=123
MYSQL_DB_NAME=go_stock_prediction

LOG_LEVEL=DEBUG
DB_LOG_LEVEL=DEBUG
```

### 3. Import schema

```bash
mysql -u root -p go_stock_prediction < database.sql
```

### 4. (Tuỳ chọn) Crawl dữ liệu lịch sử bằng Python

```bash
pip install vnstock mysql-connector-python pandas
python cmd/crawdata/main.py
```

### 5. Chạy ứng dụng

```bash
go run ./cmd/app
# hoặc build trước
go build -o go-stock-prediction ./cmd/app && ./go-stock-prediction
```

Truy cập dashboard tại: `http://localhost:31300`

### Chạy bằng Docker

```bash
docker build -t go-stock-prediction .
docker run -p 31300:31300 --env-file .env go-stock-prediction
```

## API Endpoints

| Method | Path | Mô tả |
|--------|------|-------|
| GET | `/api/training/status` | Trạng thái huấn luyện hiện tại |
| GET | `/api/training/history` | Lịch sử huấn luyện |
| GET | `/api/predictions` | Danh sách dự đoán gần nhất |
| GET | `/api/algorithms/comparison` | So sánh độ chính xác các thuật toán |
| GET | `/api/stocks/{symbol}/chart` | Dữ liệu biểu đồ |
| GET | `/api/stocks/{symbol}/current` | Giá hiện tại |
| GET | `/api/stocks/{symbol}/history` | Lịch sử giá |
| GET | `/api/stocks/watchlist` | Danh sách theo dõi |
| GET | `/api/market/overview` | Tổng quan thị trường |
| POST | `/api/trigger/crawler` | Chạy crawler thủ công |
| POST | `/api/trigger/predict` | Chạy dự đoán thủ công |
| GET | `/health` | Health check chi tiết |
| GET | `/health/simple` | Health check đơn giản |

## Lịch chạy tự động

| Thời gian | Công việc |
|-----------|-----------|
| Mỗi ngày 12:00 PM | Crawl giá cổ phiếu từ VietStock |
| Mỗi ngày 6:00 PM | Chạy dự đoán giá cho ngày giao dịch tiếp theo |
| Chủ nhật 9:00 AM | Huấn luyện lại toàn bộ mô hình với 6 tháng dữ liệu |

## Các thuật toán dự đoán

### Moving Average (MA)
- Volume-Weighted Moving Average (VWMA)
- Short period: 5 ngày, Long period: 20 ngày
- Tích hợp điều chỉnh phiên giao dịch Việt Nam (sáng/chiều)
- Output: BUY/SELL/HOLD với điểm confidence

### LSTM Neural Network
- 2 lớp LSTM, 50 hidden units
- Sequence length: 60 ngày (~3 tháng)
- 8 features: giá, khối lượng, chỉ báo kỹ thuật
- Chuẩn hóa dữ liệu bằng MinMaxScaler

### ARIMA-GARCH
- ARIMA để dự đoán xu hướng giá
- GARCH để mô hình hoá biến động (volatility)

## Công nghệ sử dụng

| Lĩnh vực | Thư viện |
|----------|---------|
| HTTP server | `net/http` (stdlib) |
| ORM | GORM v2 + MySQL driver |
| Web scraping | Gocolly v2 |
| Cron jobs | `robfig/cron/v3` |
| Logging | ZeroLog |
| Số thực tài chính | `shopspring/decimal` |
| Config | `joho/godotenv` |
| Auth | `golang-jwt/jwt/v5`, bcrypt |
