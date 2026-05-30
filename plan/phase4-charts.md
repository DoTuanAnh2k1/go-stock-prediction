# Phase 4: Biểu Đồ & Visualization

## Hiện trạng

- Dashboard không có biểu đồ nào thật sự
- `predictions.js` có `loadAccuracyChart()` nhưng vẽ fake bars trên canvas
- Trang giá vàng dùng Chart.js cho XAU/USD nhưng chỉ có 1 data point
- Không có biểu đồ giá cổ phiếu, không có so sánh thuật toán

## Thư viện

Dùng **Chart.js** (đã được import trong gold.html). Cần import thêm vào các trang khác.

## Biểu đồ cần thêm

### 4.1: Dashboard - Biểu đồ tổng quan
- [ ] **Mini sparkline** cho mỗi stock trong watchlist (giá 7 ngày gần nhất)
- [ ] **Pie chart** accuracy theo thuật toán (MA vs LSTM vs ARIMA)
- [ ] **Bar chart** số lượng predictions theo ngày (7 ngày gần nhất)

### 4.2: Trang Cổ phiếu - Biểu đồ giá
- [ ] **Line chart** giá đóng cửa 30/60/90 ngày (có toggle)
- [ ] **Candlestick chart** (OHLC) cho từng stock (dùng chartjs-chart-financial plugin)
- [ ] **Volume bar chart** bên dưới price chart
- [ ] Overlay dự đoán lên biểu đồ giá thật (predicted vs actual)

### 4.3: Trang Dự đoán - Biểu đồ phân tích
- [ ] **Line chart**: Predicted vs Actual price theo thời gian cho từng stock
- [ ] **Bar chart**: Accuracy comparison giữa 3 thuật toán
- [ ] **Scatter plot**: Prediction error distribution
- [ ] **Heatmap** (nếu có đủ dữ liệu): Accuracy theo stock x algorithm

### 4.4: Trang Huấn luyện - Biểu đồ training
- [ ] **Line chart**: Loss curve theo epochs (cho LSTM)
- [ ] **Line chart**: Accuracy trend qua các lần train
- [ ] **Bar chart**: Training time comparison giữa thuật toán

### 4.5: Trang Giá Vàng - Cải thiện biểu đồ
- [ ] Fix biểu đồ XAU/USD - cần nhiều data points hơn
- [ ] Thêm biểu đồ SJC 1 Lượng thật (hiện "Chưa có dữ liệu")
- [ ] Thêm biểu đồ so sánh SJC vs XAU/USD (dual axis)

## Backend APIs cần cho biểu đồ

- [ ] `GET /api/stocks/{symbol}/prices?days=30` → time series data cho chart
- [ ] `GET /api/predictions/accuracy-by-algorithm` → aggregate accuracy
- [ ] `GET /api/predictions/compare/{symbol}` → predicted vs actual
- [ ] `GET /api/training/accuracy-trend` → accuracy qua thời gian
- [ ] `GET /api/dashboard/stats` → real stats cho dashboard cards

## Lưu ý kỹ thuật

- Responsive: biểu đồ phải resize theo screen
- Loading state: skeleton hoặc spinner khi load data
- Empty state: hiển thị message khi chưa có đủ dữ liệu cho biểu đồ
- Format số: dùng `toLocaleString('vi-VN')` cho giá VND

---

## Test cho Phase 4

### Backend API Tests
- [ ] `pkg/server/api_chart_data_test.go`:
  - Test `GET /api/stocks/VCB/prices?days=30` → trả về array <= 30 items, mỗi item có `date`, `close`, `open`, `high`, `low`, `volume`
  - Test `GET /api/stocks/VCB/prices?days=0` → 400 bad request
  - Test `GET /api/predictions/accuracy-by-algorithm` → trả về 3 thuật toán với accuracy
  - Test `GET /api/predictions/compare/VCB` → trả về pairs `{predicted, actual, date}`
  - Test `GET /api/dashboard/stats` → trả về `total_stocks`, `total_predictions`, `avg_accuracy`, `last_update`
  - Test tất cả endpoints với DB trống → trả về empty array, không crash

### Repository Tests
- [ ] `pkg/store/mysql/mysql_test.go` (thêm):
  - Test `GetPriceHistory(stockID, 30)` → trả về data đúng thứ tự thời gian (ASC cho chart)
  - Test `GetAccuracyByAlgorithm()` → aggregate đúng
  - Test `GetPredictionComparison(symbol)` → join prediction + actual price

### Data Integrity Tests
- [ ] Test: chart data API trả về `date` format ISO 8601 (frontend Chart.js cần)
- [ ] Test: giá trị price luôn > 0 (không có negative prices)

## Definition of Done — Phase 4
- [ ] Dashboard có ít nhất 1 biểu đồ thật (accuracy pie chart hoặc sparkline)
- [ ] Trang cổ phiếu có line chart giá
- [ ] Trang dự đoán có bar chart accuracy comparison
- [ ] Biểu đồ hiển thị empty state khi chưa có dữ liệu
- [ ] Chart data APIs có test >= 70%
- [ ] `go test ./...` PASS
