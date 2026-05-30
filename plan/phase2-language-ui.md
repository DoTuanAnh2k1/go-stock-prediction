# Phase 2: Ngôn ngữ & UI - Thống nhất tiếng Việt có dấu

## Vấn đề chính

Toàn bộ giao diện lẫn lộn:
- Tiếng Anh: "Dashboard", "Stock Watchlist", "ML Training Status", "Refresh", "Latest Predictions"
- Tiếng Việt không dấu: "Co phieu", "Du doan", "Gia Vang", "Lam moi", "Thu thap du lieu"
- Tiếng Việt có dấu (ít): "Hệ thống đang tự động đồng bộ dữ liệu..."

## Quyết định thiết kế

**Option A**: Toàn bộ tiếng Việt có dấu — vì target users là nhà đầu tư Việt Nam, dữ liệu từ sàn HOSE/HNX.

## Bảng chuyển đổi ngôn ngữ

### Navigation
| Hiện tại | Sửa thành |
|----------|-----------|
| Dashboard | Tổng quan |
| Co phieu | Cổ phiếu |
| Du doan | Dự đoán |
| Training | Huấn luyện |
| Gia Vang | Giá Vàng |

### Dashboard
| Hiện tại | Sửa thành |
|----------|-----------|
| VN Stock Prediction Dashboard | Dự Đoán Giá Cổ Phiếu Việt Nam |
| Real-time Vietnamese stock market analysis with ML predictions | Phân tích thị trường chứng khoán bằng Machine Learning |
| Market Closed / Market Open | Thị trường đóng cửa / Thị trường mở cửa |
| VN30 Stocks - Actively monitored | Cổ phiếu VN30 - Đang theo dõi |
| Total Predictions - Last 7 days | Tổng dự đoán - 7 ngày qua |
| Average Accuracy - All algorithms | Độ chính xác TB - Tất cả thuật toán |
| Last Update - Next in 3m | Cập nhật lần cuối - Tiếp theo sau 3 phút |
| Stock Watchlist | Danh sách theo dõi |
| ML Training Status | Trạng thái huấn luyện ML |
| Latest Predictions | Dự đoán mới nhất |
| Refresh | Làm mới |
| Current Price | Giá hiện tại |
| Predicted Price | Giá dự đoán |
| Change | Thay đổi |
| Algorithm | Thuật toán |
| Confidence | Độ tin cậy |
| Target Date | Ngày dự đoán |
| Trained | Đã huấn luyện |

### Trang Cổ phiếu
| Hiện tại | Sửa thành |
|----------|-----------|
| VN30 Stocks | Cổ phiếu VN30 |
| Search stocks... | Tìm kiếm cổ phiếu... |
| All Sectors | Tất cả ngành |
| Sort by | Sắp xếp theo |
| Add to Watchlist | Thêm vào danh sách |
| View Details | Xem chi tiết |

### Trang Giá Vàng
| Hiện tại | Sửa thành |
|----------|-----------|
| Gia Vang Hien Tai | Giá Vàng Hiện Tại |
| Lam moi | Làm mới |
| Thu thap du lieu | Thu thập dữ liệu |
| SJC 1 LUONG | SJC 1 Lượng |
| SJC NHAN TRON | SJC Nhẫn Tròn |
| Chua co du lieu | Chưa có dữ liệu |
| Bieu do SJC 1 luong (30 ngay) | Biểu đồ SJC 1 Lượng (30 ngày) |
| Bieu do XAU/USD (30 ngay) | Biểu đồ XAU/USD (30 ngày) |
| Bang gia chi tiet SJC 1 luong | Bảng giá chi tiết SJC 1 Lượng |
| Gia Mua | Giá Mua |
| Gia Ban | Giá Bán |
| Chenh lech | Chênh lệch |
| Cap nhat luc | Cập nhật lúc |

## Files cần sửa

### Templates (HTML)
- [ ] `web/templates/dashboard.html` - Navigation, stats cards, table headers, labels
- [ ] `web/templates/stock.html` - Toàn bộ text
- [ ] `web/templates/predictions.html` - Toàn bộ text
- [ ] `web/templates/training.html` - Toàn bộ text
- [ ] `web/templates/gold.html` - Toàn bộ text (nhiều nhất)

### JavaScript
- [ ] `web/static/js/dashboard.js` - Alert messages, dynamic text
- [ ] `web/static/js/stock.js` - Alert messages, filter labels
- [ ] `web/static/js/predictions.js` - Dynamic text, status labels
- [ ] `web/static/js/training.js` - Dynamic text
- [ ] `web/static/js/gold.js` - Alert messages, status text
- [ ] `web/static/js/common.js` - Market status text

### Backend (nếu có text hardcoded)
- [ ] `pkg/server/web_handler.go` - Page titles truyền vào template
- [ ] `pkg/server/api_*.go` - Error messages (giữ tiếng Anh cho API, chỉ sửa UI text)

## Encoding
- Đảm bảo tất cả templates có `<meta charset="UTF-8">`
- Go `html/template` mặc định UTF-8 → OK
- MySQL collation phải là `utf8mb4_unicode_ci`

---

## Test cho Phase 2

### Template Rendering Test
- [ ] `pkg/server/web_handler_test.go`:
  - Test render dashboard page → response body chứa "Tổng quan", "Cổ phiếu", "Dự đoán"
  - Test render gold page → response body chứa "Giá Vàng Hiện Tại", "Làm mới"
  - Test render stock page → response body chứa "Cổ phiếu VN30"
  - Assert KHÔNG chứa tiếng Việt không dấu: "Co phieu", "Du doan", "Gia Vang"
  - Assert KHÔNG chứa tiếng Anh cũ: "Stock Watchlist", "Latest Predictions"

### Encoding Test
- [ ] `pkg/server/web_handler_test.go`:
  - Test response header `Content-Type: text/html; charset=utf-8`
  - Test Vietnamese diacritics render correctly (không bị mojibake)

### Regression Test
- [ ] Grep toàn bộ `web/` directory: assert 0 matches cho các string cũ:
  - `"Co phieu"`, `"Du doan"`, `"Gia Vang"` (không dấu)
  - `"Lam moi"`, `"Thu thap du lieu"`, `"Chua co du lieu"`
  - Script: `grep -r "Co phieu\|Du doan\|Gia Vang\|Lam moi" web/` → phải trả về 0 kết quả

## Definition of Done — Phase 2
- [ ] Tất cả text UI là tiếng Việt có dấu
- [ ] Không còn tiếng Anh (trừ thuật ngữ kỹ thuật: LSTM, ARIMA, ML, API)
- [ ] Encoding UTF-8 hoạt động đúng
- [ ] Template rendering tests pass
- [ ] `go test ./... ` PASS
