# Security Risk Assessment — go-stock-prediction

> Đánh giá dành riêng cho việc **public web ra ngoài internet**.
> Kiểm tra ngày: 2026-06-02 | Nhánh: `main`

---

## Mục lục

- [Tóm tắt nhanh (TL;DR)](#tóm-tắt-nhanh)
- [CRITICAL (phải fix trước khi public)](#critical)
- [HIGH (fix trước khi public)](#high)
- [MEDIUM (fix sớm sau khi public)](#medium)
- [LOW (cải thiện dần)](#low)
- [Checklist trước khi public](#checklist-trước-khi-public)

---

## Tóm tắt nhanh

| Mức độ | Số lỗi | Trạng thái |
|--------|--------|-----------|
| CRITICAL | 6 | Chưa fix — **KHÔNG public khi còn các lỗi này** |
| HIGH | 8 | Chưa fix |
| MEDIUM | 7 | Chưa fix |
| LOW | 4 | Chưa fix |

---

## CRITICAL

### C1. Credentials bị hardcode trong `docker-compose.yaml`

**File:** [`docker-compose.yaml`](docker-compose.yaml) — dòng 16, 45, 70–79, 121–125

**Vấn đề:** Toàn bộ credentials quan trọng đang được hardcode trực tiếp trong file commit vào git:

```yaml
MYSQL_ROOT_PASSWORD: 123           # MySQL root password
MYSQL_PASSWORD: password           # dbuser password
MYSQL_PASSWORD: 123                # password dùng bởi prediction & api service
ADMIN_PASSWORD: "admin123"         # admin account đăng nhập dashboard
JWT_SECRET: "go-stock-prediction-jwt-secret-2024"  # secret ký toàn bộ JWT
PMA_PASSWORD: 123                  # phpMyAdmin root password
```

**Rủi ro khi public:** Bất kỳ ai có git history (kể cả fork, cache, Wayback Machine) sẽ có toàn bộ credentials. JWT secret bị lộ đồng nghĩa với việc attacker có thể tự ký token admin hợp lệ mà không cần đăng nhập.

**Fix:**
```bash
# Tạo docker-compose.override.yaml (KHÔNG commit) hoặc dùng .env file riêng
cp docker-compose.yaml docker-compose.yaml.example
echo "docker-compose.override.yaml" >> .gitignore
echo ".env.production" >> .gitignore
```
Dùng biến `${VAR}` trong docker-compose và export từ file `.env` không được commit.

---

### C2. JWT Secret yếu / mặc định

**File:** [`pkg/config/init.go`](pkg/config/init.go) — `JWTSecret` default `"change-me-in-production"`

**Vấn đề:** Nếu `JWT_SECRET` không được set trong môi trường, hệ thống dùng secret có thể đoán được. Kẻ tấn công biết secret có thể tự forge token admin:

```bash
# Attacker tự tạo token admin hợp lệ
jwt_tool --sign '{"sub":"attacker","role":"admin","user_id":1}' \
  --secret 'go-stock-prediction-jwt-secret-2024'
```

**Fix:**
```bash
# Sinh secret mạnh (32 bytes trở lên)
openssl rand -base64 48
# Đặt vào .env.production và inject qua docker secret
```

---

### C3. gRPC server không có authentication

**File:** [`prediction/src/grpc_server/server.py`](prediction/src/grpc_server/server.py) — `add_insecure_port()`

**Vấn đề:** gRPC server chạy trên port 8119 không có bất kỳ cơ chế xác thực nào. Nếu port này bị expose ra ngoài (hoặc có attacker vào được Docker network), họ có thể:
- Trigger crawl/train/backtest không giới hạn → cạn kiệt CPU/RAM
- Trigger `TriggerHistoricalBacktest` → chạy backtest nặng liên tục
- Đọc dữ liệu training status

**Fix ngắn hạn:** Đảm bảo port 8119 KHÔNG bao giờ được expose ra ngoài Docker network (không có `ports:` trong docker-compose cho service `prediction`). Hiện tại đã đúng — chỉ cần cẩn thận không thêm port mapping.

**Fix dài hạn:** Thêm interceptor authentication vào gRPC server:
```python
# Thêm metadata interceptor kiểm tra API key
class AuthInterceptor(grpc.ServerInterceptor):
    def intercept_service(self, continuation, handler_call_details):
        metadata = dict(handler_call_details.invocation_metadata)
        if metadata.get('x-api-key') != INTERNAL_API_KEY:
            raise grpc.RpcError(grpc.StatusCode.UNAUTHENTICATED)
        return continuation(handler_call_details)
```

---

### C4. `POST /api/stocks/{symbol}/crawl` và `/predict` không cần authentication

**File:** [`pkg/server/router.go`](pkg/server/router.go) — dòng 113–114

**Vấn đề:** Hai endpoint này **không** được wrap bằng `AuthRequired()`:

```go
// router.go — KHÔNG có AuthRequired
mux.HandleFunc("POST /api/stocks/{symbol}/crawl", TriggerStockCrawl)
mux.HandleFunc("POST /api/stocks/{symbol}/predict", TriggerStockPredict)
```

Trong khi tất cả trigger khác đều được bảo vệ. Bất kỳ ai có thể gửi:
```bash
curl -X POST https://yourdomain.com/api/stocks/VCB/crawl  # không cần token
curl -X POST https://yourdomain.com/api/stocks/AAPL/predict
```

**Fix:**
```go
mux.HandleFunc("POST /api/stocks/{symbol}/crawl", AuthRequired(TriggerStockCrawl))
mux.HandleFunc("POST /api/stocks/{symbol}/predict", AuthRequired(TriggerStockPredict))
```

---

### C5. `GET/POST/DELETE /api/users` không có `AuthRequired` wrapper ở router level

**File:** [`pkg/server/router.go`](pkg/server/router.go) — dòng 137–139

**Vấn đề:**
```go
// Không có AuthRequired wrapper — authentication chỉ xảy ra bên trong handler
mux.HandleFunc("GET /api/users", ListUsersHandler)
mux.HandleFunc("POST /api/users", CreateUserHandler)
mux.HandleFunc("DELETE /api/users/{id}", DeleteUserHandler)
```

Hiện tại handler gọi `requireAdmin()` bên trong, nhưng đây là pattern nguy hiểm. Nếu developer sau này sửa handler và vô tình bỏ check, hoặc có race condition, request unauthenticated vẫn vào được handler. Defense-in-depth yêu cầu auth ở router level.

**Fix:**
```go
mux.HandleFunc("GET /api/users", AuthRequired(ListUsersHandler))
mux.HandleFunc("POST /api/users", AuthRequired(CreateUserHandler))
mux.HandleFunc("DELETE /api/users/{id}", AuthRequired(DeleteUserHandler))
```

---

### C6. Database password bị log ra plaintext

**File:** [`pkg/store/mysql/mysql.go`](pkg/store/mysql/mysql.go) — dòng 47

**Vấn đề:**
```go
dsn := DbUsername + ":" + DbPassword + "@tcp(" + DbHost + ":" + DbPort + ")/" + DbName + "?..."
logger.Logger.Infof("Connect to database: %s", dsn)
// → "Connect to database: root:123@tcp(db:3306)/go_stock_prediction?..."
```

Password xuất hiện trong plaintext trong stdout/stderr của container. Nếu logs được ship đến Datadog, CloudWatch, Papertrail, hay bất kỳ log aggregator nào, password bị lộ.

**Fix:**
```go
// Mask password trước khi log
dsnForLog := DbUsername + ":***@tcp(" + DbHost + ":" + DbPort + ")/" + DbName
logger.Logger.Infof("Connect to database: %s", dsnForLog)
```

---

## HIGH

### H1. Rate limiter vô hiệu hóa khi đứng sau Nginx

**File:** [`pkg/server/middleware_ratelimit.go`](pkg/server/middleware_ratelimit.go) — dòng 47–52

**Vấn đề:** Rate limiter dùng `r.RemoteAddr` để xác định IP:
```go
ip, _, err := net.SplitHostPort(r.RemoteAddr)
```

Khi đứng sau Nginx, `r.RemoteAddr` luôn là IP của Nginx container (e.g. `172.18.0.x`) — **tất cả client đều share cùng một rate limiter bucket**. Rate limit 60 req/s áp dụng cho toàn bộ traffic, không phải per-IP. Attacker có thể spam đến 60 req/s và chỉ bị throttle khi làm chậm toàn bộ hệ thống.

Nginx đã forward `X-Real-IP` header nhưng Go không đọc nó.

**Fix:**
```go
func getRealIP(r *http.Request) string {
    if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
        return realIP
    }
    if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
        return strings.Split(forwarded, ",")[0]
    }
    ip, _, _ := net.SplitHostPort(r.RemoteAddr)
    return ip
}
```
**Lưu ý:** Chỉ tin `X-Real-IP` khi đã verify Nginx set header này và không có proxy khác phía trước có thể fake nó.

---

### H2. Không có rate limiting riêng cho login endpoint

**File:** [`pkg/server/api_auth.go`](pkg/server/api_auth.go) — `LoginHandler`

**Vấn đề:** Login endpoint dùng chung rate limiter 60 req/s với tất cả endpoint khác. Không có:
- Lockout sau N lần đăng nhập sai
- Captcha
- Delay tăng dần (exponential backoff)
- Tracking per-username (chỉ per-IP)

Attacker có thể brute force password qua Tor/VPN thay đổi IP liên tục.

**Fix tối thiểu:** Thêm dedicated limiter cho login, tối đa 5 attempts/minute/IP:
```go
var loginLimiter = &ipRateLimiter{r: rate.Limit(5.0/60), b: 5}
```
Log failed attempts với username để phát hiện distributed brute force.

---

### H3. phpMyAdmin và MySQL port exposed ra ngoài

**File:** [`docker-compose.yaml`](docker-compose.yaml) — dòng 13–14, 119

**Vấn đề:**
```yaml
db:
  ports:
    - "3306:3306"   # MySQL exposed ra host

phpmyadmin:
  ports:
    - "8081:80"     # phpMyAdmin exposed ra host
```

Nếu server public, `http://yourserver:8081` là giao diện quản trị MySQL đầy đủ với password `123`. Port 3306 cũng bị exposed cho mọi IP.

**Fix:** Xóa port mapping khỏi cả 2 service. Chúng nằm trong `app-network` và chỉ cần accessible trong Docker network:
```yaml
db:
  # Xóa ports block hoàn toàn — chỉ access trong Docker network

phpmyadmin:
  # Chỉ expose qua localhost binding nếu cần dev:
  ports:
    - "127.0.0.1:8081:80"  # chỉ từ localhost
```

---

### H4. Swagger UI (`/swagger/`) accessible công khai

**File:** [`pkg/server/router.go`](pkg/server/router.go) — dòng 14–16; [`nginx/nginx.conf`](nginx/nginx.conf)

**Vấn đề:** Swagger UI không có bất kỳ protection nào. Khi public, attacker có bản đồ đầy đủ toàn bộ API endpoints, request format, và response schema. Đây là reconnaissance tool hoàn hảo.

**Fix:**
```nginx
# nginx.conf — chặn Swagger trên production
location /swagger/ {
    deny all;
    return 404;
}
```
Hoặc thêm basic auth chỉ cho internal access.

---

### H5. CORS wildcard `Access-Control-Allow-Origin: *`

**File:** [`pkg/server/middleware_cors.go`](pkg/server/middleware_cors.go) — dòng 8

**Vấn đề:**
```go
w.Header().Set("Access-Control-Allow-Origin", "*")
```

Bất kỳ website nào (`evil.com`) cũng có thể gọi API từ browser của user đang đăng nhập. Kết hợp với việc thiếu CSRF token, attacker có thể:
- Đọc toàn bộ data (predictions, stocks)
- Trigger crawl/predict nếu user đang login

**Fix:**
```go
allowedOrigins := map[string]bool{
    "https://yourdomain.com": true,
    "http://localhost:36018": true,  // dev only
}
origin := r.Header.Get("Origin")
if allowedOrigins[origin] {
    w.Header().Set("Access-Control-Allow-Origin", origin)
    w.Header().Set("Vary", "Origin")
}
```

---

### H6. Không có HTTPS — toàn bộ traffic truyền cleartext

**File:** [`docker-compose.yaml`](docker-compose.yaml), [`nginx/nginx.conf`](nginx/nginx.conf)

**Vấn đề:** Nginx chỉ listen port 80 (HTTP). Khi public:
- JWT token truyền cleartext trong header `Authorization: Bearer ...`
- Bất kỳ network observer nào (ISP, CDN, man-in-middle) có thể steal token
- Browser hiện đại cảnh báo "Not Secure"

**Fix:** Thêm TLS termination tại Nginx. Dùng Let's Encrypt (Certbot) hoặc Cloudflare proxy:
```nginx
server {
    listen 443 ssl;
    ssl_certificate /etc/letsencrypt/live/yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/yourdomain.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
}
server {
    listen 80;
    return 301 https://$host$request_uri;
}
```

---

### H7. ORDER BY string concatenation — SQL injection tiềm ẩn

**Files:**
- [`pkg/store/mysql/prediction.go`](pkg/store/mysql/prediction.go) — dòng 284
- [`pkg/store/mysql/training_log.go`](pkg/store/mysql/training_log.go)
- [`pkg/store/mysql/gold_prediction.go`](pkg/store/mysql/gold_prediction.go)
- [`pkg/store/mysql/nasdaq.go`](pkg/store/mysql/nasdaq.go), [`crypto.go`](pkg/store/mysql/crypto.go), [`fuel.go`](pkg/store/mysql/fuel.go), [`sp500.go`](pkg/store/mysql/sp500.go)

**Vấn đề:** GORM không parameterize ORDER BY clause. Các file trên dùng pattern:
```go
Order("predictions." + sortBy + " " + sortDir)
```
Hiện tại có whitelist bảo vệ `sortBy`, nhưng:
1. Pattern nguy hiểm — nếu thêm field mới mà quên validate là SQLi ngay
2. `sortDir` chỉ check `== "asc"`, KHÔNG normalize, nếu có bug sẽ bị inject

**Fix an toàn hơn:**
```go
// Dùng GORM column reference thay vì string concatenation
validColumns := map[string]string{
    "prediction_date": "predictions.prediction_date",
    "target_date":     "predictions.target_date",
    "accuracy":        "predictions.accuracy",
}
col, ok := validColumns[sortBy]
if !ok { col = "predictions.prediction_date" }
dir := "DESC"
if strings.EqualFold(sortDir, "asc") { dir = "ASC" }
query.Order(gorm.Expr(col + " " + dir))
```

---

### H8. Minimum password length chỉ 6 ký tự

**File:** [`pkg/server/api_auth.go`](pkg/server/api_auth.go) — dòng 134

**Vấn đề:**
```go
if len(req.NewPassword) < 6 {
```
6 ký tự là quá yếu. Không có yêu cầu về độ phức tạp. `password`, `123456`, `admin1` đều hợp lệ.

**Fix:**
```go
func validatePassword(p string) error {
    if len(p) < 12 {
        return errors.New("password must be at least 12 characters")
    }
    // Tùy chọn: kiểm tra có chứa digit, uppercase, lowercase
    return nil
}
```

---

## MEDIUM

### M1. Thiếu security headers

**File:** [`pkg/server/middleware_cors.go`](pkg/server/middleware_cors.go)

**Vấn đề:** Không có các HTTP security headers quan trọng:

| Header thiếu | Rủi ro |
|---|---|
| `X-Frame-Options: DENY` | Clickjacking — site của attacker nhúng app vào iframe |
| `X-Content-Type-Options: nosniff` | MIME sniffing attack |
| `Referrer-Policy: strict-origin-when-cross-origin` | Lộ URL khi redirect |
| `Content-Security-Policy` | XSS thông qua inline script |

**Fix:**
```go
w.Header().Set("X-Frame-Options", "DENY")
w.Header().Set("X-Content-Type-Options", "nosniff")
w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
w.Header().Set("Content-Security-Policy", "default-src 'self'")
```

---

### M2. Không có CSRF protection

**File:** [`pkg/server/router.go`](pkg/server/router.go) — tất cả POST/PUT/DELETE

**Vấn đề:** Kết hợp CORS `*` + không có CSRF token, attacker có thể tạo website dụ user click button, rồi gọi API trong background. Ví dụ:
```html
<!-- evil.com/attack.html -->
<script>
  fetch("https://yourdomain.com/api/users", {
    method: "POST",
    headers: {"Content-Type": "application/json", "Authorization": "Bearer " + localStorage.getItem("token")},
    body: JSON.stringify({username:"hacker", password:"h4ck3r", role:"admin"})
  })
</script>
```
(localStorage có thể bị đọc nếu có XSS trên cùng domain)

**Fix:** Khi đã fix CORS (whitelist origin), CORS cũng chặn cross-origin requests. Thêm `SameSite=Strict` nếu chuyển sang cookie-based auth.

---

### M3. JWT lưu trong `localStorage` — XSS có thể steal token

**File:** [`frontend/src/context/AuthContext.tsx`](frontend/src/context/AuthContext.tsx)

**Vấn đề:** Token lưu trong `localStorage` dễ bị đọc bởi bất kỳ JavaScript nào chạy trên cùng domain (XSS). `sessionStorage` chỉ tốt hơn một chút. Cookie với flag `HttpOnly` là cách duy nhất JS không đọc được.

**Fix dài hạn:** Chuyển sang HttpOnly cookie:
```go
http.SetCookie(w, &http.Cookie{
    Name:     "auth_token",
    Value:    signed,
    HttpOnly: true,
    Secure:   true,  // chỉ HTTPS
    SameSite: http.SameSiteStrictMode,
    MaxAge:   86400,
})
```

---

### M4. Rate limiter reset toàn bộ sau 10 phút

**File:** [`pkg/server/middleware_ratelimit.go`](pkg/server/middleware_ratelimit.go) — dòng 37–44

**Vấn đề:**
```go
ticker := time.NewTicker(10 * time.Minute)
for range ticker.C {
    globalLimiter.limiters = make(map[string]*rate.Limiter) // reset tất cả
}
```
Memory leak tuy được giải quyết, nhưng attacker có thể: burst đến limit → đợi 10 phút → burst lại. Không có window-based counting, chỉ có token bucket không persistent.

**Fix:** Dùng sliding window hoặc fixed window per-IP thay vì reset toàn bộ.

---

### M5. Không có audit log cho admin actions

**File:** [`pkg/server/api_users.go`](pkg/server/api_users.go)

**Vấn đề:** Không có ghi nhận khi:
- User tạo / xóa (ai làm, lúc nào)
- Ai login thành công / thất bại
- Ai trigger crawl/train

Khi bị tấn công không có forensics để điều tra.

**Fix tối thiểu:**
```go
logger.Logger.Infof("AUDIT: user_id=%d created new user %s with role %s", adminID, newUsername, role)
logger.Logger.Warnf("AUDIT: failed login attempt for username=%s from ip=%s", username, ip)
```

---

### M6. phpMyAdmin expose toàn bộ database qua web UI

**File:** [`docker-compose.yaml`](docker-compose.yaml) — dòng 111–126

**Vấn đề:** `http://yourserver:8081` cho phép:
- Xem, sửa, xóa toàn bộ data (predictions, users, passwords)
- Export toàn bộ database
- Chạy arbitrary SQL
- Upload PHP file (nếu version cũ)

Dùng root với password `123`.

**Fix:** Nếu không cần thiết trên production, xóa hoàn toàn service này. Nếu cần admin đôi khi, chỉ start khi cần và bind localhost.

---

### M7. bcrypt cost 10 (default) — nên dùng cao hơn

**File:** [`pkg/server/api_auth.go`](pkg/server/api_auth.go) — dòng 155

**Vấn đề:** `bcrypt.DefaultCost = 10`. Với GPU hiện đại, cost 10 có thể bị crack. Cost 12–14 được khuyến nghị cho 2026.

**Fix:**
```go
const bcryptCost = 12
hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
```

---

## LOW

### L1. Không có Content-Security-Policy header

**File:** [`pkg/server/middleware_cors.go`](pkg/server/middleware_cors.go)

Thiếu CSP cho phép inline scripts và external resource loading — tăng attack surface cho XSS.

---

### L2. Thông tin server lộ qua error responses

**File:** [`pkg/server/helper.go`](pkg/server/helper.go)

Error message "stock not found" cho phép enumerate stock symbols. Nên trả 404 generic.

---

### L3. Database connection pool không configure

**File:** [`pkg/store/mysql/mysql.go`](pkg/store/mysql/mysql.go)

Không set `MaxOpenConns`, `MaxIdleConns`, `ConnMaxLifetime`. Dưới DoS load, có thể exhaust MySQL connections.

**Fix tối thiểu:**
```go
sqlDB, _ := db.DB()
sqlDB.SetMaxOpenConns(25)
sqlDB.SetMaxIdleConns(10)
sqlDB.SetConnMaxLifetime(5 * time.Minute)
```

---

### L4. Không có giới hạn kích thước request body

**File:** [`pkg/server/server.go`](pkg/server/server.go)

Không có `http.MaxBytesReader` → có thể upload request body lớn tùy ý để exhaust memory.

**Fix:**
```go
r.Body = http.MaxBytesReader(w, r.Body, 1*1024*1024) // max 1MB
```

---

## Checklist trước khi public

### Bắt buộc (CRITICAL + HIGH)

- [ ] **C1** — Xóa tất cả hardcoded credentials khỏi `docker-compose.yaml`, dùng `.env` file không commit
- [ ] **C2** — Sinh JWT secret ngẫu nhiên 48+ bytes, inject qua environment
- [ ] **C3** — Đảm bảo port 8119 (gRPC) KHÔNG bao giờ expose ra ngoài Docker network
- [ ] **C4** — Wrap `TriggerStockCrawl` và `TriggerStockPredict` bằng `AuthRequired()`
- [ ] **C5** — Wrap `ListUsersHandler`, `CreateUserHandler`, `DeleteUserHandler` bằng `AuthRequired()` ở router level
- [ ] **C6** — Mask password trong DSN trước khi log
- [ ] **H1** — Fix rate limiter dùng `X-Real-IP` header thay vì `r.RemoteAddr`
- [ ] **H2** — Thêm dedicated rate limiter cho login (≤ 10 req/min/IP)
- [ ] **H3** — Xóa port mapping `3306:3306` và `8081:80` khỏi docker-compose (production)
- [ ] **H4** — Block `/swagger/` trên production tại Nginx
- [ ] **H5** — Thay `Access-Control-Allow-Origin: *` bằng whitelist cụ thể
- [ ] **H6** — Cấu hình HTTPS (Let's Encrypt) + redirect HTTP → HTTPS tại Nginx
- [ ] **H8** — Tăng minimum password length lên 12 ký tự

### Nên làm sớm (MEDIUM)

- [ ] **M1** — Thêm security headers (`X-Frame-Options`, `X-Content-Type-Options`, `Referrer-Policy`)
- [ ] **M5** — Thêm audit logging cho login và admin actions
- [ ] **M6** — Xóa phpMyAdmin hoặc bind localhost-only
- [ ] **M7** — Tăng bcrypt cost lên 12

### Cải thiện dần (LOW)

- [ ] **L3** — Configure MySQL connection pool limits
- [ ] **L4** — Thêm `http.MaxBytesReader` cho request body

---

*File này được tạo tự động từ security audit ngày 2026-06-02. Cập nhật lại sau mỗi lần fix.*
