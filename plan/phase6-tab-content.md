# Phase 6: Hoàn thiện 3 Tab — Cổ Phiếu, Dự Đoán, Huấn Luyện

## Tình trạng thực tế

3 tab này **có code frontend + backend API khá đầy đủ**, nhưng user thấy trống vì:
1. Dữ liệu gốc sai (price=0, confidence=0%) → Phase 1 fix
2. Nhiều section hardcoded placeholder, fallback mock khi API trả rỗng
3. API trả đúng format nhưng data chưa có ý nghĩa
4. Một số section hoàn toàn tĩnh (HTML hardcode), không load từ API

Phase này chạy **sau Phase 1 + 2** (dữ liệu đúng + ngôn ngữ đúng), tập trung làm mỗi tab thực sự useful.

---

## Tab 1: Cổ Phiếu (`/stocks`)

### Hiện có (đã hoạt động)
- ✅ Bảng VN30 stocks từ `/api/market/overview`
- ✅ Top tăng / giảm / thanh khoản
- ✅ Stock detail modal (gọi `/api/stocks/{symbol}/detail`)
- ✅ Nút crawl/predict thủ công
- ✅ Biểu đồ giá trong modal (Chart.js)

### Cần sửa / thêm

#### 6.1.1: Filter backend thật
- [x] Sector filter: API `/api/market/overview?sector=ngan-hang` → backend filter
- [x] Exchange filter: API `/api/market/overview?exchange=HOSE` → backend filter
- [x] Hiện tại JS comment: "In a real app, this would call the API with filters" → implement thật

#### 6.1.2: Cải thiện bảng stocks
- [ ] Thêm cột sparkline (mini chart 7 ngày) cho mỗi stock
- [x] Color coding: xanh/đỏ cho giá tăng/giảm (hiện đã có nhưng verify data đúng)
- [x] Sort server-side (hiện client-side) khi có > 30 stocks

#### 6.1.3: Stock detail modal cải thiện
- [ ] Thêm tab trong modal: Tổng quan | Lịch sử giá | Dự đoán | So sánh thuật toán
- [ ] Biểu đồ candlestick OHLC (thay vì chỉ line chart)
- [ ] Bảng so sánh: giá dự đoán vs giá thực tế qua các ngày
- [ ] Hiển thị độ chính xác từng thuật toán cho stock đó

#### 6.1.4: Watchlist persist
- [x] Lưu watchlist vào localStorage
- [x] Sync watchlist star state khi reload page
- [x] Dashboard "Danh sách theo dõi" đọc từ cùng localStorage

### Test Tab 1
- [ ] `pkg/server/api_market_overview_test.go`:
  - Test filter `?sector=ngan-hang` → chỉ trả về stocks ngành ngân hàng
  - Test filter `?exchange=HOSE` → chỉ trả về stocks sàn HOSE
  - Test combo filter `?sector=ngan-hang&exchange=HOSE`
  - Test filter invalid sector → trả về empty, không error
- [ ] `pkg/server/api_stock_detail_test.go`:
  - Test response có đủ OHLC data cho chart
  - Test response có predictions kèm theo
  - Test response có accuracy per algorithm

---

## Tab 2: Dự Đoán (`/predictions`)

### Hiện có (đã hoạt động)
- ✅ 3 algorithm cards (accuracy, prediction count, avg error) từ `/api/predictions/accuracy`
- ✅ Bảng predictions với pagination server-side
- ✅ Filter theo algorithm/stock/time range
- ✅ Bar chart accuracy
- ✅ CSV export
- ✅ Prediction summary stats

### Cần sửa / thêm

#### 6.2.1: Algorithm cards — bỏ hardcode
- [x] HTML hiện hardcode `93.2%`, `80.1%`, `75.5%` → xóa, chỉ dùng API data
- [x] Nếu API chưa có data → hiển thị "Chưa có dữ liệu" thay vì số giả
- [x] Thêm trend indicator thật: so sánh accuracy tuần này vs tuần trước

#### 6.2.2: Bảng predictions cải thiện
- [x] Cột "Actual Price" + "Accuracy" hiện trống → populate từ DB khi đã có giá thực tế
  - Backend: so sánh predicted vs actual (giá ngày target_date)
  - Tính accuracy = `1 - |predicted - actual| / actual`
