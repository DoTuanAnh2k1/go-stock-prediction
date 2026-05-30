# Phase 1: Critical Bugs - Dữ liệu sai/thiếu

## Vấn đề 1.1: Current Price = 0 đ

**Triệu chứng**: Bảng Latest Predictions hiển thị "0 đ" cho tất cả cổ phiếu.

**Nguyên nhân gốc**:
- `pkg/service/predict/init.go`: Khi lưu prediction vào DB, `CurrentPrice` không được set đúng từ thuật toán.
- `pkg/server/api_prediction.go` → `GetPredictions()`: Không join với `stock_prices` để lấy giá hiện tại.

**Cần kiểm tra**:
- [ ] `pkg/service/predict/init.go` - Xem `CurrentPrice` có được gán từ kết quả prediction không
- [ ] `pkg/service/predict/moving_average/algo.go` - Xem có return `CurrentPrice` không
- [ ] `pkg/service/predict/lstm_nn/algo.go` - Tương tự
- [ ] `pkg/service/predict/arima_garch/algo.go` - Tương tự
- [ ] `pkg/models/models_db/prediction.go` - Xem struct `Prediction` có field `CurrentPrice` không
- [ ] API handler `GetPredictions()` - Xem có populate `CurrentPrice` từ bảng `stock_prices` không

**Giải pháp**:
1. Khi chạy prediction, lấy giá close mới nhất từ `stock_prices` và lưu vào `predictions.current_price`
2. Khi API trả về, nếu `current_price = 0`, fallback query giá mới nhất từ `stock_prices`

**Test**:
- [ ] `pkg/service/predict/predict_test.go`:
  - Test `runPrediction()` lưu `CurrentPrice != 0` khi có dữ liệu giá
  - Test `CurrentPrice = giá close mới nhất` (không phải 0 hay nil)
- [ ] `pkg/server/api_prediction_test.go`:
  - Test `GET /api/predictions` trả về `current_price > 0` cho stock có dữ liệu
  - Test `GET /api/predictions` trả về `current_price = 0` chỉ khi DB thật sự trống

---

## Vấn đề 1.2: Confidence = 0%

**Triệu chứng**: Tất cả predictions hiển thị Confidence 0%.

**Nguyên nhân gốc**:
- Moving Average (`moving_average/algo.go`): Không set field `Confidence` trong return value
- LSTM/ARIMA: `GetAccuracy()` trả về 0 vì chưa có backtest thực tế
- `pkg/service/predict/init.go` dòng ~298: `Confidence: decimal.NewFromFloat(algorithm.GetAccuracy())` - nhưng `GetAccuracy()` = 0

**Giải pháp**:
1. **Moving Average**: Tính confidence dựa trên độ lệch giữa predicted vs actual trong N ngày gần nhất (backtest đơn giản)
2. **LSTM**: Tính confidence từ loss function sau khi train
3. **ARIMA-GARCH**: Tính confidence từ AIC/BIC hoặc residual analysis
4. Nếu chưa có đủ dữ liệu backtest → hiển thị "N/A" thay vì "0%"

**Test**:
- [ ] `pkg/service/predict/moving_average/algo_test.go`:
  - Test VWMA calculation với dữ liệu mẫu (giá đã biết → kết quả tính tay)
  - Test confidence > 0 khi có đủ dữ liệu lịch sử
  - Test confidence = 0 (hoặc N/A) khi ít dữ liệu
  - Test edge cases: volume = 0, giá không đổi, chỉ 1 ngày dữ liệu
- [ ] `pkg/service/predict/lstm_nn/algo_test.go`:
  - Test prediction output format đầy đủ các field
  - Test confidence > 0 sau khi train
  - Test với dữ liệu đủ/không đủ
- [ ] `pkg/service/predict/arima_garch/algo_test.go`:
  - Test ARIMA fitting với time series đơn giản
  - Test GARCH volatility estimation
  - Test confidence > 0

---

## Vấn đề 1.3: Giá cổ phiếu hiển thị sai đơn vị

**Triệu chứng**: VCB hiển thị "64 đ" thay vì "64,000 đ" hoặc "64.00" (x1000 VND).

