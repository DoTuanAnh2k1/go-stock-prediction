# Service Management (Service Registry / Discovery) — Design

Ngày: 2026-06-21
Trạng thái: Đã duyệt (brainstorming), chuẩn bị triển khai theo phase.

## 1. Mục tiêu

Xây một **Service Management / Registry** tập trung để các service tìm nhau **động** thay cho endpoint hardcode hiện tại (`AUTH_GRPC_TARGET`, `GRPC_TARGET` trong `.env`; backend URL trong `gateway-svc/config.yaml`).

Mục đích cốt lõi: khi về sau triển khai thêm service mới (hoặc đổi port), **service gọi đến không cần quan tâm ip/port** — chỉ hỏi mgt-svc và nhận lại địa chỉ đang sống để gửi request.

Ba chức năng:
1. **Register** — service vừa up tự đăng ký (tên, ip, port, metadata).
2. **Discovery** — service A hỏi "B ở đâu?" → registry trả endpoint của instance đang UP.
3. **Heartbeat (push/lease-TTL)** — service tự gia hạn lease định kỳ; quá TTL → registry đánh `DOWN`, loại khỏi discovery.

## 2. Quyết định đã chốt (từ brainstorming)

| Vấn đề | Quyết định |
|---|---|
| Phạm vi | Tích hợp **tất cả** service (Go x2, Java, Python, Rust), làm theo phase |
| Ngôn ngữ registry | **Go** (trùng stack api-svc/cli-svc) |
| Giao thức control-plane | **gRPC** |
| Mô hình discovery | **Client-side** — registry chỉ trả địa chỉ, A gọi B trực tiếp (không proxy traffic) |
| Heartbeat | **Push / lease-TTL** (giống Eureka) |
| Lưu trữ | **Write-through cache**: ghi DB trước → OK mới ghi cache; đọc chỉ từ cache. Postgres = nguồn sự thật, cache in-memory = tầng đọc |
| Heartbeat ghi DB? | **KHÔNG** — heartbeat chỉ chạm cache (cập nhật TTL); goroutine định kỳ flush `last_seen` xuống DB |
| Discovery client | **Lookup + cache TTL ngắn** (vd 5s), refresh khi hết TTL hoặc khi gọi B fail |
| Bật/tắt | **Env flag toàn cục** `SERVICE_MGT_ENABLED`; tắt → mọi service chạy y như hiện tại (static endpoint) |

## 3. Kiến trúc

`service-mgt/` — service Go mới, **gRPC server `:8121`** (internal only, không expose ra ngoài, không qua gateway).

**Bootstrap:** thêm `REGISTRY_GRPC_TARGET=service-mgt:8121` vào `.env` — endpoint hardcode *duy nhất* còn lại (hạt giống bắt buộc để mọi service biết registry ở đâu).

### 3.1 Proto — `registry.proto`, `service Registry`

| RPC | Request | Response | Việc |
|---|---|---|---|
| `Register` | service_name, instance_id, address, port, metadata, ttl_seconds | instance_id, lease_ttl | Đăng ký khi boot |
| `Heartbeat` | instance_id | ok; `NOT_FOUND` nếu lease mất | Gia hạn lease; NOT_FOUND → client tự re-register |
| `Deregister` | instance_id | Empty | Tắt êm (graceful shutdown) |
| `Discover` | service_name | repeated Instance{address, port, metadata} | Trả các instance đang UP |
| `ListServices` | Empty | toàn bộ danh bạ | Debug/dashboard |

Proto đặt tại `service-mgt/proto/registry/registry.proto`. Sinh stub cho Go (registry + Go client), Java, Python, Rust.

### 3.2 Bên trong service-mgt

- **gRPC handlers** → gọi **Registry core**.
- **Registry core** = cache in-memory: `map[serviceName]map[instanceID]*Instance{addr, port, meta, expireAt, status}`. Lock phù hợp (RWMutex).
- **Store (Postgres/GORM)** — write-through: Register / Deregister / đổi status (UP↔DOWN) ghi DB trước, OK mới ghi cache. Heartbeat KHÔNG ghi DB.
- **Flusher goroutine** — định kỳ (vd 30s) flush `last_seen` của các instance đang sống xuống DB.
- **Reaper goroutine** — quét cache mỗi ~1s: lease quá `expireAt` → status=DOWN (write-through DB), loại khỏi tập healthy; sau grace period (vd 60s) xóa hẳn khỏi cache + DB.
- **Logger** zerolog 1 dòng/màu, message tiếng Anh, ANSI forced — nhất quán repo.
- **Khởi động:** load các instance từ DB vào cache (warm-start) nhưng đánh dấu chờ heartbeat; instance không heartbeat trong TTL đầu sẽ bị reaper dọn.

