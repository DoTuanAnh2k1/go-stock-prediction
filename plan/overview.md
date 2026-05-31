# Plan: Thêm NASDAQ 100, Bitcoin & Giá Xăng Markets

## Tổng quan

Thêm 3 market mới vào hệ thống dự đoán, theo đúng pattern AssetMarket đã có (VN30, GOLD):

| Market | Key | Data Source | Instruments |
|--------|-----|-------------|-------------|
| **NASDAQ 100** | `NASDAQ100` | Yahoo Finance API (đã dùng cho XAU) | Top 15 cổ phiếu (AAPL, MSFT, GOOGL, AMZN, NVDA, ...) |
| **Bitcoin/Crypto** | `CRYPTO` | CoinGecko API (free, không cần key) | BTC/USD, ETH/USD |
| **Giá Xăng VN** | `FUEL` | giaxanghomnay.com API (JSON, free) | RON 95-III, E5 RON 92, Diesel, Dầu hỏa |

## Thứ tự thực hiện

Ba market **độc lập hoàn toàn** — có thể chạy song song. Mỗi market theo 8 bước trong `ADDING_NEW_MARKET.md`.

**Phase 1 — Backend (bắt buộc):** DB model → Repository → Crawler → AssetMarket → Blank import
**Phase 2 — API (bắt buộc):** gRPC trigger → HTTP trigger endpoints → Data query endpoints
**Phase 3 — Frontend (tùy chọn):** Page UI với KPI, chart, predictions table

## Kế hoạch chi tiết

- [Phase 1a: NASDAQ 100 Backend](phase1_nasdaq100.md)
- [Phase 1b: Bitcoin/Crypto Backend](phase1_crypto.md)
- [Phase 1c: Giá Xăng VN Backend](phase1_fuel.md)
- [Phase 2: gRPC & API Endpoints](phase2_api.md)
- [Phase 3: Frontend Pages](phase3_frontend.md)

## Data Sources

### Yahoo Finance (NASDAQ 100)
- **URL:** `https://query1.finance.yahoo.com/v8/finance/chart/{SYMBOL}?interval=1d&range=6mo`
- **Đã sử dụng:** Trong `gold_crawler.go` cho XAU — code fetch + parse đã có sẵn
- **Rate limit:** ~2000 req/ngày (không cần API key)
- **Lưu ý:** Yahoo Finance có thể chặn IP nếu crawl quá nhanh — cần delay giữa các request

### CoinGecko (Bitcoin/Crypto)
- **URL:** `https://api.coingecko.com/api/v3/simple/price?ids=bitcoin,ethereum&vs_currencies=usd`
- **History:** `https://api.coingecko.com/api/v3/coins/{id}/market_chart?vs_currency=usd&days=180`
- **Free tier:** 10-30 req/phút, không cần API key
- **Ưu điểm:** Stable, free, dữ liệu đầy đủ, có OHLCV

### giaxanghomnay.com (Giá Xăng VN)
- **Lịch sử:** `GET https://giaxanghomnay.com/api/chart` — 467+ records từ 08/2018, 1 request
- **Daily:** `GET https://giaxanghomnay.com/api/pvdate/{YYYY-MM-DD}` — giá theo ngày
- **Free, không cần key, JSON thuần**
- **Fields:** `a`=RON 95-III, `b`=E5 RON 92, `c`=DO 0,05S (Diesel), `d`=Dầu hỏa (đơn vị: nghìn VND/lít)
- **Đặc thù:** Giá thay đổi ~7 ngày/lần (52 kỳ/năm), không phải daily như stock
- **Fallback:** webgia.com/gia-xang-dau/petrolimex/ (HTML scrape)

## Cron Schedule mới

