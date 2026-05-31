# Phase 1: Giá Xăng Việt Nam Backend

## Data Source

**Nguồn chính: giaxanghomnay.com (JSON API, free, không cần key)**

| Endpoint | URL | Mô tả |
|----------|-----|-------|
| Lịch sử đầy đủ | `GET https://giaxanghomnay.com/api/chart` | 467+ records từ 08/2018, JSON array |
| Giá theo ngày | `GET https://giaxanghomnay.com/api/pvdate/{YYYY-MM-DD}` | 2-3 kỳ quanh ngày request |

**Response `/api/chart`:**
```json
[
  {"date": "2018-08-22", "a": 20.87, "b": 19.72, "c": 16.94, "d": 14.55},
  {"date": "2026-05-28", "a": 24.15, "b": 23.02, "c": 20.41, "d": 19.28}
]
```

**Field mapping:**
| Field | Sản phẩm | Đơn vị |
|-------|----------|--------|
| `a` | RON 95-III | nghìn VND/lít (24.15 = 24,150 VND) |
| `b` | E5 RON 92-II | nghìn VND/lít |
| `c` | DO 0,05S-II (Diesel) | nghìn VND/lít |
| `d` | Dầu hỏa 2-K | nghìn VND/lít |

**Đặc thù giá xăng VN:**
- Điều chỉnh theo **chu kỳ ~7 ngày** (Nghị định 80/2023), có thể điều chỉnh ngoài chu kỳ khi biến động >7%
- Thời điểm có hiệu lực: 15:00 hoặc 0:00
- **Không có OHLCV** — mỗi kỳ chỉ có 1 mức giá duy nhất
- Dữ liệu thưa hơn stock (52 kỳ/năm thay vì 250 phiên/năm)

---

## Bước 1 — DB Model

Tạo `pkg/models/models_db/fuel_price.go`:

```go
type FuelPrice struct {
    ID          uint            `gorm:"primaryKey;autoIncrement"`
    ProductType string          `gorm:"type:varchar(30);not null;uniqueIndex:idx_fuel_product_date,priority:1"`
    // "ron95_iii", "e5_ron92", "do_005s", "kerosene"
    Price       decimal.Decimal `gorm:"type:decimal(10,3);not null"`  // nghìn VND/lít (24.150)
    TradingDate time.Time       `gorm:"type:date;not null;uniqueIndex:idx_fuel_product_date,priority:2"`
    CreatedAt   time.Time
    UpdatedAt   time.Time
    DeletedAt   gorm.DeletedAt  `gorm:"index"`
}
```

**Tại sao `decimal(10,3)`?** Giá xăng từ API là đơn vị nghìn VND (e.g. 24.150), lưu nguyên giá trị gốc. Frontend nhân 1000 khi hiển thị VND đầy đủ.

Tạo `pkg/models/models_db/fuel_prediction.go`:

```go
type FuelPrediction struct {
    ID             uint             `gorm:"primaryKey;autoIncrement"`
    ProductType    string           `gorm:"type:varchar(30);not null;index:idx_fuel_pred,priority:1"`
    AlgorithmName  string           `gorm:"type:varchar(100);not null;index:idx_fuel_pred,priority:2"`
    PredictedPrice decimal.Decimal  `gorm:"type:decimal(10,3);not null"`
    CurrentPrice   decimal.Decimal  `gorm:"type:decimal(10,3);not null"`
    Confidence     decimal.Decimal  `gorm:"type:decimal(5,4)"`
    PredictionDate time.Time        `gorm:"not null;index"`
    TargetDate     time.Time        `gorm:"not null;index"`
    ActualPrice    *decimal.Decimal `gorm:"type:decimal(10,3)"`
    Accuracy       *decimal.Decimal `gorm:"type:decimal(5,4)"`
    CreatedAt      time.Time
    UpdatedAt      time.Time
    DeletedAt      gorm.DeletedAt   `gorm:"index"`
}
```

Sửa `migrations.go` — thêm `&FuelPrice{}`, `&FuelPrediction{}` vào `AllModels`.

---

## Bước 2 — Repository

Tạo `pkg/store/repository/fuel.go`:

```go
type FuelPriceStore interface {
    CreateFuelPrice(p *modelsdb.FuelPrice) error
    UpsertFuelPrice(p *modelsdb.FuelPrice) error
    BulkUpsertFuelPrices(prices []modelsdb.FuelPrice) error
    GetFuelPricesByDateRange(productType string, from, to time.Time) ([]modelsdb.FuelPrice, error)
    GetLatestFuelPrice(productType string) (*modelsdb.FuelPrice, error)
    GetFuelProducts() ([]string, error)  // distinct product types
}

type FuelPredictionStore interface {
    CreateFuelPrediction(p *modelsdb.FuelPrediction) error
    GetFuelPredictions(productType, algorithm string, limit int) ([]modelsdb.FuelPrediction, error)
    GetLatestFuelPredictions() ([]modelsdb.FuelPrediction, error)
    GetFuelPredictionsByDateRange(productType string, from, to time.Time) ([]modelsdb.FuelPrediction, error)
    GetFuelPredictionsPage(page, limit int, search, algorithm, status, sortBy, sortDir string) ([]modelsdb.FuelPrediction, int64, error)
}
```

Thêm vào `DatabaseStore`: embed `FuelPriceStore` + `FuelPredictionStore`.

Tạo `pkg/store/mysql/fuel.go` — implement theo pattern `gold.go`.

---

## Bước 3 — Crawler