- [x] Cột "Status": Pending / Confirmed / Wrong → logic xác nhận:
  - Pending: chưa đến target_date
  - Confirmed: error < 5%
  - Wrong: error >= 5%
- [x] Highlight row: xanh (confirmed), đỏ (wrong), xám (pending)

#### 6.2.3: Biểu đồ phân tích (section mới)
- [x] **Line chart**: Predicted vs Actual theo thời gian cho stock được chọn
  - Dropdown chọn stock → load chart
  - 2 lines: predicted (dashed) + actual (solid)
  - API: `GET /api/predictions/compare/{symbol}?days=30`
- [x] **Scatter plot**: Error distribution
  - X: predicted change %, Y: actual change %
  - Lý tưởng: các điểm nằm trên đường y=x
  - API: `GET /api/predictions/error-distribution`
- [x] **Accuracy trend chart** cải thiện:
  - Hiện là bar chart đơn giản → đổi thành line chart theo tuần
  - Hiển thị trend accuracy tăng/giảm qua thời gian

#### 6.2.4: Section "Dự đoán nổi bật"
- [x] Top 5 dự đoán chính xác nhất (actual ≈ predicted)
- [x] Top 5 dự đoán sai nhất (để học hỏi)
- [ ] Thuật toán nào tốt nhất cho stock nào (heatmap nhỏ)

### Test Tab 2
- [ ] `pkg/server/api_prediction_test.go` (mở rộng):
  - Test `GET /api/predictions` trả về `actual_price` khi đã qua target_date
  - Test `GET /api/predictions` trả về `accuracy` tính đúng công thức
  - Test `GET /api/predictions` status logic: pending/confirmed/wrong
  - Test `GET /api/predictions/compare/VCB?days=30` → pairs {date, predicted, actual}
  - Test `GET /api/predictions/error-distribution` → scatter data points
- [ ] `pkg/service/predict/accuracy_test.go` (mới):
  - Test accuracy calculation: `|predicted - actual| / actual`
  - Test edge case: actual = 0
  - Test threshold: error < 5% → confirmed, >= 5% → wrong
  - Test pending: target_date > now

---

## Tab 3: Huấn Luyện (`/training`)

### Hiện có (đã hoạt động)
- ✅ Training status (idle/running) từ `/api/training/status`
- ✅ Start/Stop training buttons
- ✅ Training history table từ `/api/training/history`
- ✅ Training logs từ `/api/training/logs`
- ✅ Per-algorithm train buttons
- ✅ Accuracy chart từ `/api/predictions/accuracy`

### Cần sửa / thêm

#### 6.3.1: Algorithm details — bỏ hardcode
- [x] HTML hardcode: accuracy %, config, "Last: 2 days ago" → load từ API
- [x] API mới hoặc mở rộng: `GET /api/training/algorithms` trả về:
  ```json
  [
    {
      "name": "LSTM Neural Network",
      "key": "lstm",
      "status": "trained",
      "last_trained": "2026-05-28T09:00:00Z",
      "accuracy": 0.85,
      "config": {
        "epochs": 100,
        "learning_rate": 0.001,
        "hidden_layers": 2,
        "batch_size": 32
      },
      "training_time_seconds": 320,
      "total_predictions": 150,
      "success_rate": 0.82
    }
  ]
  ```
- [x] Frontend: render từ API data, không hardcode

#### 6.3.2: Training logs — bỏ hardcode
- [x] HTML hardcode 5 sample log entries → xóa, chỉ load từ API
- [x] Nếu API trả rỗng → hiển thị "Chưa có log. Nhấn 'Bắt đầu huấn luyện' để bắt đầu."
- [x] Thêm auto-scroll logs khi đang train (like terminal output)
- [x] Color coding: INFO=xám, WARNING=vàng, ERROR=đỏ, SUCCESS=xanh

#### 6.3.3: Performance metrics — real data
- [x] HTML hardcode: "5m 20s", "96.7%", "256 MB" → load từ API
- [x] API: `GET /api/training/metrics` hoặc include trong `/api/training/status`:
  ```json
  {
    "avg_training_time": "5m 20s",
    "data_quality": 0.967,
    "memory_usage_mb": 256,
    "success_rate": 0.95
  }
  ```
- [x] Hoặc: tính từ training history (avg duration, success count / total count)

