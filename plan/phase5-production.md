# Phase 5: Performance & Production Readiness

Chuẩn bị để deploy thực sự. Không cần làm hết — chọn theo nhu cầu.

---

## 5.1 — Database: thêm composite indexes

**File:** `pkg/models/models_db/` (các model files)

**Vấn đề:**
Các queries phổ biến không có index tối ưu:
```sql
-- Query hay dùng:
WHERE stock_id = ? AND trade_date BETWEEN ? AND ?   -- không có index
WHERE stock_id = ? ORDER BY trade_date DESC LIMIT 1 -- scan toàn bộ
WHERE is_vn30 = true                                -- full table scan
```

**Cách sửa — thêm index vào GORM tags:**
```go
// models_db/stock_price.go
type StockPrice struct {
    gorm.Model
    StockID    uint      `gorm:"not null;index:idx_stock_date,priority:1"`
    TradeDate  time.Time `gorm:"not null;index:idx_stock_date,priority:2"`
    // ...
}

// models_db/stock.go
type Stock struct {
    gorm.Model
    Symbol  string `gorm:"uniqueIndex;not null"`
    IsVN30  bool   `gorm:"index"` // thêm index cho filter VN30
    // ...
}

// models_db/prediction.go
type Prediction struct {
    gorm.Model
    StockID       uint   `gorm:"not null;index:idx_pred_stock_algo,priority:1"`
    AlgorithmName string `gorm:"not null;index:idx_pred_stock_algo,priority:2"`
    TargetDate    time.Time `gorm:"index"`
    // ...
}
```

**Cần drop và recreate tables** hoặc chạy ALTER TABLE thủ công vì GORM auto-migrate
không tự thêm index mới vào bảng đã tồn tại theo mặc định.

---

## 5.2 — Thêm caching đơn giản cho market overview

**Tạo file:** `pkg/server/cache.go`

**Vấn đề:**
`GET /api/market/overview` query tất cả VN30 stocks mỗi lần request.
Dashboard auto-refresh 30 giây → 120 requests/giờ → 120 full table scans.

**Giải pháp:** In-memory cache với TTL (không cần Redis, dùng sync.Map):

```go
package server

import (
    "sync"
    "time"
)

type cacheEntry struct {
    data      interface{}
    expiresAt time.Time
}

type Cache struct {
    mu      sync.RWMutex
    entries map[string]cacheEntry
}

var globalCache = &Cache{entries: make(map[string]cacheEntry)}

func (c *Cache) Get(key string) (interface{}, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    entry, ok := c.entries[key]
    if !ok || time.Now().After(entry.expiresAt) {
        return nil, false
    }
    return entry.data, true
}

func (c *Cache) Set(key string, data interface{}, ttl time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.entries[key] = cacheEntry{data: data, expiresAt: time.Now().Add(ttl)}
}

func (c *Cache) Delete(key string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    delete(c.entries, key)
}
```

**Áp dụng trong market overview:**
```go
func (s *Server) GetMarketOverview(w http.ResponseWriter, r *http.Request) {
    const cacheKey = "market_overview"
    if cached, ok := globalCache.Get(cacheKey); ok {
        ResponseSuccess(w, http.StatusOK, cached)
        return
    }
    // ... existing logic ...
    globalCache.Set(cacheKey, result, 60*time.Second) // cache 1 phút
    ResponseSuccess(w, http.StatusOK, result)
}
```

**Invalidate cache sau khi crawler chạy xong.**

---

## 5.3 — Structured logging: bỏ emoji, thêm fields

**Files:** tất cả service files dùng `logger.Logger`

**Vấn đề:**
```go
logger.Logger.Info().Msg("🚀 Starting crawler...")
logger.Logger.Info().Msg("✅ Crawled VIC successfully")
```
- Emoji làm khó parse logs bằng tools (Grafana, ELK)
- Thiếu structured fields (stock symbol, duration, error context)

**Cách sửa:**
```go
// Thay:
logger.Logger.Info().Msg("🚀 Starting crawler...")

// Bằng:
logger.Logger.Info().Str("component", "crawler").Msg("Starting crawler")

// Thay:
logger.Logger.Info().Msgf("✅ Crawled %s successfully", symbol)

// Bằng:
logger.Logger.Info().
    Str("component", "crawler").
    Str("symbol", symbol).
    Dur("duration", time.Since(start)).
    Msg("Stock data crawled")

// Thay:
logger.Logger.Error().Err(err).Msgf("❌ Failed to crawl %s", symbol)

// Bằng:
logger.Logger.Error().
    Err(err).
    Str("component", "crawler").
    Str("symbol", symbol).
    Msg("Failed to crawl stock data")
```

