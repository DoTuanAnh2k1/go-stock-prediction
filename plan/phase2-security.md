# Phase 2: Security & Input Validation

Bảo vệ API khỏi lạm dụng và input độc hại.

---

## 2.1 — Thêm API key authentication cho trigger endpoints

**Files:** `pkg/server/router.go`, tạo mới `pkg/server/middleware_auth.go`

**Vấn đề:**
`POST /api/trigger/crawler` và `POST /api/trigger/predict` không có auth.
Bất kỳ ai cũng có thể kích hoạt crawl/train bất cứ lúc nào → tốn resource.

**Giải pháp:** API key đơn giản qua header `X-API-Key`.
(Không cần JWT vì đây là internal tool, không có user accounts.)

**Tạo file mới `pkg/server/middleware_auth.go`:**
```go
package server

import (
    "net/http"
    "github.com/user/go-stock-prediction/pkg/config"
)

func APIKeyMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        apiKey := r.Header.Get("X-API-Key")
        expectedKey := config.GetConfig().ServerConfig.APIKey
        if expectedKey == "" || apiKey != expectedKey {
            ResponseError(w, http.StatusUnauthorized, "Invalid or missing API key")
            return
        }
        next(w, r)
    }
}
```

**Thêm vào config:** `API_KEY=<random-string>` trong `.env` và struct config.

**Áp dụng trong router.go:**
```go
mux.HandleFunc("POST /api/trigger/crawler", APIKeyMiddleware(server.TriggerCrawlerHandler))
mux.HandleFunc("POST /api/trigger/predict", APIKeyMiddleware(server.TriggerPredictHandler))
```

---

## 2.2 — Thêm rate limiting cho tất cả API

**Files:** tạo mới `pkg/server/middleware_ratelimit.go`, cập nhật `pkg/server/router.go`

**Vấn đề:**
Không có rate limiting → có thể DDoS hoặc spam request làm DB quá tải.

**Giải pháp:** In-memory rate limiter per IP dùng `golang.org/x/time/rate`.
(Đã có trong go.mod.)

**Tạo file mới `pkg/server/middleware_ratelimit.go`:**
```go
package server

import (
    "net"
    "net/http"
    "sync"
    "time"
    "golang.org/x/time/rate"
)

type ipRateLimiter struct {
    limiters map[string]*rate.Limiter
    mu       sync.Mutex
    r        rate.Limit
    b        int
}

var globalLimiter = &ipRateLimiter{
    limiters: make(map[string]*rate.Limiter),
    r:        rate.Limit(10), // 10 requests/second per IP
    b:        30,             // burst 30
}

func (l *ipRateLimiter) getLimiter(ip string) *rate.Limiter {
    l.mu.Lock()
    defer l.mu.Unlock()
    limiter, exists := l.limiters[ip]
    if !exists {
        limiter = rate.NewLimiter(l.r, l.b)
        l.limiters[ip] = limiter
    }
    return limiter
}

func RateLimitMiddleware(next http.Handler) http.Handler {
    // Cleanup goroutine để tránh memory leak
    go func() {
        ticker := time.NewTicker(10 * time.Minute)
        for range ticker.C {
            globalLimiter.mu.Lock()
            globalLimiter.limiters = make(map[string]*rate.Limiter)
            globalLimiter.mu.Unlock()
        }
    }()

    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ip, _, err := net.SplitHostPort(r.RemoteAddr)
        if err != nil {
            ip = r.RemoteAddr
        }
        limiter := globalLimiter.getLimiter(ip)
        if !limiter.Allow() {
            ResponseError(w, http.StatusTooManyRequests, "Rate limit exceeded")
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

**Áp dụng trong `StartHTTPServer()`:**
```go
handler := RateLimitMiddleware(mux)
server := &http.Server{Handler: handler, ...}
```

---

## 2.3 — Validate input parameters ở tất cả API handlers

**Files:** `pkg/server/api_prediction.go`, `pkg/server/api_chart_data.go`,
`pkg/server/api_historical_data.go`, `pkg/server/api_current_price.go`

**Vấn đề hiện tại:**
```go
// api_prediction.go
symbol := r.URL.Query().Get("symbol")   // không validate — có thể là ""
limitStr := r.URL.Query().Get("limit")  // không validate — có thể là "-999"
algorithm := r.URL.Query().Get("algorithm") // không validate — SQL injection (dù GORM giúp)
```

**Tạo helper validation trong `pkg/server/helper.go`:**
```go
var validAlgorithms = map[string]bool{
    "moving_average": true,
    "lstm_nn":        true,
    "arima_garch":    true,
}

