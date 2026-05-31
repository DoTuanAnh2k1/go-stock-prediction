# Phase 1: NASDAQ 100 Backend

## Bước 1 — DB Model

Tạo `pkg/models/models_db/nasdaq_price.go`:

```go
type NasdaqPrice struct {
    ID          uint            `gorm:"primaryKey;autoIncrement"`
    Symbol      string          `gorm:"type:varchar(10);not null;uniqueIndex:idx_nasdaq_symbol_date,priority:1"`
    CompanyName string          `gorm:"type:varchar(200)"`
    OpenPrice   decimal.Decimal `gorm:"type:decimal(15,4)"`
    HighPrice   decimal.Decimal `gorm:"type:decimal(15,4)"`
    LowPrice    decimal.Decimal `gorm:"type:decimal(15,4)"`
    ClosePrice  decimal.Decimal `gorm:"type:decimal(15,4);not null"`
    Volume      int64
    TradingDate time.Time       `gorm:"type:date;not null;uniqueIndex:idx_nasdaq_symbol_date,priority:2"`
    Currency    string          `gorm:"type:varchar(3);not null;default:'USD'"`
    CreatedAt   time.Time
    UpdatedAt   time.Time
    DeletedAt   gorm.DeletedAt  `gorm:"index"`
}
```

**Tại sao có OHLCV?** Thuật toán VWMA cần volume, LSTM cần nhiều features. VN30 đã có OHLCV trong StockPrice — NASDAQ nên tương đương.

**Không dùng chung bảng `stock_prices`** vì:
- `stock_prices` dùng `stock_id` FK → bảng `stocks` có `is_vn30`, `exchange_id` — schema khác biệt
- NASDAQ price đơn vị USD (decimal 15,4), VN30 đơn vị VND (decimal 15,2)
- Tách bảng giúp query performance tốt hơn và migration dễ hơn

Prediction dùng chung pattern với Gold — tạo `NasdaqPrediction` struct tương tự `GoldPrediction`:

```go
type NasdaqPrediction struct {
    ID             uint             `gorm:"primaryKey;autoIncrement"`
    Symbol         string           `gorm:"type:varchar(10);not null;index:idx_nasdaq_pred,priority:1"`
    AlgorithmName  string           `gorm:"type:varchar(100);not null;index:idx_nasdaq_pred,priority:2"`
    PredictedPrice decimal.Decimal  `gorm:"type:decimal(15,4);not null"`
    CurrentPrice   decimal.Decimal  `gorm:"type:decimal(15,4);not null"`
    Confidence     decimal.Decimal  `gorm:"type:decimal(5,4)"`
    PredictionDate time.Time        `gorm:"not null;index"`
    TargetDate     time.Time        `gorm:"not null;index"`
    ActualPrice    *decimal.Decimal `gorm:"type:decimal(15,4)"`
    Accuracy       *decimal.Decimal `gorm:"type:decimal(5,4)"`
    CreatedAt      time.Time
    UpdatedAt      time.Time
    DeletedAt      gorm.DeletedAt   `gorm:"index"`
}
```

Sửa `migrations.go` — thêm `&NasdaqPrice{}`, `&NasdaqPrediction{}` vào `AllModels`.

---

## Bước 2 — Repository

Tạo `pkg/store/repository/nasdaq.go`:

```go
type NasdaqPriceStore interface {
    CreateNasdaqPrice(p *modelsdb.NasdaqPrice) error
    UpsertNasdaqPrice(p *modelsdb.NasdaqPrice) error
    GetNasdaqPricesByDateRange(symbol string, from, to time.Time) ([]modelsdb.NasdaqPrice, error)
    GetLatestNasdaqPrice(symbol string) (*modelsdb.NasdaqPrice, error)
    GetNasdaqSymbols() ([]string, error)  // distinct symbols in DB
}

type NasdaqPredictionStore interface {
    CreateNasdaqPrediction(p *modelsdb.NasdaqPrediction) error
    GetNasdaqPredictions(symbol, algorithm string, limit int) ([]modelsdb.NasdaqPrediction, error)
    GetLatestNasdaqPredictions() ([]modelsdb.NasdaqPrediction, error)
    GetNasdaqPredictionsByDateRange(symbol string, from, to time.Time) ([]modelsdb.NasdaqPrediction, error)
    GetNasdaqPredictionsPage(page, limit int, search, algorithm, status, sortBy, sortDir string) ([]modelsdb.NasdaqPrediction, int64, error)
}
```

