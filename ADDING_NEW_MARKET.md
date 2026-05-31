# Hướng dẫn thêm thị trường mới

## Tổng quan kiến trúc

Hệ thống dùng pattern plugin để mở rộng thị trường mà không cần sửa orchestrator hay cron job:

```
AssetMarket interface (pkg/service/market/iface.go)
    ↓
market.Register() — gọi từ init() của mỗi market package
    ↓
market.All() — trả về tất cả markets đã đăng ký
    ↓
orchestrator.RunAllMarkets(ctx) — loop qua All(), chạy prediction cho từng market
    ↓
Daily6PM cron → CronjobDailyPrediction() → RunAllMarkets() — tự động chạy tất cả markets
```

Thêm market mới chỉ cần: implement `AssetMarket`, gọi `market.Register()` trong `init()`, và thêm blank import trong `cmd/prediction/main.go`. Orchestrator tự động picks up market mới ở lần khởi động tiếp theo.

**Markets hiện có:**
- `VN30` — `pkg/service/market/vn30/market.go` — 30 cổ phiếu vốn hóa lớn nhất HOSE
- `GOLD` — `pkg/service/market/gold/market.go` — 3 sản phẩm: XAU/spot, BTMC/sjc, BTMC/nhan_tron

---

## Ví dụ: Thêm thị trường Crypto (BTC, ETH, ...)

### Bước 1: DB Model + Migration

Tạo GORM struct cho bảng giá và bảng dự đoán:

```go
// pkg/models/models_db/crypto_price.go
package modelsdb

import (
    "time"
    "github.com/shopspring/decimal"
)

type CryptoPrice struct {
    ID          uint            `gorm:"primaryKey;autoIncrement"`
    Symbol      string          `gorm:"type:varchar(20);not null;index:idx_crypto_price,priority:1"`
    Exchange    string          `gorm:"type:varchar(50);not null"`
    ClosePrice  decimal.Decimal `gorm:"type:decimal(20,8);not null"`
    Volume      decimal.Decimal `gorm:"type:decimal(30,8)"`
    TradingDate time.Time       `gorm:"type:date;not null;index:idx_crypto_price,priority:2"`
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

```go
// pkg/models/models_db/crypto_prediction.go
package modelsdb

import (
    "time"
    "github.com/shopspring/decimal"
)

type CryptoPrediction struct {
    ID             uint            `gorm:"primaryKey;autoIncrement"`
    Symbol         string          `gorm:"type:varchar(20);not null;index"`
    Exchange       string          `gorm:"type:varchar(50);not null"`
    AlgorithmName  string          `gorm:"type:varchar(100);not null"`
    PredictedPrice decimal.Decimal `gorm:"type:decimal(20,8);not null"`
    CurrentPrice   decimal.Decimal `gorm:"type:decimal(20,8);not null"`
    Confidence     decimal.Decimal `gorm:"type:decimal(5,4)"`
    PredictionDate time.Time       `gorm:"type:datetime;not null"`
    TargetDate     time.Time       `gorm:"type:date;not null"`
    CreatedAt      time.Time
    UpdatedAt      time.Time
}
```

Đăng ký auto-migrate trong `pkg/models/models_db/migrations.go`:

```go
// Thêm vào danh sách AutoMigrate
db.AutoMigrate(
    // ... các model hiện có ...
    &CryptoPrice{},
    &CryptoPrediction{},
)
```

### Bước 2: Repository Interface + Implementation

Thêm vào `pkg/store/repository/repository.go`:

```go
// Trong interface DatabaseStore
GetCryptoPricesByDateRange(symbol, exchange string, from, to time.Time) ([]modelsdb.CryptoPrice, error)
CreateCryptoPrediction(p *modelsdb.CryptoPrediction) error
```

Tạo implementation trong `pkg/store/mysql/crypto.go`:

```go
package mysql

import (
    "time"
    modelsdb "go-stock-prediction/pkg/models/models_db"
)

func (s *store) GetCryptoPricesByDateRange(symbol, exchange string, from, to time.Time) ([]modelsdb.CryptoPrice, error) {
    var prices []modelsdb.CryptoPrice
    err := s.db.Where("symbol = ? AND exchange = ? AND trading_date BETWEEN ? AND ?",
        symbol, exchange, from, to).
        Order("trading_date DESC").
        Find(&prices).Error
    return prices, err
}

func (s *store) CreateCryptoPrediction(p *modelsdb.CryptoPrediction) error {
    return s.db.Create(p).Error
}
```

### Bước 3: Crawler

Tạo `pkg/service/crawler/crypto_crawler.go`:

```go
package crawler

