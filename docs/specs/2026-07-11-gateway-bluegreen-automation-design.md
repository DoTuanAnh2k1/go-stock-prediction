# Gateway-native Blue/Green Automation — Design Spec

- **Ngày:** 2026-07-11
- **Trạng thái:** Approved (design) — chờ writing-plans
- **Phạm vi:** `gateway-svc` (Rust/Axum) + manifest k8s (`deploy/k8s/`)
- **Bối cảnh:** Học CKAD trên cluster kind `ckad`. Lab 2.2 đã làm blue/green thủ công bằng
  `kubectl patch` selector của Service (và dính bẫy patch-vs-apply). Mục tiêu ở đây là
  **tự động hoá** blue/green thẳng trong gateway Rust tự viết, thay cho thao tác tay.

## 1. Mục tiêu & Non-goals

**Mục tiêu:**
- Deploy bản mới = `kubectl set image` lên Deployment **idle** của một app được quản lý.
- Gateway TỰ phát hiện, lật 100% traffic sang bản mới (candidate), đo success-rate trong
  một cửa sổ thời gian, rồi **tự promote** (đồng bộ bản còn lại) hoặc **tự rollback**.
- Áp dụng cho **web-svc** (`/`) và **api-svc** (`/api`).

**Non-goals:**
- KHÔNG làm canary (chia traffic theo %). Đây là blue/green thuần: lật 100%.
- KHÔNG lo HA của gateway (giả định 1 replica giữ state; scale >1 nằm ngoài spec).
- KHÔNG dùng Prometheus/mesh — gateway tự đo từ request nó proxy.
- KHÔNG quản lý migration DB (api-svc không chạy migration; auth-svc mới migrate, không thuộc phạm vi).

## 2. Thuật ngữ (chốt để tránh nhầm)

- **Blue/green:** hai môi trường song song; lật **100%** traffic một phát (atomic). "Cửa sổ
  verify" = sau khi đã 100% ở candidate, quan sát rồi giữ (promote) hoặc lật về (rollback).
- **Canary:** chia traffic theo trọng số tăng dần — KHÔNG làm ở đây.
- **active / idle:** màu đang nhận traffic / màu không nhận. Deploy lên màu idle.
- **candidate:** màu idle vừa được `set image` bản mới, đang chờ được lật + verify.

## 3. Prior art & vì sao tự viết

Cái cần làm đúng bằng chức năng của **Flagger** / **Argo Rollouts** (progressive delivery
controllers): quản 2 ReplicaSet/version, điều khiển traffic (mesh/ingress/Service), chạy
analysis (success-rate/latency), auto-promote/rollback. Khác biệt của bản này:
- Điều khiển traffic **trong chính gateway** (data-plane) thay vì qua mesh/ingress.
- Nguồn metric = **gateway tự đếm** request nó proxy (nó thấy mọi status code) — không cần Prometheus.
- Mục đích **học + đã có gateway custom**; chấp nhận tự xây lại một "mini-Flagger" tối giản.

## 4. Kiến trúc tổng quan

```
             ┌──────────────────────────── gateway-svc ────────────────────────────┐
 request ───▶│ proxy_handler ──route()──▶ BlueGreen{app} ──▶ backend = active color │
             │        └─ sau response: ghi metric (app,color,success|fail)          │
             │                                                                      │
             │ rollout controller (tokio task, kube-rs):                            │
             │   watch Deployment blue/green ──▶ state machine ──▶ patch Deployment │
             │   đọc/ghi active_color + metric qua shared state                     │
             └──────────────────────────────────────────────────────────────────────┘
                       │ k8s API (get/list/watch/patch deployments, get/update cm)
                       ▼
      web-svc-blue/green (Svc:3000)   api-svc-blue/green (Svc:8118)   cm gateway-bluegreen-state
```

## 5. Topology k8s

Mỗi app quản lý tách thành **2 Deployment + 2 Service** theo màu:
- `web-svc-blue` / `web-svc-green` → Service `web-svc-blue` / `web-svc-green` (:3000).
- `api-svc-blue` / `api-svc-green` → Service `api-svc-blue` / `api-svc-green` (:8118).
- Deployment giữ probe/preStop như bản đơn hiện tại (đảm bảo Ready trước khi lật).
- Steady state: cả 2 màu cùng image; một màu `active`.
- Deploy: `kubectl set image deploy/<app>-<idle> <c>=<newimage>`.

