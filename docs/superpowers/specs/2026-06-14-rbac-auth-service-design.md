# RBAC + Java Auth Service — Design Spec
**Date:** 2026-06-14
**Status:** Approved

## Tóm tắt

Tách auth/RBAC ra thành một Java microservice độc lập giao tiếp qua gRPC. Hệ thống phân quyền gồm 3 role (`super_admin`, `admin`, `user`) kết hợp market group để kiểm soát access vào từng thị trường.

---

## 1. Tổng quan kiến trúc

```
Frontend
    │ HTTP
    ▼
Nginx :80
    │
    ▼
Go API :8118
    ├─ JWT validate LOCAL (shared secret) ──► market endpoints
    ├─ gRPC :8120 → Java Auth Service ───────► auth, users, market-groups
    └─ gRPC :8119 → Python Prediction ──────► crawl, train, predict

Java Auth Service :8120 (gRPC, internal only)
    └─ MySQL go_stock_prediction (shared)
         ├─ users                 (hiện có — thêm role super_admin)
         ├─ market_groups         [mới]
         ├─ market_group_markets  [mới]
         └─ user_market_groups    [mới]
```

**Nguyên tắc:**
- Go API không còn truy cập DB cho user/auth — mọi thao tác đi qua gRPC.
- JWT do Java sinh, Go chỉ validate local bằng `JWT_SECRET` shared.
- Market access check: đọc `accessible_markets` từ JWT claims — không cần gọi Java mỗi request.
- Thay đổi market group cần re-login để JWT được cấp lại với claims mới.

---

## 2. Roles & Hierarchy

| Role | Trigger | Market access | Quản lý users | Quản lý market groups |
|------|:-------:|:-------------:|:-------------:|:---------------------:|
| `super_admin` | ✓ | Tất cả | ✓ (kể cả super_admin) | ✓ |
| `admin` | ✓ | Group only | ✓ (không đụng super_admin) | ✓ |
| `user` | ✗ | Group only | ✗ | ✗ |

**Super admin cố định:** `chon / Ch1nch2n@` — seeded mỗi lần Java service khởi động. Nếu đã tồn tại thì bỏ qua. Không thể bị xóa hay edit bởi `admin`.

**Chưa có group = không có access:** user/admin chưa được gán group nào → `accessible_markets: []` trong JWT → UI ẩn tất cả market tabs → backend trả 403 nếu gọi trực tiếp.

---

## 3. JWT Payload

Java Auth Service ký JWT bằng `JWT_SECRET` (env var shared với Go API):

```json
{
  "sub": "1",
  "username": "chon",
  "role": "super_admin",
  "accessible_markets": ["GOLD", "NASDAQ", "CRYPTO", "SP500"],
  "exp": 1750000000
}
```

- `super_admin`: luôn có đủ 4 markets trong claims.
- `admin`/`user`: chỉ có markets thuộc các groups được gán.
- `accessible_markets: []` khi chưa có group.
- TTL: 24h (giữ nguyên như hiện tại).

---

## 4. DB Schema (bảng mới + thay đổi)

Schema mới do **Flyway trong Java** quản lý. Go GORM auto-migrate không đụng vào các bảng này.

**`users`** — role column mở rộng thêm giá trị `super_admin` (app-level validation):
```
role VARCHAR(20) NOT NULL DEFAULT 'user'
-- valid values: 'super_admin', 'admin', 'user'
```

**`market_groups`** (mới):
```sql
CREATE TABLE market_groups (
    id          BIGINT AUTO_INCREMENT PRIMARY KEY,
    name        VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    created_at  DATETIME NOT NULL,
    updated_at  DATETIME NOT NULL
);
```

**`market_group_markets`** (join — mới):
```sql
CREATE TABLE market_group_markets (
    group_id    BIGINT      NOT NULL,
    market_key  VARCHAR(20) NOT NULL,   -- 'GOLD' | 'NASDAQ' | 'CRYPTO' | 'SP500'
    PRIMARY KEY (group_id, market_key),
    FOREIGN KEY (group_id) REFERENCES market_groups(id) ON DELETE CASCADE
);
```

**`user_market_groups`** (join — mới):
```sql
CREATE TABLE user_market_groups (
    user_id   BIGINT NOT NULL,
    group_id  BIGINT NOT NULL,
    PRIMARY KEY (user_id, group_id),
    FOREIGN KEY (user_id)  REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (group_id) REFERENCES market_groups(id) ON DELETE CASCADE
);
```

---

## 5. gRPC Contract — auth.proto