import (
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    modelsdb "go-stock-prediction/pkg/models/models_db"
    "go-stock-prediction/pkg/logger"
    "go-stock-prediction/pkg/store/repository"
)

// CronjobCryptoCrawler fetches latest crypto prices from Binance public API.
func CronjobCryptoCrawler() error {
    logger.Logger.Info("Starting crypto crawler...")
    symbols := []string{"BTCUSDT", "ETHUSDT"}

    for _, sym := range symbols {
        price, err := fetchBinancePrice(sym)
        if err != nil {
            logger.Logger.Errorf("Failed to fetch %s: %v", sym, err)
            continue
        }
        if err := repository.GetSingleton().CreateCryptoPrice(price); err != nil {
            logger.Logger.Errorf("Failed to save %s: %v", sym, err)
        }
    }
    return nil
}

func fetchBinancePrice(symbol string) (*modelsdb.CryptoPrice, error) {
    url := fmt.Sprintf("https://api.binance.com/api/v3/ticker/price?symbol=%s", symbol)
    resp, err := http.Get(url)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var data struct {
        Price string `json:"price"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
        return nil, err
    }
    // ... parse và trả về CryptoPrice
    _ = data
    return nil, fmt.Errorf("not implemented")
}
```

Đăng ký cron job trong `pkg/service/crawler/init.go`:

```go
err = cron.AddJob("Crypto Crawler", cron.Daily12PM, CronjobCryptoCrawler)
```

### Bước 4: Implement AssetMarket

Tạo `pkg/service/market/crypto/market.go`:

```go
// Package cryptomarket registers the crypto market as an AssetMarket.
package cryptomarket

import (
    "context"
    "strings"
    "time"

    "github.com/shopspring/decimal"
    modelsdb "go-stock-prediction/pkg/models/models_db"
    "go-stock-prediction/pkg/service/crawler"
    "go-stock-prediction/pkg/service/market"
    "go-stock-prediction/pkg/store/repository"
)

// Market implements market.AssetMarket for cryptocurrency.
type Market struct{}

var cryptoInstruments = []market.Instrument{
    {ID: "BTC/BINANCE", Name: "Bitcoin (Binance)"},
    {ID: "ETH/BINANCE", Name: "Ethereum (Binance)"},
}

func (m *Market) MarketKey() string  { return "CRYPTO" }
func (m *Market) MarketName() string { return "Cryptocurrency" }

func (m *Market) GetInstruments(ctx context.Context) ([]market.Instrument, error) {
    return cryptoInstruments, nil
}

// FetchPrices returns close prices (oldest→newest) for the instrument.
// DB returns DESC; reverse to ASC for algorithm input.
func (m *Market) FetchPrices(ctx context.Context, inst market.Instrument, months int) ([]float64, error) {
    symbol, exchange, _ := parseID(inst.ID)
    from := time.Now().AddDate(0, -months, 0)
    prices, err := repository.GetSingleton().GetCryptoPricesByDateRange(symbol, exchange, from, time.Now())
    if err != nil {
        return nil, err
    }
    result := make([]float64, len(prices))
    for i, p := range prices {
        f, _ := p.ClosePrice.Float64()
        result[len(prices)-1-i] = f // reverse DESC → ASC
    }
    return result, nil
}

func (m *Market) SavePrediction(ctx context.Context, inst market.Instrument, pred market.PredictionData) error {
    symbol, exchange, _ := parseID(inst.ID)
    return repository.GetSingleton().CreateCryptoPrediction(&modelsdb.CryptoPrediction{
        Symbol:         symbol,
        Exchange:       exchange,
        AlgorithmName:  pred.AlgorithmName,
        PredictedPrice: decimal.NewFromFloat(pred.PredictedPrice),
        CurrentPrice:   decimal.NewFromFloat(pred.CurrentPrice),
        Confidence:     decimal.NewFromFloat(pred.Confidence),
        PredictionDate: time.Now(),
        TargetDate:     pred.TargetDate,
    })
}

// Crawl delegates to the crypto crawler for on-demand crawling.
func (m *Market) Crawl(ctx context.Context) error {
    return crawler.CronjobCryptoCrawler()
}

func parseID(id string) (symbol, exchange string, err error) {
    parts := strings.SplitN(id, "/", 2)
    if len(parts) != 2 {
        return "", "", nil
    }
    return parts[0], parts[1], nil
}

func init() {
    market.Register(&Market{})
}
```

### Bước 5: Đăng ký trong cmd/prediction/main.go

Thêm đúng 1 dòng blank import:

```go
import (
    // ... imports hiện có ...
    _ "go-stock-prediction/pkg/service/market/crypto" // registers CRYPTO market
)
```

Orchestrator tự động picks up `"CRYPTO"` ở lần start tiếp theo — không cần sửa thêm file nào.

**Lưu ý:** Market imports cũng có thể thêm vào `pkg/service/predict/init.go` (xem pattern của GOLD và VN30 đã import ở đó). Thêm vào cả hai nơi không gây lỗi vì `market.Register()` là idempotent.

### Bước 6: gRPC Trigger (tùy chọn)

Để hỗ trợ trigger thủ công qua API, thêm vào `proto/prediction/prediction.proto`:

```protobuf
rpc TriggerCryptoPredict(Empty) returns (TriggerResponse);
rpc TriggerCryptoCrawler(Empty) returns (TriggerResponse);
```

Tái sinh proto:

```bash
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       proto/prediction/prediction.proto
```

Implement handler trong `pkg/grpc/server/server.go`:

```go
func (s *Server) TriggerCryptoPredict(ctx context.Context, req *pb.Empty) (*pb.TriggerResponse, error) {
    go orchestrator.RunForMarket(context.Background(), "CRYPTO")
    return &pb.TriggerResponse{Message: "Crypto prediction triggered"}, nil
}
```

### Bước 7: API Endpoints (tùy chọn)

Tạo `pkg/server/api_trigger_crypto_predict.go`:

```go
// POST /api/trigger/crypto-predict → gRPC TriggerCryptoPredict
func TriggerCryptoPredict(w http.ResponseWriter, r *http.Request) {
    client, ok := requireGRPCClient(w)
    if !ok {
        return
    }
    _, err := client.TriggerCryptoPredict(r.Context(), &pb.Empty{})
    if err != nil {
        ResponseError(w, http.StatusInternalServerError, err.Error())
        return
    }
    ResponseSuccess(w, http.StatusAccepted, map[string]string{"message": "triggered"})
}
```

Tạo `pkg/server/api_crypto_prediction.go` cho các GET endpoints (`/api/crypto/predictions`, v.v.).

Đăng ký trong `pkg/server/router.go`:

```go
router.HandleFunc("/api/trigger/crypto-predict", server.TriggerCryptoPredict).Methods("POST")
router.HandleFunc("/api/crypto/predictions", server.GetCryptoPredictions).Methods("GET")
```

### Bước 8: Frontend (tùy chọn)

File placeholder đã có tại `frontend/src/pages/Crypto.tsx` và route `/crypto` đã đăng ký. Xóa placeholder và implement đầy đủ tương tự `Gold.tsx`. Nav link đã có sẵn.

### Bước 9: Docker rebuild

```bash
docker-compose build prediction api
docker-compose up -d
```

---

## Checklist nhanh

| Bước | File/Action | Bắt buộc? |
|------|------------|-----------|
| DB model | `pkg/models/models_db/crypto_price.go` + `crypto_prediction.go` | Bắt buộc |
| Migration | `pkg/models/models_db/migrations.go` | Bắt buộc |
| Repository | `pkg/store/repository/repository.go` + `pkg/store/mysql/crypto.go` | Bắt buộc |
| Crawler | `pkg/service/crawler/crypto_crawler.go` + đăng ký trong `init.go` | Bắt buộc |
| AssetMarket | `pkg/service/market/crypto/market.go` — implement interface, gọi `market.Register()` trong `init()` | Bắt buộc |
| Đăng ký | `cmd/prediction/main.go` — thêm 1 dòng blank import | Bắt buộc |
| gRPC trigger | `proto/prediction/prediction.proto` + regenerate + `pkg/grpc/server/server.go` | Tùy chọn |
| API routes | `pkg/server/api_trigger_crypto_*.go` + `api_crypto_prediction.go` + `router.go` | Tùy chọn |
| Frontend page | `frontend/src/pages/Crypto.tsx` (placeholder đã có) | Tùy chọn |

---

## Lưu ý kỹ thuật

**Data ordering — QUAN TRỌNG:** `FetchPrices()` phải trả về prices theo thứ tự `oldest→newest` (ASC). DB thường trả DESC, vì vậy phải đảo ngược trong `FetchPrices()` trước khi return — xem cách VN30 và Gold market đã làm.

**Minimum data points:** Orchestrator bỏ qua instrument có dưới 20 data points. Đảm bảo crawler chạy đủ lâu trước khi prediction được trigger.

**Cron scheduling:** Prediction crypto tự động chạy hàng ngày lúc 6PM (Daily6PM cron) cùng với tất cả markets khác — không cần đăng ký cron riêng. Chỉ crawler mới cần đăng ký cron riêng trong `pkg/service/crawler/init.go`.

**Concurrency:** `market.Register()` thread-safe (dùng `sync.RWMutex`). `init()` function đảm bảo đăng ký xảy ra trước `main()`.