Thêm vào `DatabaseStore` interface: embed `NasdaqPriceStore` + `NasdaqPredictionStore`.

Tạo `pkg/store/mysql/nasdaq.go` — implement cả 2 interfaces. Follow pattern của `gold.go`.

---

## Bước 3 — Crawler

Tạo `pkg/service/crawler/nasdaq_crawler.go`:

```go
// Danh sách NASDAQ 100 cổ phiếu theo dõi ban đầu
var nasdaqSymbols = []string{
    "AAPL", "MSFT", "GOOGL", "AMZN", "NVDA",
    "META", "TSLA", "AVGO", "COST", "NFLX",
    "AMD", "ADBE", "QCOM", "INTC", "CSCO",
}
```

**Data source: Yahoo Finance API**
- Dùng lại logic fetch từ `gold_crawler.go` (`yahooHistoricalResponse` struct)
- URL: `https://query1.finance.yahoo.com/v8/finance/chart/{SYMBOL}?interval=1d&range=1d`
- History: `https://query1.finance.yahoo.com/v8/finance/chart/{SYMBOL}?interval=1d&range=6mo`
- Parse `chart.result[0].indicators.quote[0]` → OHLCV
- **Rate limiting:** Sleep 500ms giữa mỗi symbol để tránh bị chặn

**Functions:**
- `CronjobNasdaqCrawler() error` — crawl daily price cho tất cả symbols
- `ImportNasdaqHistory() error` — backfill 6 tháng cho symbols chưa có data
- `fetchYahooOHLCV(symbol string) ([]NasdaqPrice, error)` — internal helper

**Đăng ký cron** trong `crawler/init.go`:
- Thêm vào `crawlerDefaults`: `{JobKey: "crawler_nasdaq", JobName: "Crawler NASDAQ 100", CronExpression: "0 30 22 * * 1-5", Enabled: true}`
- Thêm case vào `getCrawlerJobFn()`: `"crawler_nasdaq" → CronjobNasdaqCrawler`

**Backfill on startup** trong `crawler/Init()`:
```go
go func() {
    logger.Logger.Info("Starting NASDAQ history backfill in background")
    ImportNasdaqHistory()
}()
```

---

## Bước 4 — AssetMarket

Tạo `pkg/service/market/nasdaq100/market.go`:

```go
package nasdaq100

type Market struct{}

func (m *Market) MarketKey() string  { return "NASDAQ100" }
func (m *Market) MarketName() string { return "NASDAQ 100 — Top US Tech Stocks" }

func (m *Market) GetInstruments(ctx context.Context) ([]market.Instrument, error) {
    // Lấy danh sách distinct symbols từ DB (đã crawl)
    symbols, err := repository.GetSingleton().GetNasdaqSymbols()
    // Map thành []market.Instrument{ID: symbol, Name: symbol}
}

func (m *Market) FetchPrices(ctx context.Context, inst market.Instrument, months int) ([]float64, error) {
    // GetNasdaqPricesByDateRange → reverse DESC → ASC → return ClosePrice as []float64
}

func (m *Market) SavePrediction(ctx context.Context, inst market.Instrument, pred market.PredictionData) error {
    // CreateNasdaqPrediction
}

func (m *Market) Crawl(ctx context.Context) error {
    return crawler.CronjobNasdaqCrawler()
}

func init() {
    market.Register(&Market{})
}
```

---

## Bước 5 — Blank import

Sửa `cmd/prediction/main.go`:

```go
_ "go-stock-prediction/pkg/service/market/nasdaq100" // registers NASDAQ100 market
```

---

## Checklist

- [ ] `pkg/models/models_db/nasdaq_price.go` — NasdaqPrice + NasdaqPrediction structs
- [ ] `pkg/models/models_db/migrations.go` — thêm vào AllModels
- [ ] `pkg/store/repository/nasdaq.go` — interfaces
- [ ] `pkg/store/repository/repository.go` — embed interfaces mới
- [ ] `pkg/store/mysql/nasdaq.go` — GORM implementations
- [ ] `pkg/service/crawler/nasdaq_crawler.go` — Yahoo Finance crawler
- [ ] `pkg/service/crawler/init.go` — đăng ký cron + backfill
- [ ] `pkg/service/market/nasdaq100/market.go` — AssetMarket implementation
- [ ] `cmd/prediction/main.go` — blank import
- [ ] Test: `go build ./...` pass