File đặt tại `auth-service/proto/auth.proto`. Go stubs generated vào `api/proto/auth/`.

```protobuf
syntax = "proto3";
package auth;
option java_package = "vn.gostock.auth.proto";
option go_package = "go-stock-prediction/proto/auth";

service AuthService {
  // Auth
  rpc Login(LoginRequest)               returns (LoginResponse);
  rpc GetMe(CallerMeta)                 returns (UserResponse);
  rpc ChangePassword(ChangePassRequest) returns (Empty);

  // User management
  rpc ListUsers(CallerMeta)             returns (ListUsersResponse);
  rpc CreateUser(CreateUserRequest)     returns (UserResponse);
  rpc DeleteUser(DeleteUserRequest)     returns (Empty);
  rpc UpdateUserRole(UpdateRoleRequest) returns (UserResponse);

  // Market groups
  rpc ListMarketGroups(CallerMeta)              returns (ListGroupsResponse);
  rpc CreateMarketGroup(CreateGroupRequest)     returns (MarketGroupResponse);
  rpc UpdateMarketGroup(UpdateGroupRequest)     returns (MarketGroupResponse);
  rpc DeleteMarketGroup(DeleteGroupRequest)     returns (Empty);
  rpc SetGroupMarkets(SetGroupMarketsRequest)   returns (Empty);
  rpc ListGroupUsers(GroupRequest)              returns (ListUsersResponse);
  rpc AddUserToGroup(UserGroupRequest)          returns (Empty);
  rpc RemoveUserFromGroup(UserGroupRequest)     returns (Empty);
  rpc GetUserMarketGroups(UserRequest)          returns (ListGroupsResponse);
}

// Caller context — injected by Go API from JWT claims
message CallerMeta {
  int64  caller_id   = 1;
  string caller_role = 2;
}

message Empty {}

message LoginRequest {
  string username = 1;
  string password = 2;
}

message LoginResponse {
  string token    = 1;
  string username = 2;
  string role     = 3;
}

message UserResponse {
  int64  id         = 1;
  string username   = 2;
  string role       = 3;
  string created_at = 4;
}

message ListUsersResponse {
  repeated UserResponse users = 1;
}

message CreateUserRequest {
  CallerMeta caller   = 1;
  string     username = 2;
  string     password = 3;
  string     role     = 4;
}

message DeleteUserRequest {
  CallerMeta caller    = 1;
  int64      target_id = 2;
}

message UpdateRoleRequest {
  CallerMeta caller    = 1;
  int64      target_id = 2;
  string     new_role  = 3;
}

message ChangePassRequest {
  CallerMeta caller       = 1;
  string     old_password = 2;
  string     new_password = 3;
}

message MarketGroupResponse {
  int64             id           = 1;
  string            name         = 2;
  string            description  = 3;
  repeated string   market_keys  = 4;
  string            created_at   = 5;
  string            updated_at   = 6;
}

message ListGroupsResponse {
  repeated MarketGroupResponse groups = 1;
}

message CreateGroupRequest {
  CallerMeta caller      = 1;
  string     name        = 2;
  string     description = 3;
}

message UpdateGroupRequest {
  CallerMeta caller      = 1;
  int64      group_id    = 2;
  string     name        = 3;
  string     description = 4;
}

message DeleteGroupRequest {
  CallerMeta caller   = 1;
  int64      group_id = 2;
}

message SetGroupMarketsRequest {
  CallerMeta      caller      = 1;
  int64           group_id    = 2;
  repeated string market_keys = 3;
}

message GroupRequest {
  CallerMeta caller   = 1;
  int64      group_id = 2;
}

message UserGroupRequest {
  CallerMeta caller   = 1;
  int64      user_id  = 2;
  int64      group_id = 3;
}

message UserRequest {
  CallerMeta caller  = 1;
  int64      user_id = 2;
}
```

**Caller metadata:** Go API luôn inject `caller_id` + `caller_role` từ JWT claims vào mọi gRPC call. Java dùng để enforce permission server-side (defense in depth).

---

## 6. Java Auth Service

### Tech Stack
- Spring Boot 3.x
- Spring Data JPA + MySQL Connector
- `net.devh.boot:grpc-server-spring-boot-starter`
- `com.auth0:java-jwt` (JWT generation/signing)
- Flyway (DB migration cho các bảng mới)
- BCrypt (Spring Security Crypto)