### 3.3 DB — bảng mới `service_instances` (khai trong `database.sql`)

```
id            BIGSERIAL PK
service_name  VARCHAR        -- "api-svc", "auth-svc", ...
instance_id   VARCHAR UNIQUE -- service_name + uuid/host
address       VARCHAR        -- ip hoặc docker dns name
port          INT
metadata      JSONB
status        VARCHAR        -- UP / DOWN
ttl_seconds   INT
last_seen     TIMESTAMP
registered_at TIMESTAMP
updated_at    TIMESTAMP
```
Không phải hypertable. Index trên `service_name`, `status`. Timezone ICT-at-rest (theo convention repo).

### 3.4 Client SDK (mỗi ngôn ngữ, mỏng — wrapper quanh proto stub)

- **boot** → `Register`; chạy goroutine/thread **heartbeat** mỗi N giây (N < TTL, vd TTL=30s heartbeat=10s); bắt `NOT_FOUND` → re-register.
- **gọi B** → `Discover(B)` có **cache TTL ngắn (5s)**, refresh khi hết TTL hoặc khi gọi B fail (invalidate + lookup lại).
- **shutdown** → `Deregister` (best-effort).
- **Static fallback:** nếu `SERVICE_MGT_ENABLED=false` HOẶC registry không reach được → dùng env target cũ (`AUTH_GRPC_TARGET`, ...) → hệ thống không sập.

4 client: Go (api-svc, cli-svc), Java (auth-svc), Python (prediction-svc), Rust (gateway-svc → discover api-svc & web-svc).

## 4. Env flag & fallback

- `SERVICE_MGT_ENABLED` (bool, **mặc định `false`**) — đặt trong `.env`, truyền vào mọi service.
  - `false` → service bỏ qua register/discover, dùng endpoint static như hiện tại. Không thay đổi hành vi.
  - `true` → service register lúc boot, discover qua registry (với static fallback khi registry chết).
- `REGISTRY_GRPC_TARGET=service-mgt:8121` — địa chỉ registry.
- Mặc định false để bật dần, không big-bang; CI/dev hiện tại không bị ảnh hưởng tới khi opt-in.

## 5. Rollout theo phase

Mỗi phase: build → test → docker rebuild → verify. Mỗi phase giữ static fallback → an toàn.

- **Phase 1** — Nền tảng: `service-mgt/` (proto, gRPC server, core cache, store write-through, reaper, flusher) + bảng `service_instances` + Go client SDK. Thêm `service-mgt` vào docker-compose (depends_on db). Tích hợp **api-svc**: register lúc boot + discover auth-svc & prediction-svc qua registry, có flag + fallback. Thêm `SERVICE_MGT_ENABLED`, `REGISTRY_GRPC_TARGET` vào `.env`/compose.
- **Phase 2** — auth-svc (Java) register + heartbeat; prediction-svc (Python) register + heartbeat. api-svc discover thật qua registry.
- **Phase 3** — gateway-svc (Rust) discover api-svc & web-svc (thay dần hardcode trong `config.yaml`); cli-svc register/discover. Dọn endpoint static thừa.

## 6. Cập nhật tài liệu

Sau khi triển khai: cập nhật `CLAUDE.md` (thêm service-mgt vào kiến trúc, ports `:8121`, env mới, docker-compose service mới, convention registry/discovery) và `.env` mẫu.

## 7. Ngoài phạm vi (YAGNI)

- Watch/stream discovery (push realtime) — chưa cần, lookup+cache đủ.
- Load balancing nâng cao (round-robin/weighted) — mỗi service hiện 1 instance; client lấy instance đầu/random.
- mTLS/ACL giữa service ↔ registry — sau.
- Web dashboard cho registry — `ListServices` debug là đủ ở v1.