| Job Key | Schedule mặc định | Công việc |
|---------|-------------------|-----------|
| `crawler_nasdaq` | `0 30 22 * * 1-5` (22:30 T2-T6 giờ VN = 10:30 ET) | Crawl NASDAQ sau khi thị trường Mỹ mở |
| `crawler_crypto` | `0 0 */4 * * *` (mỗi 4 giờ) | Crawl crypto (24/7 market) |
| `crawler_fuel` | `0 0 20 * * *` (20:00 hàng ngày) | Crawl giá xăng (sau 15:00 giờ điều chỉnh) |

**Prediction** không cần cron riêng — `predict_daily` (6PM) tự động chạy `orchestrator.RunAllMarkets()` bao gồm tất cả market mới.

## Files sẽ tạo/sửa

### Files MỚI (tạo)
```
# NASDAQ 100
pkg/models/models_db/nasdaq_price.go
pkg/store/mysql/nasdaq.go
pkg/service/crawler/nasdaq_crawler.go
pkg/service/market/nasdaq100/market.go

# Crypto (Bitcoin + ETH)
pkg/models/models_db/crypto_price.go
pkg/store/mysql/crypto.go
pkg/service/crawler/crypto_crawler.go
pkg/service/market/crypto/market.go

# Giá Xăng VN
pkg/models/models_db/fuel_price.go
pkg/store/mysql/fuel.go
pkg/service/crawler/fuel_crawler.go
pkg/service/market/fuel/market.go

# API endpoints
pkg/server/api_nasdaq.go
pkg/server/api_nasdaq_prediction.go
pkg/server/api_trigger_nasdaq_crawler.go
pkg/server/api_crypto.go
pkg/server/api_crypto_prediction.go
pkg/server/api_trigger_crypto_crawler.go
pkg/server/api_fuel.go
pkg/server/api_fuel_prediction.go
pkg/server/api_trigger_fuel_crawler.go
```

### Files SỬA (edit)
```
pkg/models/models_db/migrations.go          # Thêm NasdaqPrice, CryptoPrice, FuelPrice vào AllModels
pkg/store/repository/repository.go          # Thêm 6 store interfaces mới
pkg/service/crawler/init.go                 # Thêm 3 crawler vào crawlerDefaults
cmd/prediction/main.go                      # Thêm 3 blank imports
proto/prediction/prediction.proto           # Thêm 6 RPCs mới
pkg/grpc/server/server.go                   # Implement RPC handlers mới
pkg/server/router.go                        # Đăng ký routes mới
frontend/src/App.tsx                        # Thêm routes /markets/nasdaq100, /markets/fuel
frontend/src/pages/Crypto.tsx               # Thay placeholder bằng full page
frontend/src/components/Sidebar.tsx (hoặc nav) # Thêm nav links
```

## Quyết định kiến trúc

1. **Dùng chung bảng predictions hay tách?**
   → **Tách riêng** (`nasdaq_predictions`, `crypto_predictions`, `fuel_predictions`) — theo pattern Gold. Mỗi market có schema riêng.

2. **NASDAQ 100 đầy đủ 100 cổ phiếu?**
   → **Bắt đầu 15** cổ phiếu lớn nhất để tránh rate limit Yahoo Finance. Mở rộng sau.

3. **Crypto chỉ BTC hay nhiều hơn?**
   → **BTC + ETH** ban đầu. CoinGecko hỗ trợ thêm dễ dàng.

4. **Timezone NASDAQ?**
   → Market Mỹ mở 9:30-16:00 ET. Crawler chạy 22:30 giờ VN (= 10:30 ET) để đảm bảo có dữ liệu. Chỉ crawl T2-T6.

5. **Giá xăng — dữ liệu thưa?**
   → ~52 kỳ/năm (vs 250 phiên stock). 9 tháng ≈ 39 data points > 20 minimum. Thuật toán vẫn hoạt động nhưng accuracy có thể thấp hơn.

6. **Giá xăng — đơn vị?**
   → Lưu nghìn VND/lít (24.150) theo đúng format API gốc. Frontend xử lý hiển thị.