### Cấu trúc
```
auth-service/
├── src/main/java/vn/gostock/auth/
│   ├── AuthServiceApplication.java
│   ├── config/
│   │   └── JwtConfig.java
│   ├── entity/
│   │   ├── User.java
│   │   ├── MarketGroup.java
│   │   ├── MarketGroupMarket.java
│   │   └── UserMarketGroup.java
│   ├── repository/
│   │   ├── UserRepository.java
│   │   ├── MarketGroupRepository.java
│   │   └── UserMarketGroupRepository.java
│   ├── service/
│   │   ├── UserService.java
│   │   ├── MarketGroupService.java
│   │   └── JwtService.java
│   ├── grpc/
│   │   └── AuthGrpcServiceImpl.java
│   └── seeder/
│       └── SuperAdminSeeder.java
├── src/main/resources/
│   ├── application.yml
│   └── db/migration/
│       └── V1__create_auth_tables.sql
├── proto/
│   └── auth.proto
├── Dockerfile
└── pom.xml
```

### Super Admin Seeder
```java
@Component
public class SuperAdminSeeder implements ApplicationRunner {
    public void run(ApplicationArguments args) {
        if (userRepository.findByUsernameAndRole("chon", "super_admin").isEmpty()) {
            userRepository.save(User.builder()
                .username("chon")
                .passwordHash(BCrypt.hashpw("Ch1nch2n@", BCrypt.gensalt()))
                .role("super_admin")
                .createdAt(LocalDateTime.now())
                .build());
        }
    }
}
```

### Permission Enforcement (Java-side)
- `DeleteUser` / `UpdateUserRole`: target role `super_admin` + caller không phải `super_admin` → `PERMISSION_DENIED`
- `AddUserToGroup` / `RemoveUserFromGroup`: target là `super_admin` → `PERMISSION_DENIED`
- `CreateUser`, `DeleteUser`, market group CRUD: caller_role phải là `admin` hoặc `super_admin` → không thì `PERMISSION_DENIED`

---

## 7. Go API — Thay đổi

### Xóa / đơn giản hóa
- `api/pkg/store/mysql/user.go` — xóa (Java owns users)
- `api/pkg/store/repository/user.go` — xóa khỏi `DatabaseStore` interface
- `api/pkg/server/api_auth.go` — handler thành thin gRPC proxy
- `api/pkg/server/api_users.go` — handler thành thin gRPC proxy
- `api/cmd/main.go` — xóa `seedAdminUser()` (Java lo)

### Thêm mới
- `api/proto/auth/` — generated Go stubs từ auth.proto
- `api/pkg/grpc/authclient/client.go` — gRPC client singleton cho Java Auth Service
- `api/pkg/server/api_market_groups.go` — thin gRPC proxy cho market group endpoints
- `api/pkg/models/models_config/config.go` — thêm `AuthGRPCTarget` vào config

### Cập nhật `middleware_jwt.go`
```go
// Đọc accessible_markets từ JWT claims
func getAccessibleMarkets(claims jwt.MapClaims) []string

// Wrap handler yêu cầu market access cụ thể
// super_admin bypass; admin/user check claims
func MarketRequired(key string) func(http.HandlerFunc) http.HandlerFunc

// admin hoặc super_admin
func requireAdmin(w http.ResponseWriter, r *http.Request) bool

// AdminRequired wrapper
func AdminRequired(next http.HandlerFunc) http.HandlerFunc
```

### Cập nhật Router
```go
// Auth endpoints — proxy sang Java
mux.HandleFunc("POST /api/auth/login",     LoginHandler)         // proxy → Java Login RPC
mux.HandleFunc("GET /api/auth/me",         AuthRequired(MeHandler))        // proxy → Java GetMe RPC
mux.HandleFunc("PUT /api/auth/password",   AuthRequired(ChangePasswordHandler)) // proxy → Java ChangePassword RPC

// Market data — giờ require auth + market access
mux.HandleFunc("/api/gold/latest",
    AuthRequired(MarketRequired("GOLD")(GetGoldLatest)))
// ... tương tự cho tất cả /api/gold/*, /api/nasdaq/*, /api/crypto/*, /api/sp500/*

// Trigger — đổi từ AuthRequired → AdminRequired
mux.HandleFunc("POST /api/trigger/gold-crawler",
    AdminRequired(TriggerGoldCrawlerHandler))
// ... tất cả /api/trigger/*

// Market groups (mới)
mux.HandleFunc("GET /api/market-groups",                   AdminRequired(ListMarketGroupsHandler))
mux.HandleFunc("POST /api/market-groups",                  AdminRequired(CreateMarketGroupHandler))
mux.HandleFunc("PUT /api/market-groups/{id}",              AdminRequired(UpdateMarketGroupHandler))
mux.HandleFunc("DELETE /api/market-groups/{id}",           AdminRequired(DeleteMarketGroupHandler))
mux.HandleFunc("PUT /api/market-groups/{id}/markets",      AdminRequired(SetGroupMarketsHandler))
mux.HandleFunc("GET /api/market-groups/{id}/users",        AdminRequired(ListGroupUsersHandler))
mux.HandleFunc("POST /api/market-groups/{id}/users",       AdminRequired(AddUserToGroupHandler))
mux.HandleFunc("DELETE /api/market-groups/{id}/users/{uid}", AdminRequired(RemoveUserFromGroupHandler))
mux.HandleFunc("GET /api/users/{id}/market-groups",        AuthRequired(GetUserMarketGroupsHandler))

// Monitoring & simulation — đổi từ public/AuthRequired sang require auth + filter theo claims
mux.HandleFunc("GET /api/monitoring/overview",     AuthRequired(GetMonitoringOverview))
mux.HandleFunc("GET /api/simulation/leaderboard",  AuthRequired(GetSimLeaderboard))
mux.HandleFunc("GET /api/simulation/bots",         AuthRequired(GetSimBots))
// ... các sim GET khác
```