**Scope:** Chỉ sửa khi đụng đến file đó — không cần sửa tất cả một lần.

---

## 5.4 — Graceful context cancellation trong crawler

**File:** `pkg/service/crawler/crawler.go`

**Vấn đề:**
Crawler không check context cancellation trong loop:
```go
for _, symbol := range symbols {
    c.crawlSingleStock(symbol) // không pass context
    time.Sleep(2 * time.Second) // không có context.Done() check
}
```

**Cách sửa:**
```go
for _, symbol := range symbols {
    select {
    case <-ctx.Done():
        logger.Logger.Info().Msg("Crawler cancelled")
        return ctx.Err()
    default:
    }

    if err := c.crawlSingleStock(ctx, symbol); err != nil {
        logger.Logger.Warn().Str("symbol", symbol).Err(err).Msg("Failed to crawl, continuing")
    }

    select {
    case <-ctx.Done():
        return ctx.Err()
    case <-time.After(2 * time.Second):
    }
}
```

---

## 5.5 — Health check: thêm DB ping thực sự

**File:** `pkg/server/web_handler.go`

**Vấn đề:**
`HealthCheckHandler` hiện tại check DB bằng cách query một record.
Nên dùng `db.Ping()` hoặc `SELECT 1` — nhanh hơn, ít tốn kém hơn.

**Cách sửa:**
```go
func checkDatabaseHealth(ctx context.Context) HealthStatus {
    start := time.Now()
    sqlDB, err := repository.GetSingleton().GetDB().DB()
    if err != nil {
        return HealthStatus{Status: "unhealthy", Message: err.Error()}
    }
    if err := sqlDB.PingContext(ctx); err != nil {
        return HealthStatus{Status: "unhealthy", Message: err.Error()}
    }
    return HealthStatus{
        Status:       "healthy",
        ResponseTime: time.Since(start).String(),
    }
}
```

---

## 5.6 — Dockerfile cải thiện

**File:** `Dockerfile` (xem và cải thiện)

**Kiểm tra:**
- Có dùng multi-stage build chưa (giảm image size)?
- Có chạy với non-root user không?
- Có HEALTHCHECK không?

**Template multi-stage build:**
```dockerfile
# Build stage
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o go-stock-prediction ./cmd/app

# Run stage
FROM alpine:latest
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app
COPY --from=builder /app/go-stock-prediction .
COPY --from=builder /app/web ./web
USER appuser
EXPOSE 31300
HEALTHCHECK --interval=30s --timeout=3s CMD wget -qO- http://localhost:31300/health/simple || exit 1
ENTRYPOINT ["./go-stock-prediction"]
```

---

## 5.7 — Retry logic cho crawler

**File:** `pkg/service/crawler/crawler.go`

**Vấn đề:**
Nếu VietStock trả về lỗi 1 lần, toàn bộ stock đó bị skip, không có retry.

**Cách sửa — thêm retry với exponential backoff:**
```go
func (c *VietStockCrawler) crawlWithRetry(ctx context.Context, symbol string, maxRetries int) error {
    var lastErr error
    for attempt := 0; attempt < maxRetries; attempt++ {
        if attempt > 0 {
            backoff := time.Duration(attempt*attempt) * time.Second
            select {
            case <-ctx.Done():
                return ctx.Err()
            case <-time.After(backoff):
            }
        }

        if err := c.crawlSingleStock(ctx, symbol); err != nil {
            lastErr = err
            logger.Logger.Warn().
                Str("symbol", symbol).
                Int("attempt", attempt+1).
                Err(err).
                Msg("Crawl attempt failed, retrying")
            continue
        }
        return nil
    }
    return fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}
```

---

## Checklist Phase 5

**Database:**
- [ ] 5.1 Thêm composite indexes vào GORM model tags

**Performance:**
- [ ] 5.2 Tạo `pkg/server/cache.go` + áp dụng cho market overview

**Observability:**
- [ ] 5.3 Refactor logging: bỏ emoji, thêm structured fields (sửa dần)
- [ ] 5.5 Sửa health check dùng DB ping

**Reliability:**
- [ ] 5.4 Thêm context cancellation vào crawler loop
- [ ] 5.7 Thêm retry logic với exponential backoff

**Deployment:**
- [ ] 5.6 Cải thiện Dockerfile (multi-stage, non-root user, HEALTHCHECK)

**Thời gian ước tính:** 1–2 ngày (làm dần, không cần làm tất cả cùng lúc)
**Ưu tiên trong phase này:** 5.1 (indexes) → 5.2 (cache) → 5.4 (context) → còn lại