**Migration từ setup hiện tại:** hiện `web-svc`/`api-svc` là Deployment đơn + Service `{app}`.
Kế hoạch: thêm manifest blue/green (`deploy/k8s/<app>/bluegreen.yaml`), giữ Service gốc
`web-svc`/`api-svc` **chỉ để tương thích ngược** hoặc bỏ; gateway route thẳng tới Service theo màu.
(Chi tiết bước chuyển để writing-plans quyết; không xoá đột ngột Service đang được dùng.)

## 6. Config (config.yaml — thêm khối `bluegreen`)

```yaml
bluegreen:
  enabled: true
  analysis:
    window_seconds: 60          # cửa sổ quan sát sau khi lật
    success_threshold: 0.99     # tỉ lệ success tối thiểu để promote
    min_requests: 20            # số request tối thiểu để verdict có ý nghĩa
    max_window_multiplier: 3    # gia hạn tối đa khi traffic thấp
    failure_status_from: 500    # status >= giá trị này = failure
  apps:
    - name: web-svc
      prefix: "/"
      namespace: stock
      blue:  { deployment: web-svc-blue,  backend: "http://web-svc-blue:3000" }
      green: { deployment: web-svc-green, backend: "http://web-svc-green:3000" }
      default_active: green
    - name: api-svc
      prefix: "/api"
      namespace: stock
      blue:  { deployment: api-svc-blue,  backend: "http://api-svc-blue:8118" }
      green: { deployment: api-svc-green, backend: "http://api-svc-green:8118" }
      default_active: green
```

Ràng buộc: `apps[].prefix` không được đè lên route tĩnh (`/swagger` block, `/health`).
`/health` route tới backend của **active api-svc**.

## 7. Router + metrics

- Thêm biến thể `RouteAction::BlueGreen { app_id: usize }` bên cạnh `Block`/`Proxy`.
- `PathRouter::from_config` build route blue/green cho mỗi app.prefix (đúng luật longest-prefix
  hiện có: `/api` thắng `/`).
- `proxy_handler`:
  1. `route(path)` → `BlueGreen{app_id}`.
  2. Đọc `active_color[app_id]` từ shared state → chọn `backend`.
  3. Forward qua `ProxyClient` (giữ nguyên logic SSE/retry).
  4. Sau khi có response (hoặc proxy error): ghi metric `(app_id, color, outcome)`.
     - `outcome = fail` nếu status ≥ `failure_status_from` HOẶC `ProxyError` (timeout/connection).
     - 4xx = success (lỗi phía client, không phải lỗi deploy).
- **Shared state** (`Arc<BlueGreenState>`):
  - `active: [AtomicU8; N]` (hoặc `ArcSwap<Vec<Color>>`) — đọc mỗi request, rẻ.
  - `counters: [[AtomicU64; 2]; N]` — `{total, fail}` per (app,color), reset khi bắt đầu Analyzing.

## 8. Rollout state machine (controller task)

Một tokio task/app (hoặc một task đa app) dùng **kube-rs** `watcher` trên Deployment.

```
        Stable
          │  (idle.image != active.image) && (idle Deployment Available/Ready)
          ▼
        Flip:  active_color[app] = candidate ; persist ConfigMap ; reset counters
          ▼
        Analyzing  (100% traffic ở candidate; đọc counters định kỳ ~2s)
          ├─ total ≥ min && fail/total > (1-threshold)  ─────────────▶ RollingBack (sớm)
          ├─ elapsed ≥ window && total ≥ min && success ≥ threshold ─▶ Promoting
          ├─ elapsed ≥ window && total < min:
          │     gia hạn tới window × max_window_multiplier; hết mà vẫn < min
          │     & không có fail ─────────────────────────────────────▶ Promoting (low-confidence, log)
          ▼
        Promoting: kubectl patch Deployment idle → image của candidate ; log ; → Stable
        RollingBack: active_color[app] = màu cũ ; persist ; KHÔNG patch candidate ; alert ; → Stable
```

**Trigger chi tiết — EDGE-TRIGGERED (quan trọng):** controller giữ `last_seen_image` per
(app,màu). Phản ứng với **sự kiện ĐỔI image** trên màu idle (transition), KHÔNG so sánh
tĩnh "images differ" — vì sau khi flip thì active=new/idle=old vẫn "differ" và sẽ kích nhầm.
- Candidate = màu idle vừa có image mới (khác `last_seen`) và ReplicaSet mới của nó Available.
- **Boot:** seed `last_seen` từ image hiện tại của cả 2 màu ⇒ gateway restart KHÔNG tự kích
  rollout giả. Chỉ cho **1 rollout/app** cùng lúc (đang Analyzing thì bỏ qua, log "in progress").

