# Phase 1: Bitcoin/Crypto Backend

## Bước 1 — DB Model

Tạo `pkg/models/models_db/crypto_price.go`:

```go
type CryptoPrice struct {
    ID          uint            `gorm:"primaryKey;autoIncrement"`
    CoinID      string          `gorm:"type:varchar(50);not null;uniqueIndex:idx_crypto_coin_date,priority:1"` // coingecko id: "bitcoin", "ethereum"
    Symbol      string          `gorm:"type:varchar(10);not null"`  // BTC, ETH
    ClosePrice  decimal.Decimal `gorm:"type:decimal(20,2);not null"`
    MarketCap   decimal.Decimal `gorm:"type:decimal(30,2)"`
    Volume24h   decimal.Decimal `gorm:"type:decimal(30,2)"`
    TradingDate time.Time       `gorm:"type:date;not null;uniqueIndex:idx_crypto_coin_date,priority:2"`
    Currency    string          `gorm:"type:varchar(3);not null;default:'USD'"`
    CreatedAt   time.Time
    UpdatedAt   time.Time
    DeletedAt   gorm.DeletedAt  `gorm:"index"`
}
```

**Tại sao dùng `CoinID` thay vì chỉ `Symbol`?**
- CoinGecko API dùng `id` (e.g. "bitcoin") thay vì symbol ("BTC")
- Tránh conflict: nhiều coin có cùng symbol nhưng khác chain
- `Symbol` giữ lại để display (BTC, ETH)

Tạo `pkg/models/models_db/crypto_prediction.go`:

```go
type CryptoPrediction struct {
    ID             uint             `gorm:"primaryKey;autoIncrement"`
    CoinID         string           `gorm:"type:varchar(50);not null;index:idx_crypto_pred,priority:1"`
    Symbol         string           `gorm:"type:varchar(10);not null"`
    AlgorithmName  string           `gorm:"type:varchar(100);not null;index:idx_crypto_pred,priority:2"`
    PredictedPrice decimal.Decimal  `gorm:"type:decimal(20,2);not null"`
    CurrentPrice   decimal.Decimal  `gorm:"type:decimal(20,2);not null"`
    Confidence     decimal.Decimal  `gorm:"type:decimal(5,4)"`
    PredictionDate time.Time        `gorm:"not null;index"`
    TargetDate     time.Time        `gorm:"not null;index"`
    ActualPrice    *decimal.Decimal `gorm:"type:decimal(20,2)"`
    Accuracy       *decimal.Decimal `gorm:"type:decimal(5,4)"`
    CreatedAt      time.Time
    UpdatedAt      time.Time
    DeletedAt      gorm.DeletedAt   `gorm:"index"`
}
```

Sửa `migrations.go` — thêm `&CryptoPrice{}`, `&CryptoPrediction{}` vào `AllModels`.

---

## Bước 2 — Repository

Tạo `pkg/store/repository/crypto.go`:

```go
type CryptoPriceStore interface {
    CreateCryptoPrice(p *modelsdb.CryptoPrice) error
    UpsertCryptoPrice(p *modelsdb.CryptoPrice) error
    GetCryptoPricesByDateRange(coinID string, from, to time.Time) ([]modelsdb.CryptoPrice, error)
    GetLatestCryptoPrice(coinID string) (*modelsdb.CryptoPrice, error)
    GetCryptoCoins() ([]modelsdb.CryptoPrice, error)  // distinct coins (latest record each)
}

type CryptoPredictionStore interface {
    CreateCryptoPrediction(p *modelsdb.CryptoPrediction) error
    GetCryptoPredictions(coinID, algorithm string, limit int) ([]modelsdb.CryptoPrediction, error)
    GetLatestCryptoPredictions() ([]modelsdb.CryptoPrediction, error)
    GetCryptoPredictionsByDateRange(coinID string, from, to time.Time) ([]modelsdb.CryptoPrediction, error)
    GetCryptoPredictionsPage(page, limit int, search, algorithm, status, sortBy, sortDir string) ([]modelsdb.CryptoPrediction, int64, error)
}
```

Thêm vào `DatabaseStore` interface: embed `CryptoPriceStore` + `CryptoPredictionStore`.

Tạo `pkg/store/mysql/crypto.go` — implement cả 2 interfaces. Follow pattern `pkg/store/mysql/gold.go`.

---

## Bước 3 — Crawler

Tạo `pkg/service/crawler/crypto_crawler.go`:

```go
var cryptoCoins = []struct {
    CoinID string
    Symbol string
}{
    {"bitcoin", "BTC"},
    {"ethereum", "ETH"},
}
```

**Data source: CoinGecko API (free, no API key)**

**Endpoints sử dụng:**
1. **Daily price:** `GET /api/v3/simple/price?ids=bitcoin,ethereum&vs_currencies=usd&include_market_cap=true&include_24hr_vol=true`
   - Trả về giá, market cap, volume 24h cho nhiều coins cùng lúc
   - 1 request cho tất cả coins → không cần rate limiting

2. **History backfill:** `GET /api/v3/coins/{id}/market_chart?vs_currency=usd&days=180&interval=daily`
   - Trả về array `[timestamp, price]` cho mỗi ngày
   - Cần 1 request per coin → sleep 2s giữa mỗi request (free tier: 10-30 req/min)

**Functions:**
- `CronjobCryptoCrawler() error` — crawl current price cho tất cả coins (1 API call)
- `ImportCryptoHistory() error` — backfill 180 ngày cho mỗi coin
- `fetchCoinGeckoPrice(coinIDs []string) (map[string]CoinPrice, error)` — batch fetch
- `fetchCoinGeckoHistory(coinID string, days int) ([]CryptoPrice, error)` — single coin history

**Đăng ký cron** trong `crawler/init.go`:
- Thêm vào `crawlerDefaults`: `{JobKey: "crawler_crypto", JobName: "Crawler Crypto (BTC/ETH)", CronExpression: "0 0 */4 * * *", Enabled: true}`
- Thêm case vào `getCrawlerJobFn()`: `"crawler_crypto" → CronjobCryptoCrawler`

**Backfill on startup** trong `crawler/Init()`:
```go
go func() {
    logger.Logger.Info("Starting crypto history backfill in background")
    ImportCryptoHistory()
}()
```

---

## Bước 4 — AssetMarket

Tạo `pkg/service/market/crypto/market.go`:

```go
package cryptomarket

type Market struct{}

func (m *Market) MarketKey() string  { return "CRYPTO" }
func (m *Market) MarketName() string { return "Cryptocurrency — BTC & ETH" }

func (m *Market) GetInstruments(ctx context.Context) ([]market.Instrument, error) {
    return []market.Instrument{
        {ID: "bitcoin", Name: "Bitcoin (BTC)"},
        {ID: "ethereum", Name: "Ethereum (ETH)"},
    }, nil
}

func (m *Market) FetchPrices(ctx context.Context, inst market.Instrument, months int) ([]float64, error) {
    from := time.Now().AddDate(0, -months, 0)
    prices, err := repository.GetSingleton().GetCryptoPricesByDateRange(inst.ID, from, time.Now())
    // reverse DESC → ASC, return ClosePrice as []float64
}

func (m *Market) SavePrediction(ctx context.Context, inst market.Instrument, pred market.PredictionData) error {
    // Lookup symbol from coinID, CreateCryptoPrediction
}

func (m *Market) Crawl(ctx context.Context) error {
    return crawler.CronjobCryptoCrawler()
}

func init() {
    market.Register(&Market{})
}
```

**Lưu ý:** Instrument ID = CoinGecko `id` ("bitcoin", "ethereum") — khớp với `CoinID` trong DB. Không dùng "BTC/BINANCE" format như trong `ADDING_NEW_MARKET.md` vì ta dùng CoinGecko thay vì Binance.

---

## Bước 5 — Blank import

Sửa `cmd/prediction/main.go`:

```go
_ "go-stock-prediction/pkg/service/market/crypto" // registers CRYPTO market
```

---

## Checklist

- [ ] `pkg/models/models_db/crypto_price.go` — CryptoPrice + CryptoPrediction structs
- [ ] `pkg/models/models_db/migrations.go` — thêm vào AllModels
- [ ] `pkg/store/repository/crypto.go` — interfaces
- [ ] `pkg/store/repository/repository.go` — embed interfaces mới
- [ ] `pkg/store/mysql/crypto.go` — GORM implementations
- [ ] `pkg/service/crawler/crypto_crawler.go` — CoinGecko crawler
- [ ] `pkg/service/crawler/init.go` — đăng ký cron + backfill
- [ ] `pkg/service/market/crypto/market.go` — AssetMarket implementation
- [ ] `cmd/prediction/main.go` — blank import
- [ ] Test: `go build ./...` pass