Tạo `pkg/service/crawler/fuel_crawler.go`:

**Mapping sản phẩm:**
```go
var fuelProducts = map[string]string{
    "a": "ron95_iii",   // RON 95-III
    "b": "e5_ron92",    // E5 RON 92-II
    "c": "do_005s",     // DO 0,05S-II (Diesel)
    "d": "kerosene",    // Dầu hỏa 2-K
}
```

**Functions:**

1. `CronjobFuelCrawler() error`
   - Gọi `GET https://giaxanghomnay.com/api/pvdate/{today}`
   - Parse JSON → upsert vào DB cho 4 sản phẩm
   - Nếu không có data cho hôm nay (chưa đến kỳ điều chỉnh) → skip, log info

2. `ImportFuelHistory() error`
   - Gọi `GET https://giaxanghomnay.com/api/chart` (1 request, trả toàn bộ lịch sử)
   - Parse 467+ records → bulk upsert vào DB
   - Chỉ chạy lần đầu khi DB trống

3. `fetchFuelChart() ([]FuelChartEntry, error)` — internal helper
   ```go
   type FuelChartEntry struct {
       Date string  `json:"date"`
       A    float64 `json:"a"` // RON 95-III
       B    float64 `json:"b"` // E5 RON 92
       C    float64 `json:"c"` // DO 0,05S
       D    float64 `json:"d"` // Kerosene
   }
   ```

**Đăng ký cron** trong `crawler/init.go`:
- Thêm vào `crawlerDefaults`: `{JobKey: "crawler_fuel", JobName: "Crawler Fuel (Giá xăng VN)", CronExpression: "0 0 20 * * *", Enabled: true}`
  - 20:00 hàng ngày — sau 15:00 (giờ điều chỉnh) để chắc chắn có data mới
- Thêm case vào `getCrawlerJobFn()`: `"crawler_fuel" → CronjobFuelCrawler`

**Backfill on startup** trong `crawler/Init()`:
```go
go func() {
    logger.Logger.Info("Starting fuel history backfill in background")
    ImportFuelHistory()
}()
```

---

## Bước 4 — AssetMarket

Tạo `pkg/service/market/fuel/market.go`:

```go
package fuelmarket

type Market struct{}

var fuelInstruments = []market.Instrument{
    {ID: "ron95_iii", Name: "Xăng RON 95-III"},
    {ID: "e5_ron92",  Name: "Xăng E5 RON 92-II"},
    {ID: "do_005s",   Name: "Dầu DO 0,05S-II"},
    {ID: "kerosene",  Name: "Dầu hỏa 2-K"},
}

func (m *Market) MarketKey() string  { return "FUEL" }
func (m *Market) MarketName() string { return "Giá Xăng Dầu Việt Nam" }

func (m *Market) GetInstruments(ctx context.Context) ([]market.Instrument, error) {
    return fuelInstruments, nil
}

func (m *Market) FetchPrices(ctx context.Context, inst market.Instrument, months int) ([]float64, error) {
    from := time.Now().AddDate(0, -months, 0)
    prices, err := repository.GetSingleton().GetFuelPricesByDateRange(inst.ID, from, time.Now())
    // reverse DESC → ASC, return Price (nghìn VND) as []float64
}

func (m *Market) SavePrediction(ctx context.Context, inst market.Instrument, pred market.PredictionData) error {
    // CreateFuelPrediction
}

func (m *Market) Crawl(ctx context.Context) error {
    return crawler.CronjobFuelCrawler()
}

func init() {
    market.Register(&Market{})
}
```

**Lưu ý quan trọng — Minimum data points:**
- Orchestrator bỏ qua instrument có < 20 data points
- Giá xăng ~52 kỳ/năm → 9 tháng history ≈ 39 kỳ → đủ 20 points
- `FetchPrices(ctx, inst, 9)` trong orchestrator sẽ lấy 9 tháng → OK

---

## Bước 5 — Blank import

Sửa `cmd/prediction/main.go`:

```go
_ "go-stock-prediction/pkg/service/market/fuel" // registers FUEL market
```

---

## Lưu ý đặc thù

1. **Dữ liệu thưa:** Giá xăng chỉ thay đổi ~52 lần/năm (vs 250 phiên stock). Thuật toán vẫn hoạt động nhưng accuracy có thể thấp hơn do ít data points.

2. **Interpolation:** Giữa 2 kỳ điều chỉnh, giá không đổi. DB chỉ lưu các ngày có thay đổi giá (theo `/api/chart`), không lưu ngày trùng giá.

3. **Fallback source:** Nếu giaxanghomnay.com ngừng hoạt động, có thể fallback sang scrape `webgia.com/gia-xang-dau/petrolimex/` (HTML table, cần Gocolly).

---

## Checklist

- [ ] `pkg/models/models_db/fuel_price.go` — FuelPrice + FuelPrediction structs
- [ ] `pkg/models/models_db/migrations.go` — thêm vào AllModels
- [ ] `pkg/store/repository/fuel.go` — interfaces
- [ ] `pkg/store/repository/repository.go` — embed interfaces mới
- [ ] `pkg/store/mysql/fuel.go` — GORM implementations
- [ ] `pkg/service/crawler/fuel_crawler.go` — giaxanghomnay.com crawler
- [ ] `pkg/service/crawler/init.go` — đăng ký cron + backfill
- [ ] `pkg/service/market/fuel/market.go` — AssetMarket implementation
- [ ] `cmd/prediction/main.go` — blank import
- [ ] Test: `go build ./...` pass