**Guard ngoài luồng:** nếu image của màu **active** đổi (không phải idle) → log cảnh báo
`bluegreen.out_of_band_change`, KHÔNG auto-rollout (tránh lật nhầm).

## 9. State persistence + RBAC

- **State:** ConfigMap `gateway-bluegreen-state` (ns stock), `data: {web-svc: green, api-svc: green}`.
  Đọc lúc boot (thiếu → `default_active`), ghi mỗi lần Flip/RollingBack. Sống sót gateway restart.
- **RBAC:** ServiceAccount `gateway-svc`; Role (ns stock): `deployments` [get,list,watch,patch],
  `configmaps` [get,create,update] (chỉ cần cho state), `replicasets` [get,list,watch] (đọc Ready).
  RoleBinding gắn SA vào Role. Deployment gateway set `serviceAccountName: gateway-svc`.

## 10. Edge cases

- **Gateway restart giữa rollout:** state ConfigMap giữ `active` (đã persist ngay lúc flip) →
  traffic ở đâu giữ đó; rollout dang dở coi như hủy (không promote, không rollback). Boot seed
  `last_seen` nên không tự kích rollout giả. Nếu boot thấy `active.image != idle.image` (rollout
  dở dang, idle chưa được đồng bộ) → log `bluegreen.inconsistent_on_boot`, để nguyên (operator
  re-deploy nếu muốn hoàn tất). An toàn, không loop.
- **Traffic thấp (< min_requests):** gia hạn tới `max_window_multiplier`, sau đó promote
  low-confidence nếu không thấy lỗi (candidate đã phục vụ 100% mà không lỗi).
- **Cả 2 màu cùng image (không có gì để rollout):** Stable, không làm gì.
- **api-svc:** 2 màu chung DB (không migration ở api-svc → an toàn); 2 màu cùng dial
  prediction/auth qua gRPC — OK. Backup scheduler chạy ở cả 2 màu api-svc → cân nhắc để idle
  màu không chạy scheduler? (Ghi chú: backup cron 3AM, xác suất trùng thấp; nếu cần, disable
  scheduler ở màu idle — để writing-plans/hiện thực quyết, không chặn design.)
- **kube-rs mất kết nối API:** watcher tự reconnect (backoff); nếu patch promote lỗi → retry,
  quá số lần → giữ Analyzing/để active ở candidate + log lỗi (không rollback vì candidate đang khỏe).

## 11. Module structure (Rust) — `gateway-svc/src/bluegreen/`

| File | Trách nhiệm |
|---|---|
| `mod.rs` | re-export, khởi tạo (spawn controller nếu `enabled`) |
| `config.rs` | serde structs cho khối `bluegreen` |
| `state.rs` | `BlueGreenState` (active per app + counters), thread-safe; API `active_backend()`, `record(app,color,ok)`, `flip()`, `snapshot_counters()` |
| `controller.rs` | tokio task: watch Deployment, state machine Analyzing/Promote/Rollback |
| `k8s.rs` | kube-rs client: watch deployments, patch image, get/update state ConfigMap |

Sửa: `router/mod.rs` (thêm `BlueGreen`), `routes/proxy.rs` (resolve active + record metric),
`config/mod.rs` (nạp khối bluegreen), `main.rs` (spawn controller). Giữ nguyên `proxy/client.rs`.

## 12. Testing

- **Unit (không cần cluster):**
  - state machine: cho chuỗi metric giả → assert quyết định (promote / rollback-sớm / low-confidence).
  - phân loại outcome: 200/301/404 = success; 500/502/timeout = fail.
  - router: `/api/x` → BlueGreen{api-svc}, `/x` → BlueGreen{web-svc}, `/swagger` → Block.
  - state: flip đổi active_backend; record cập nhật counter đúng màu.
- **Integration (cluster ckad):** apply blue/green manifest cho web-svc; `set image` màu idle;
  quan sát log gateway: detect → flip → analyzing → promote; và một ca `exit`/bad image → rollback.
  Kèm harness `e2e-test.sh` (bắn tải trong lúc rollout, kiểm zero-downtime + verdict).

## 13. Tương lai (ngoài scope)

- Canary (weight tăng dần) — mở rộng state để giữ %.
- HA gateway (state ra ngoài: CRD/lease).
- Metric latency (không chỉ success-rate).
- API `GET /_bluegreen/status` + `POST /_bluegreen/abort` để quan sát/can thiệp tay.