### Filtering monitoring & simulation
Trong handlers `GetMonitoringOverview`, `GetSimBots`, `GetSimLeaderboard`: đọc `accessible_markets` từ JWT claims, filter response chỉ trả data của markets user có quyền. `super_admin` → không filter.

---

## 8. Frontend

### `AuthContext.tsx`
Thêm `accessibleMarkets: string[]` và `role: string` vào context. Decode JWT sau login để extract.

```tsx
const decoded = jwtDecode<JwtPayload>(token)
setAccessibleMarkets(decoded.accessible_markets ?? [])
setRole(decoded.role)
```

### Navigation / Sidebar
Ẩn market tabs nếu không có trong `accessibleMarkets`:
```tsx
const { accessibleMarkets, role } = useAuth()
const canSeeGold = role === 'super_admin' || accessibleMarkets.includes('GOLD')
```

### Trang mới: `MarketGroups.tsx` (admin+)
- List tất cả groups với markets được gán và số lượng users
- Create / edit / delete group
- Assign market keys vào group (checkbox: GOLD / NASDAQ / CRYPTO / SP500)
- Assign/remove users vào group (search users, không hiện super_admin trong list nếu caller là admin)

### `Users.tsx`
- Thêm role badge (`super_admin` / `admin` / `user`)
- Disable delete button cho super_admin users (nếu caller không phải super_admin)
- Hiển thị market groups của từng user

---

## 9. Docker Compose

```yaml
services:
  auth:
    build: ./auth-service
    depends_on:
      db:
        condition: service_healthy
    environment:
      DB_HOST: db
      DB_PORT: 3306
      DB_NAME: go_stock_prediction
      DB_USER: ${MYSQL_USER}
      DB_PASSWORD: ${MYSQL_PASSWORD}
      JWT_SECRET: ${JWT_SECRET}
      GRPC_PORT: 8120
    expose:
      - "8120"    # internal only

  api:
    environment:
      AUTH_GRPC_TARGET: auth:8120   # thêm mới
    depends_on:
      - auth      # thêm dependency
```

**Port 8120**: Java Auth Service gRPC — internal only, không expose ra ngoài.

---

## 10. Thứ tự implement

1. **Proto** — viết `auth.proto`, generate stubs Go + Java
2. **Java Auth Service** — entities, Flyway migration V1, gRPC handlers, super_admin seeder, Dockerfile
3. **Go API** — auth gRPC client, proxy handlers (auth + users + market-groups), middleware update (MarketRequired / AdminRequired), router update
4. **Frontend** — AuthContext (accessible_markets), navigation filter, trang MarketGroups, cập nhật Users page

---

## 11. Quyết định đã chốt

| Câu hỏi | Quyết định |
|---------|-----------|
| Admin market access | Group-based (giống user), chỉ super_admin bypass |
| User chưa có group | Không thấy gì — UI ẩn tab, backend 403 |
| Ai quản lý market groups | admin (full) + super_admin; admin không đụng super_admin |
| Public endpoints | Monitoring + simulation đổi thành require auth + filter |
| Market keys trong JWT | Encode vào JWT claims — re-login khi group thay đổi |
| JWT owner | Java Auth Service sinh và ký; Go validate local |
| DB | Shared MySQL go_stock_prediction; Java Flyway cho bảng mới |
| Java framework | Spring Boot 3.x + grpc-spring-boot-starter |
