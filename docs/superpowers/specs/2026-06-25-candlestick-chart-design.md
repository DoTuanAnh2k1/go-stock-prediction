# Design: Line / Candlestick chart toggle cho biểu đồ giá

**Ngày:** 2026-06-25
**Trạng thái:** Approved design → implementation plan
**Phạm vi:** 4 market (CRYPTO, NASDAQ, SP500, GOLD-XAU), 4 tầng (DB → Python → Go → Frontend)

## 1. Mục tiêu

Cho phép người dùng chuyển đổi biểu đồ giá của từng mã (vd BTC trong CRYPTO) giữa **biểu đồ đường (line)** và **biểu đồ nến (candlestick)** kiểu sàn giao dịch. Nến là OHLC **thật**, hỗ trợ cả khung **ngày** và **giờ** (intraday), khớp với cơ chế `granularity` hiện có (`days==1` → intraday/`1h`, ngược lại → daily/`1d`).

## 2. Bức tranh dữ liệu (đã verify qua codebase)

| Market | Daily OHLC | Intraday OHLC | Việc cần ở data layer |
|---|---|---|---|
| NASDAQ | ✅ có sẵn (`nasdaq_prices`) | ✅ có sẵn (`nasdaq_intraday_prices`) | Không — chỉ expose ra API |
| SP500 | ✅ có sẵn (`sp500_prices`) | ✅ có sẵn (`sp500_intraday_prices`) | Không — chỉ expose ra API |
| CRYPTO | ❌ chỉ `close_price` | ❌ chỉ `price` | Thêm cột O/H/L; đổi crawler sang CoinGecko `/ohlc` |
| GOLD (XAU) | ❌ chỉ buy/sell | ❌ chỉ buy/sell | Thêm cột O/H/L; crawler **đã fetch OHLC**, chỉ cần lưu (đang vứt đi) |

GOLD nguồn VN (SJC/BTMC/Phú Quý/vang.today) **giữ line chart** — không có khái niệm OHLC, chỉ buy/sell spread. Toggle nến bị ẩn/disable khi chọn nguồn VN.

## 3. Quyết định thiết kế

- **Candlestick component**: tự viết SVG thuần trong `web-svc/src/components/charts.tsx`, khớp 100% convention hiện tại (`var(--up)`/`var(--down)`/`var(--grid-line)`, viewBox W=800, tooltip absolute, label theo `granularity`). Không thêm npm dependency.
- **Backfill**: chạy lại trigger `crypto-history` + `gold-history` sau deploy để lấp OHLC lịch sử. Row cũ chưa backfill có O/H/L=NULL → frontend fallback line cho đoạn đó.
- **Tương thích ngược**: API vẫn trả `prices`/`closes` (close) như cũ cho line chart và input dự đoán. Các thuật toán ML **không đổi** (vẫn dùng close series).

## 4. Thay đổi theo tầng

### 4.1. DB schema — `database.sql`

Dùng pattern `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` đã có sẵn trong file (vd dòng 408, 947-948). Thêm sau phần CREATE TABLE tương ứng:

```sql
-- Crypto OHLC (daily-live + intraday)
ALTER TABLE crypto_prices          ADD COLUMN IF NOT EXISTS open_price NUMERIC(20,2);
ALTER TABLE crypto_prices          ADD COLUMN IF NOT EXISTS high_price NUMERIC(20,2);
ALTER TABLE crypto_prices          ADD COLUMN IF NOT EXISTS low_price  NUMERIC(20,2);
ALTER TABLE crypto_intraday_prices ADD COLUMN IF NOT EXISTS open_price NUMERIC(30,8);
ALTER TABLE crypto_intraday_prices ADD COLUMN IF NOT EXISTS high_price NUMERIC(30,8);
ALTER TABLE crypto_intraday_prices ADD COLUMN IF NOT EXISTS low_price  NUMERIC(30,8);

-- Gold XAU OHLC (giữ buy/sell cho nguồn VN)
ALTER TABLE gold_prices            ADD COLUMN IF NOT EXISTS open_price NUMERIC(15,2);
ALTER TABLE gold_prices            ADD COLUMN IF NOT EXISTS high_price NUMERIC(15,2);
ALTER TABLE gold_prices            ADD COLUMN IF NOT EXISTS low_price  NUMERIC(15,2);
ALTER TABLE gold_intraday_prices   ADD COLUMN IF NOT EXISTS open_price NUMERIC(15,2);
ALTER TABLE gold_intraday_prices   ADD COLUMN IF NOT EXISTS high_price NUMERIC(15,2);
ALTER TABLE gold_intraday_prices   ADD COLUMN IF NOT EXISTS low_price  NUMERIC(15,2);
```

