# Phase 1: Critical Bug Fixes

Sửa các lỗi khiến app crash hoặc cho kết quả sai hoàn toàn.
Không có phase này, app không chạy đúng.

---

## 1.1 — Sửa SQL dialect sai (MySQL vs PostgreSQL)

**File:** `pkg/store/mysql/stock_prices.go`

**Vấn đề:**
`DISTINCT ON (stock_id)` là cú pháp PostgreSQL, không tồn tại trong MySQL.
Gọi `GetLatestStockPricesForVN30()` sẽ trả về lỗi SQL ngay lập tức.
Đây là hàm được gọi trong dashboard và market overview.

**Hiện tại:**
```go
func (c *Client) GetLatestStockPricesForVN30(ctx context.Context) ([]modelsdb.StockPrice, error) {
    var stockPrices []modelsdb.StockPrice
    err := c.db.WithContext(ctx).
        Joins("JOIN stocks ON stocks.id = stock_prices.stock_id").
        Where("stocks.is_vn30 = ?", true).
        Select("DISTINCT ON (stock_id) stock_prices.*").
        Order("stock_id, trade_date DESC").
        Find(&stockPrices).Error
    ...
}
```

**Cách sửa:**
Dùng subquery để lấy latest price mỗi stock:
```go
func (c *Client) GetLatestStockPricesForVN30(ctx context.Context) ([]modelsdb.StockPrice, error) {
    var stockPrices []modelsdb.StockPrice
    subQuery := c.db.WithContext(ctx).
        Model(&modelsdb.StockPrice{}).
        Select("stock_id, MAX(trade_date) as max_date").
        Group("stock_id")

    err := c.db.WithContext(ctx).
        Joins("JOIN stocks ON stocks.id = stock_prices.stock_id").
        Joins("JOIN (?) as latest ON latest.stock_id = stock_prices.stock_id AND latest.max_date = stock_prices.trade_date", subQuery).
        Where("stocks.is_vn30 = ? AND stocks.deleted_at IS NULL", true).
        Find(&stockPrices).Error

    return stockPrices, err
}
```

**Test:** Gọi `GET /api/market/overview` sau khi sửa, phải trả về 200 với data thực.

---

## 1.2 — Sửa database name sai trong config default

**File:** `pkg/config/init.go`

**Vấn đề:**
Default DB name là `"weather_alerts_db"` — copy-paste từ project khác.
Nếu `.env` không set `MYSQL_DB_NAME`, app kết nối sai DB.

**Hiện tại:**
```go
DbName: getEnv("MYSQL_DB_NAME", "weather_alerts_db"),
```

**Cách sửa:**
```go
DbName: getEnv("MYSQL_DB_NAME", "go_stock_prediction"),
```

---

## 1.3 — Sửa trigger endpoints dùng GET thay vì POST

**File:** `pkg/server/router.go`

**Vấn đề:**
Các endpoint trigger side-effects đang dùng `GET`:
```go
mux.HandleFunc("GET /api/trigger/crawler", ...)
mux.HandleFunc("GET /api/trigger/predict", ...)
```

`GET` request không được phép có side effects (chuẩn HTTP/REST).
Browsers, proxies, crawlers có thể tự động gọi GET — gây trigger không mong muốn.

**Cách sửa:**
```go
mux.HandleFunc("POST /api/trigger/crawler", ...)
mux.HandleFunc("POST /api/trigger/predict", ...)
```

Cũng cập nhật CLAUDE.md section "Trigger thủ công" để dùng `-X POST`.

---

## 1.4 — Sửa race condition trong prediction service

**File:** `pkg/service/predict/init.go`

**Vấn đề:**
`CronjobDailyPrediction()` và `CronjobWeeklyTraining()` có thể chạy đồng thời
(manual trigger qua API + cron job). Không có mutex bảo vệ state `isTraining`.

**Hiện tại:**
```go
var isTraining bool  // global, không có mutex

func (ps *PredictionService) CronjobWeeklyTraining(ctx context.Context) {
    isTraining = true  // data race
    defer func() { isTraining = false }()
    ...
}
```

**Cách sửa:**
```go
type PredictionService struct {
    ...
    mu         sync.Mutex
    isTraining bool
    lastTrained time.Time
}

func (ps *PredictionService) CronjobWeeklyTraining(ctx context.Context) {
    ps.mu.Lock()
    if ps.isTraining {
        ps.mu.Unlock()
        logger.Logger.Warn().Msg("Training already in progress, skipping")
        return
    }
    ps.isTraining = true
    ps.mu.Unlock()
    defer func() {
        ps.mu.Lock()
        ps.isTraining = false
        ps.mu.Unlock()
    }()
    ...
}
```

---

## 1.5 — Sửa timezone không ổn định

**File:** `pkg/config/init.go`

**Vấn đề:**
DSN dùng `loc=Local` — timezone phụ thuộc vào OS của server.
Nếu deploy lên server UTC, toàn bộ time comparison sai.
Vietnam market dùng Asia/Ho_Chi_Minh (UTC+7).

**Hiện tại:**
```go
dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true&loc=Local",
    ...
)
```

**Cách sửa:**
```go
dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true&loc=Asia%%2FHo_Chi_Minh",
    ...
)
```

Đồng thời trong `main.go` thêm:
```go
loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
if err != nil {
    logger.Logger.Fatal().Err(err).Msg("Failed to load timezone")
}
time.Local = loc
```

---

## 1.6 — Sửa random number generator không đúng cách

**File:** `pkg/service/predict/moving_average/algo.go`

**Vấn đề:**
```go
func (m *MovingAveragePredictor) random() float64 {
    return float64(time.Now().UnixNano()%1000) / 1000.0
}
```
- Gọi liên tiếp trong vòng lặp sẽ trả về cùng giá trị (nanosecond không thay đổi kịp)
- Distribution không đều — modulo bias với số lớn

**Cách sửa:**
```go
import "math/rand"

// Trong struct, thêm:
type MovingAveragePredictor struct {
    ...
    rng *rand.Rand
}

// Trong constructor:
func NewMovingAveragePredictor() *MovingAveragePredictor {
    return &MovingAveragePredictor{
        ...
        rng: rand.New(rand.NewSource(time.Now().UnixNano())),
    }
}

// Thay random():
func (m *MovingAveragePredictor) random() float64 {
    return m.rng.Float64()
}
```

---

## Checklist Phase 1

- [ ] 1.1 Sửa `DISTINCT ON` → subquery trong `pkg/store/mysql/stock_prices.go`
- [ ] 1.2 Sửa default DB name trong `pkg/config/init.go`
- [ ] 1.3 Đổi trigger endpoints từ GET → POST trong `pkg/server/router.go`
- [ ] 1.4 Thêm mutex bảo vệ `isTraining` trong `pkg/service/predict/init.go`
- [ ] 1.5 Fix timezone → `Asia/Ho_Chi_Minh` trong config DSN và main.go
- [ ] 1.6 Sửa `random()` dùng `math/rand` proper trong moving_average algo

**Thời gian ước tính:** 2–3 giờ
**Rủi ro nếu bỏ qua:** App crash khi load dashboard, race condition khi trigger training
