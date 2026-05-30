# Phase 3: Trang Chi Tiết - Stock, Prediction, Training

## 3.1: Trang Chi Tiết Cổ Phiếu

**Hiện tại**: Click vào stock → `alert("Stock details for SHB - Coming soon!")`

### Backend
- [ ] Tạo API `GET /api/stocks/{symbol}` → trả về thông tin chi tiết:
  ```json
  {
    "symbol": "VCB",
    "name": "Ngân hàng Ngoại thương Việt Nam",
    "exchange": "HOSE",
    "current_price": 64000,
    "price_change": 500,
    "price_change_pct": 0.78,
    "volume": 1234567,
    "high": 65000,
    "low": 63500,
    "open": 63800,
    "historical_prices": [...],
    "predictions": [...]
  }
  ```
- [ ] Tạo file `pkg/server/api_stock_detail.go`
- [ ] Repository method: `GetStockDetail(symbol string)`, `GetLatestPrices(stockID, days int)`

### Frontend
- [ ] Tạo modal hoặc trang `/stock/{symbol}` với:
  - Thông tin cơ bản (tên, sàn, giá, khối lượng)
  - Biểu đồ giá 30/60/90 ngày (Chart.js line chart)
  - Bảng giá lịch sử (có pagination)
  - Dự đoán từ 3 thuật toán cho stock này
  - So sánh dự đoán vs thực tế (nếu có dữ liệu)

### Test 3.1
- [ ] `pkg/server/api_stock_detail_test.go`:
  - Test `GET /api/stocks/VCB` với DB có data → response đúng format
  - Test `GET /api/stocks/VCB` → `current_price`, `volume`, `high`, `low` đều > 0
  - Test `GET /api/stocks/INVALID` → 404
  - Test `historical_prices` trả về đúng số ngày
  - Test `predictions` trả về predictions cho đúng stock đó
- [ ] `pkg/store/mysql/mysql_test.go` (thêm):
  - Test `GetStockDetail("VCB")` trả về đúng thông tin
  - Test `GetLatestPrices(stockID, 30)` trả về <= 30 records, đúng thứ tự

---

## 3.2: Trang Dự Đoán (Tab "Dự đoán")

**Hiện tại**: Bảng predictions không có filter, pagination, biểu đồ, hay chi tiết.

### Backend
- [ ] Cải thiện API `GET /api/predictions` - thêm query params:
  - `?symbol=VCB` - lọc theo mã
  - `?algorithm=MA` - lọc theo thuật toán
  - `?from=2026-01-01&to=2026-05-29` - lọc theo ngày
  - `?page=1&limit=20` - pagination thật
- [ ] Tạo API `GET /api/predictions/{id}` - chi tiết 1 prediction
- [ ] Tạo API `GET /api/predictions/accuracy` - thống kê accuracy theo thuật toán

### Frontend
- [ ] Filter bar hoạt động thật (gọi API với params)
- [ ] Pagination thật (server-side)
- [ ] Biểu đồ accuracy theo thuật toán (bar chart)
- [ ] Biểu đồ predicted vs actual price (line chart) cho từng stock
- [ ] Modal chi tiết prediction khi click

### Test 3.2
- [ ] `pkg/server/api_prediction_test.go` (mở rộng):
  - Test filter `?symbol=VCB` → chỉ trả về predictions cho VCB
  - Test filter `?algorithm=MA` → chỉ trả về MA predictions
  - Test filter `?from=...&to=...` → chỉ trả về trong khoảng thời gian
  - Test pagination `?page=1&limit=5` → trả về tối đa 5 records + metadata
  - Test `GET /api/predictions/123` → đúng prediction
  - Test `GET /api/predictions/999` → 404
  - Test `GET /api/predictions/accuracy` → response có 3 thuật toán, accuracy >= 0

---

## 3.3: Trang Huấn Luyện (Tab "Huấn luyện")

**Hiện tại**: Layout có nhưng trống/mock, click chi tiết → alert.

### Backend
- [ ] Tạo bảng `training_logs` trong DB:
  ```sql
  CREATE TABLE training_logs (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    algorithm VARCHAR(50),
    started_at DATETIME,
    completed_at DATETIME,
    status ENUM('running', 'completed', 'failed'),
    accuracy DECIMAL(10,4),
    loss DECIMAL(10,4),
    epochs INT,
    details JSON,
    created_at DATETIME
  );
  ```
- [ ] Lưu log khi train (trong `predict/init.go`)
- [ ] API `GET /api/training/history` - danh sách training sessions
- [ ] API `GET /api/training/{id}` - chi tiết 1 session
- [ ] API `POST /api/trigger/train` - trigger train thủ công

### Frontend
- [ ] Bảng training history (algorithm, thời gian, status, accuracy)
- [ ] Biểu đồ accuracy trend theo thời gian
- [ ] Nút "Huấn luyện ngay" cho từng thuật toán
- [ ] Progress indicator khi đang train
- [ ] Chi tiết session: epochs, loss curve, hyperparameters

### Test 3.3
- [ ] `pkg/server/api_training_test.go`:
  - Test `GET /api/training/history` → danh sách sessions
  - Test `GET /api/training/1` → chi tiết session đúng format
  - Test `POST /api/trigger/train` → trigger thành công, tạo training_log record
  - Test `POST /api/trigger/train` khi đang train → trả về conflict/busy
- [ ] `pkg/store/mysql/training_test.go`:
  - Test CRUD training_logs
  - Test query history có pagination

---

## 3.4: Watchlist thật

**Hiện tại**: Nút "Add to Watchlist" chỉ show notification, không lưu.

- [ ] Dùng localStorage cho MVP (không cần backend)
- [ ] Dashboard Stock Watchlist query từ localStorage
- [ ] Nút thêm/xóa khỏi watchlist
- [ ] Sync watchlist giữa các tab (StorageEvent)

### Test 3.4
- Watchlist dùng localStorage → test ở frontend level (nếu có e2e test setup)
- Không cần backend test

---

## Thứ tự ưu tiên trong Phase 3
1. Stock detail page (có ích nhất cho người dùng)
2. Predictions tab cải thiện
3. Training tab cải thiện
4. Watchlist (localStorage)

## Definition of Done — Phase 3
- [ ] Click stock → hiện trang chi tiết với dữ liệu thật
- [ ] Tab Dự đoán có filter, pagination, biểu đồ accuracy
- [ ] Tab Huấn luyện hiện training history thật
- [ ] Watchlist lưu vào localStorage
- [ ] Tất cả API endpoints mới có test >= 80% coverage
- [ ] `go test ./...` PASS