Các cột nullable → an toàn với hypertable composite PK (không đụng PK/unique index). NASDAQ/SP500 không thêm gì.

### 4.2. Python prediction-svc (chỉ CRYPTO + GOLD)

**ORM models** (`src/database/models.py`): `CryptoPrice`, `CryptoIntradayPrice`, `GoldPrice`, `GoldIntradayPrice` thêm `open_price`/`high_price`/`low_price`. NASDAQ/SP500 đã có.

**Repository** (`src/database/repository.py`):
- `upsert_crypto_price()` + intraday: thêm tham số OHLC.
- `upsert_gold_price()` + intraday: thêm tham số OHLC (nullable; nguồn VN truyền None).

**Crawler crypto** (`src/crawlers/crypto.py`):
- Daily/history: bổ sung gọi CoinGecko `/coins/{id}/ohlc?vs_currency=usd&days=N` để lấy O/H/L/C, ghi vào `crypto_prices`. `close_price` vẫn giữ (line + ML input).
- Intraday: lấy OHLC từ `/ohlc?days=1` (bucket ~30min) hoặc giữ market_chart cho close + bổ sung O/H/L từ `/ohlc`.
- **Giới hạn đã biết:** CoinGecko free bucket nến theo range (1d→30min, 7–30d→4h, 90d+→4day). Nến crypto theo bucket này, không khít daily/hourly tuyệt đối — chấp nhận được; ghi rõ trong code comment.

**Crawler gold** (`src/crawlers/gold.py`):
- `_crawl_xau()`, `_import_xau_history()`, intraday XAU: Yahoo response **đã có** open/high/low/close (hiện chỉ lấy close làm buy/sell). Lấy thêm O/H/L lưu vào cột mới. `sell_price`/`buy_price` giữ nguyên (close).
- Nguồn VN: không đổi, O/H/L để NULL.

### 4.3. Go api-svc

**GORM models** (`pkg/models/models_db/`): `CryptoPrice`, `CryptoIntradayPrice`, `GoldPrice`, `GoldIntradayPrice` thêm `OpenPrice`/`HighPrice`/`LowPrice` (`*decimal.Decimal`, nullable). NASDAQ/SP500 đã có.

**Chart handlers** (`api_crypto.go`, `api_nasdaq.go`, `api_sp500.go`, `api_gold.go`): mở rộng response struct thêm mảng song song:

```go
type cryptoChartResponse struct {
    CoinID      string            `json:"coin_id"`
    Dates       []string          `json:"dates"`
    Prices      []decimal.Decimal `json:"prices"`   // = close, giữ nguyên
    Opens       []decimal.Decimal `json:"opens"`
    Highs       []decimal.Decimal `json:"highs"`
    Lows        []decimal.Decimal `json:"lows"`
    Closes      []decimal.Decimal `json:"closes"`
    Granularity string            `json:"granularity"`
}
```

- NASDAQ/SP500/crypto/gold-XAU: điền O/H/L/C từ model.
- **Quy tắc NULL (thống nhất):** nếu nguồn không có OHLC (gold VN source, hoặc coin chưa backfill toàn bộ) → trả `opens/highs/lows/closes` = **mảng rỗng** → frontend ẩn nút Nến, dùng line. Nếu có OHLC nhưng lẻ tẻ vài điểm NULL → điểm đó điền O=H=L=close (nến doji) để giữ alignment với `labels`. Frontend bật nút Nến dựa trên cờ `hasOHLC = closes.length > 0`.
- Giữ logic đảo DESC→ASC hiện có cho tất cả mảng.