**Cần kiểm tra**:
- [ ] Dữ liệu trong DB: `SELECT close_price FROM stock_prices WHERE stock_id = ... ORDER BY date DESC LIMIT 5`
- [ ] Crawler lưu giá ở đơn vị nào (VND hay x1000 VND)
- [ ] Frontend format function có nhân lại x1000 không

**Giải pháp**:
- Thống nhất: DB lưu giá thực (VND), frontend format bằng `toLocaleString('vi-VN')`

**Test**:
- [ ] `pkg/service/crawler/crawler_test.go`:
  - Test parse giá từ HTML fixture → verify đơn vị đúng (VND)
  - So sánh giá parse được vs giá thật trên VietStock tại thời điểm lưu fixture

---

## Vấn đề 1.4: Mock data lẫn với real data

**Triệu chứng**: Dashboard stats (87.3% accuracy, 250 predictions) có thể là giả.

**Các vị trí mock data**:
- `pkg/server/api_market_overview.go`: VN30 Index hardcoded `1200.50`
- `web/static/js/dashboard.js`: `updateMockStats()` function
- `web/static/js/stock.js`: `renderSampleStocks()` function
- `web/static/js/dashboard.js`: ML Training Status section - accuracy values có thể hardcoded

**Giải pháp**:
1. Xóa tất cả mock data functions
2. Nếu chưa có dữ liệu thật → hiển thị "Chưa có dữ liệu" hoặc "—"
3. API `market-overview` phải query từ DB thật, không hardcode

**Test**:
- [ ] `pkg/server/api_market_overview_test.go`:
  - Test với DB trống → response chứa giá trị 0 hoặc null, KHÔNG hardcode
  - Test với DB có data → response match dữ liệu DB
  - Grep codebase: assert KHÔNG còn magic numbers (1200.50, 5.25, etc.)

---

## Vấn đề 1.5: Stock detail không hoạt động

**Triệu chứng**: Click vào stock → alert("Stock details for SHB - Coming soon!")

→ Chuyển sang Phase 3 (Detail Pages). Ở phase này chỉ cần xóa alert placeholder.

---

## Checklist thực hiện Phase 1

### Backend
- [ ] Fix prediction save logic - đảm bảo `CurrentPrice` được lưu đúng
- [ ] Fix confidence calculation cho Moving Average
- [ ] Fix confidence calculation cho LSTM
- [ ] Fix confidence calculation cho ARIMA-GARCH
- [ ] Xóa mock data trong `api_market_overview.go`
- [ ] Thêm API endpoint lấy giá hiện tại của stock

### Frontend
- [ ] Fix format giá (đúng đơn vị VND)
- [ ] Xóa `updateMockStats()` trong `dashboard.js`
- [ ] Xóa `renderSampleStocks()` trong `stock.js`
- [ ] Hiển thị "—" hoặc "Chưa có dữ liệu" khi giá trị = 0

### Database
- [ ] Verify dữ liệu `stock_prices` có đúng đơn vị không
- [ ] Verify `predictions` table có `current_price` column không

### Test (PHẢI hoàn thành trước khi đóng Phase 1)
- [ ] `pkg/service/predict/moving_average/algo_test.go` — VWMA + confidence
- [ ] `pkg/service/predict/lstm_nn/algo_test.go` — prediction + confidence
- [ ] `pkg/service/predict/arima_garch/algo_test.go` — prediction + confidence
- [ ] `pkg/service/predict/predict_test.go` — save logic (CurrentPrice)
- [ ] `pkg/server/api_prediction_test.go` — API response format
- [ ] `pkg/server/api_market_overview_test.go` — no mock data
- [ ] `pkg/service/crawler/crawler_test.go` — price unit verification
- [ ] `go test ./... ` PASS

## Definition of Done — Phase 1
- [ ] Current Price hiển thị đúng (> 0 khi có dữ liệu)
- [ ] Confidence hiển thị > 0% (hoặc "N/A" khi chưa đủ data)
- [ ] Giá đúng đơn vị VND
- [ ] Không còn mock/hardcoded data trong backend
- [ ] Tất cả test pass, coverage algorithms >= 80%
