# Phase 5: Trang Giá Vàng

## Hiện trạng

- Chỉ có dữ liệu XAU/USD (1 data point duy nhất)
- SJC 1 Lượng: "Chưa có dữ liệu"
- SJC Nhẫn Tròn: "Chưa có dữ liệu"
- Biểu đồ XAU/USD chỉ có 1 điểm → không có ý nghĩa
- Bảng giá chi tiết SJC trống

## Vấn đề cốt lõi

### 5.1: Crawler SJC không hoạt động
- [ ] Kiểm tra `pkg/service/crawler/gold_crawler.go` - logic crawl SJC
- [ ] SJC website có thể đã thay đổi cấu trúc HTML → crawler parse sai
- [ ] Cần verify URL crawl và CSS selectors
- [ ] Thêm error logging chi tiết khi crawl thất bại
- [ ] Thêm retry mechanism

### 5.2: Dữ liệu XAU/USD thiếu
- [ ] Chỉ có 1 record trong DB → biểu đồ vô nghĩa
- [ ] Cần crawl lịch sử XAU/USD (ít nhất 30 ngày)
- [ ] Kiểm tra cron schedule cho gold crawler
- [ ] Source XAU/USD: cần API đáng tin cậy (không chỉ scraping)

### 5.3: Thiếu dự đoán giá vàng
- [ ] Hiện chỉ hiển thị giá hiện tại, không có dự đoán
- [ ] Cần thêm thuật toán dự đoán cho vàng (có thể reuse Moving Average)
- [ ] Hoặc: chỉ hiển thị trend/phân tích kỹ thuật đơn giản

## Cần làm

### Backend
- [ ] Fix gold crawler cho SJC (kiểm tra selectors)
- [ ] Thêm crawl historical data cho SJC (backfill)
- [ ] Fix XAU/USD crawler - đảm bảo chạy hàng ngày và lưu đúng
- [ ] API `GET /api/gold/prices?type=sjc_1l&days=30` - trả time series
- [ ] API `GET /api/gold/latest` - trả giá mới nhất tất cả loại vàng

### Frontend
- [ ] Hiển thị đúng giá SJC khi có data
- [ ] Biểu đồ SJC có nhiều data points
- [ ] Biểu đồ XAU/USD có nhiều data points
- [ ] Bảng giá chi tiết có dữ liệu thật
- [ ] Thêm so sánh giá mua/bán SJC
- [ ] Format giá VND cho SJC (x triệu đồng/lượng)

### Cải thiện UX
- [ ] Auto-refresh giá vàng mỗi 5 phút (khi market mở)
- [ ] Hiển thị xu hướng (lên/xuống) bằng icon arrow
- [ ] Highlight thay đổi giá bằng animation (flash xanh/đỏ)

---

## Test cho Phase 5

### Crawler Tests
- [ ] `pkg/service/crawler/gold_crawler_test.go`:
  - Test parse SJC HTML fixture → extract đúng giá mua/bán cho SJC 1 Lượng
  - Test parse SJC HTML fixture → extract đúng giá cho SJC Nhẫn Tròn
  - Test parse XAU/USD response → extract đúng giá
  - Test với HTML cũ/thay đổi structure → error handling không panic
  - Test retry mechanism khi request fail
  - Lưu HTML fixtures: `testdata/sjc_sample.html`, `testdata/xau_sample.json`

### API Tests
- [ ] `pkg/server/api_gold_test.go`:
  - Test `GET /api/gold/latest` → trả về SJC 1 Lượng, SJC Nhẫn Tròn, XAU/USD
  - Test `GET /api/gold/latest` khi DB trống → trả về empty, không crash
  - Test `GET /api/gold/prices?type=sjc_1l&days=30` → trả về <= 30 records
  - Test `GET /api/gold/prices?type=invalid` → 400
  - Test format giá: SJC giá VND (triệu), XAU/USD giá USD

### Repository Tests
- [ ] `pkg/store/mysql/gold_price_test.go`:
  - Test save gold price → retrieve đúng
  - Test `GetLatestGoldPrices()` → trả về giá mới nhất mỗi loại
  - Test `GetGoldPriceHistory(type, days)` → trả về đúng số records, đúng thứ tự

### Integration Test
- [ ] Test crawl → save → API query → verify data consistency
- [ ] Test cron trigger gold crawl → verify new records in DB

## Definition of Done — Phase 5
- [ ] SJC 1 Lượng hiển thị giá mua/bán thật
- [ ] SJC Nhẫn Tròn hiển thị giá mua/bán thật
- [ ] XAU/USD có >= 7 data points trên biểu đồ
- [ ] Bảng giá chi tiết có dữ liệu
- [ ] Gold crawler tests >= 60% coverage
- [ ] Gold API tests >= 70% coverage
- [ ] `go test ./...` PASS