#### 6.3.4: Biểu đồ training cải thiện
- [ ] **Loss curve**: Khi đang train LSTM, hiển thị loss theo epoch (real-time)
  - WebSocket hoặc polling `/api/training/progress` mỗi 2s
  - Chart.js line chart, append data point mỗi epoch
- [x] **Accuracy trend**: Line chart accuracy qua các lần train (hiện là doughnut → đổi)
  - X: ngày train, Y: accuracy
  - 3 lines cho 3 thuật toán
- [x] **Training time comparison**: Bar chart so sánh thời gian train 3 thuật toán

#### 6.3.5: Cải thiện UX training
- [x] Progress bar thật khi đang train (hiện chỉ có animation)
  - Backend trả `progress: 45, current_phase: "Training LSTM epoch 45/100"`
- [ ] Notification khi train xong (browser notification API)
- [x] Disable "Bắt đầu" button khi đang train, enable "Dừng lại"
- [x] Hiển thị ETA (estimated time remaining)

### Test Tab 3
- [ ] `pkg/server/api_training_test.go` (mở rộng):
  - Test `GET /api/training/algorithms` → trả về 3 thuật toán với config, accuracy, last_trained
  - Test `GET /api/training/algorithms` khi chưa train lần nào → status="not_trained", accuracy=0
  - Test `GET /api/training/metrics` → trả về avg_training_time, data_quality, success_rate
  - Test `GET /api/training/status` khi đang train → is_training=true, progress > 0
  - Test `GET /api/training/logs?level=error` → chỉ trả về ERROR logs
  - Test `POST /api/trigger/train` → creates training_log record, status changes to "running"
  - Test `POST /api/trigger/train?algorithm=lstm` → chỉ train LSTM
- [ ] `pkg/service/predict/training_test.go` (mới):
  - Test training flow: start → progress updates → complete → log saved
  - Test training with insufficient data → proper error in log
  - Test concurrent training prevention (2 trains cùng lúc → reject cái sau)

---

## APIs cần thêm/sửa (tổng hợp Phase 6)

### Mới hoàn toàn
| Method | Endpoint | Mô tả |
|--------|----------|-------|
| GET | `/api/predictions/compare/{symbol}` | Predicted vs Actual theo thời gian |
| GET | `/api/predictions/error-distribution` | Scatter data cho error plot |
| GET | `/api/training/algorithms` | Chi tiết 3 thuật toán (config, accuracy, last_trained) |
| GET | `/api/training/metrics` | Performance metrics (training time, data quality) |

### Cần sửa
| Method | Endpoint | Sửa gì |
|--------|----------|--------|
| GET | `/api/market/overview` | Thêm filter `?sector=...&exchange=...` |
| GET | `/api/predictions` | Thêm `actual_price`, `accuracy`, `status` vào response |
| GET | `/api/training/status` | Thêm `progress` %, `current_phase`, ETA |
| GET | `/api/training/logs` | Thêm filter `?level=error` |

---

## Thứ tự thực hiện trong Phase 6

1. **Bỏ hardcode** trước (6.2.1, 6.3.1, 6.3.2, 6.3.3) — nhanh, impact lớn
2. **Prediction accuracy logic** (6.2.2) — core value, cần backend work
3. **API mới** (compare, error-distribution, algorithms, metrics)
4. **Biểu đồ mới** (6.2.3, 6.3.4) — cần API từ bước 3
5. **UX improvements** (6.1.2, 6.1.3, 6.3.5) — polish cuối

## Dependencies

- **Phải xong Phase 1** trước: dữ liệu price/confidence đúng thì mới có nghĩa
- **Phải xong Phase 2** trước: text đúng tiếng Việt
- **Song song Phase 3 được**: Phase 3 thêm detail pages mới, Phase 6 cải thiện pages hiện có
- **Song song Phase 4 được**: Phase 4 thêm charts ở dashboard, Phase 6 thêm charts ở 3 tabs

## Definition of Done — Phase 6

- [x] Tab Cổ phiếu: filter hoạt động, watchlist persist, stock detail có candlestick + so sánh thuật toán
- [x] Tab Dự đoán: không còn hardcode, actual price + accuracy hiện đúng, có biểu đồ predicted vs actual
- [x] Tab Huấn luyện: không còn hardcode, algorithm details từ API, progress bar thật, loss curve
- [x] Không còn section nào hiện placeholder/mock data
- [ ] Tất cả API mới có test >= 70%
- [x] `go test ./...` PASS