### 4.4. Frontend web-svc

**Component `Candlestick`** mới trong `charts.tsx`:
- Props: `{ data: { o: number; h: number; l: number; c: number }[]; labels: string[]; height?; yFmt?; valueFmt?; padL? }`.
- SVG: thân nến = rect (xanh `var(--up)` nếu c≥o, đỏ `var(--down)` nếu c<o), bấc = line min→max. Grid + trục Y + tooltip hover khớp `LineChart`. Label trục X theo `xStep` như `LineChart`.
- Xử lý rỗng giống `LineChart` ("Không có dữ liệu").

**Mỗi page** (`Crypto.tsx`, `Nasdaq.tsx`, `SP500.tsx`, `Gold.tsx`):
- `ChartData` mở rộng: thêm `opens/highs/lows/closes: number[]`.
- Map response các mảng OHLC mới.
- State `chartType: 'line' | 'candle'` (default `line`).
- Toggle `Seg` (Line / Nến) cạnh selector ngày trong Panel tools.
- Render: `chartType === 'candle' && hasOHLC` → `<Candlestick>`, ngược lại `<LineChart>`.
- `hasOHLC = chart.closes.length > 0`. Khi false → ẩn/disable nút Nến (đặc biệt gold nguồn VN).
- Gold dùng helper `goldChart()` trong `api/index.ts` — mở rộng helper trả thêm OHLC.

**i18n** (`i18n.ts`): thêm key cho cả VI/EN, vd:
```ts
chartType: { line: 'Đường', candle: 'Nến' }   // VI
chartType: { line: 'Line', candle: 'Candle' }  // EN
```
Đặt cạnh các key `priceChart` hiện có (crypto/gold/nasdaq/sp500) hoặc dùng chung 1 key `common.chartType`.

## 5. Backfill & vận hành

Sau deploy:
1. Rebuild stack (DB schema tự apply ALTER khi init; với DB đang chạy cần chạy ALTER thủ công hoặc qua init script).
2. Trigger `POST /api/trigger/crypto-history` và `POST /api/trigger/gold-history` để lấp OHLC lịch sử.
3. NASDAQ/SP500 có OHLC sẵn → nến hiển thị ngay.

## 6. Phạm vi KHÔNG làm (YAGNI)

- Không zoom/pan/crosshair kiểu TradingView (SVG tĩnh, tooltip hover như chart hiện tại).
- Không đổi thuật toán ML (vẫn dùng close series).
- Không thêm volume sub-panel.
- Không làm nến cho gold nguồn VN.
- Không đổi NASDAQ/SP500 ở tầng crawler/DB.

## 7. Kiểm thử

- **Python**: unit test crypto/gold crawler parse OHLC; `cd prediction-svc && make test`.
- **Go**: `cd api-svc && go test ./...` — test chart handler trả OHLC arrays đúng thứ tự ASC, gold VN source trả mảng rỗng.
- **Frontend**: build `web-svc` không lỗi TS; smoke test toggle Line/Nến trên 4 page.
- **E2E**: sau backfill, curl `/api/crypto/chart?coin=bitcoin&days=30` thấy `closes` non-empty; `/api/gold/chart?source=SJC` thấy `closes` rỗng.
- **Docs**: cập nhật `docs/claude/database.md` (cột OHLC mới), `api-endpoints.md` (response chart có OHLC).

## 8. Thứ tự triển khai đề xuất

1. DB schema (ALTER) + Go/Python models (4 market crypto/gold).
2. Python crawler crypto + gold lưu OHLC + repository.
3. Go chart handlers trả OHLC arrays (cả 4 market).
4. Frontend: Candlestick component → toggle 4 page → i18n.
5. Backfill + verify + docs.