var validPeriods = map[string]bool{
    "1D": true, "1W": true, "1M": true,
    "3M": true, "6M": true, "1Y": true,
}

func validateSymbol(symbol string) error {
    if symbol == "" {
        return fmt.Errorf("symbol is required")
    }
    // VN stock symbols: 3-4 uppercase letters
    if len(symbol) < 2 || len(symbol) > 5 {
        return fmt.Errorf("invalid symbol format")
    }
    for _, c := range symbol {
        if !unicode.IsLetter(c) && !unicode.IsDigit(c) {
            return fmt.Errorf("symbol contains invalid characters")
        }
    }
    return nil
}

func validateLimit(limitStr string, defaultLimit, maxLimit int) (int, error) {
    if limitStr == "" {
        return defaultLimit, nil
    }
    limit, err := strconv.Atoi(limitStr)
    if err != nil || limit < 1 || limit > maxLimit {
        return 0, fmt.Errorf("limit must be between 1 and %d", maxLimit)
    }
    return limit, nil
}

func validateAlgorithm(algorithm string) error {
    if algorithm == "" {
        return nil // optional filter
    }
    if !validAlgorithms[algorithm] {
        return fmt.Errorf("unknown algorithm: %s", algorithm)
    }
    return nil
}
```

**Áp dụng trong handlers:**
```go
// api_prediction.go
func (s *Server) GetPredictions(w http.ResponseWriter, r *http.Request) {
    symbol := r.URL.Query().Get("symbol")
    if err := validateSymbol(symbol); err != nil {
        ResponseError(w, http.StatusBadRequest, err.Error())
        return
    }

    algorithm := r.URL.Query().Get("algorithm")
    if err := validateAlgorithm(algorithm); err != nil {
        ResponseError(w, http.StatusBadRequest, err.Error())
        return
    }

    limit, err := validateLimit(r.URL.Query().Get("limit"), 20, 100)
    if err != nil {
        ResponseError(w, http.StatusBadRequest, err.Error())
        return
    }
    ...
}
```

---

## 2.4 — Thêm CORS headers

**File:** tạo mới `pkg/server/middleware_cors.go`

**Vấn đề:**
Nếu frontend được serve từ domain khác (e.g., deploy frontend riêng), browser sẽ block request.

**Tạo file:**
```go
package server

import "net/http"

func CORSMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*") // hoặc specific domain
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")

        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusNoContent)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

**Áp dụng:**
```go
handler := CORSMiddleware(RateLimitMiddleware(mux))
```

---

## 2.5 — Xóa credentials hardcode

**Vấn đề:**
`.env` có `MYSQL_PASSWORD=123` — mật khẩu quá yếu, và default password trong code
nên được xóa khỏi git history nếu đã commit.

**Bước:**
1. Đổi `MYSQL_PASSWORD=123` thành password mạnh hơn trong `.env`
2. Kiểm tra `.gitignore` có ignore `.env` chưa — nếu chưa, thêm vào
3. Kiểm tra git log xem `.env` đã bị commit chưa, nếu có cần xóa khỏi history
4. Đổi default trong `pkg/config/init.go`:
   ```go
   DbPassword: getEnv("MYSQL_PASSWORD", ""), // bỏ default "123"
   ```
   Nếu không set env, app fail sớm thay vì kết nối bằng password mặc định.

---

## Checklist Phase 2

- [ ] 2.1 Tạo `middleware_auth.go` + thêm `API_KEY` vào config và `.env`
- [ ] 2.2 Tạo `middleware_ratelimit.go` + bọc router
- [ ] 2.3 Thêm validation helpers + áp dụng vào tất cả API handlers
- [ ] 2.4 Tạo `middleware_cors.go`
- [ ] 2.5 Xóa hardcoded password, verify `.gitignore`

**Thời gian ước tính:** 4–6 giờ
**Rủi ro nếu bỏ qua:** Resource exhaustion, unexpected trigger, potential data corruption
