# Day 5 — Observability & exam prep
## Lab 5.1 — Self-Healing App
Duration: ~45 min | CKAD domain: Application Observability and Maintenance (15%)
Configure HTTP liveness probe
Configure file-based readiness probe
Optional: startup probe for slow-start container

### ✅ Đã thực hiện (2026-07-24) — probes áp thẳng trên deploy THẬT

Probe đã phủ sẵn 6 service (day trước): **HTTP liveness/readiness** (api-svc
`/health/*` :8118; auth-svc Actuator `/actuator/health/{liveness,readiness}` :9464), **native
gRPC probe** (service-mgt `grpc:8121`), **tcpSocket** (prediction-svc :8119). Bài này bổ sung
phần còn thiếu: **startupProbe cho container slow-start**.

- **auth-svc** (Java, JVM + Flyway migrate) — ĐÃ có `startupProbe` từ trước
  (`failureThreshold:30 periodSeconds:5` ≈ 150s cho JVM boot).
- **prediction-svc** (Python, torch import + ML init + DB connect ~30–120s) — **THÊM mới**
  (`prediction-svc/templates/deployment.yaml`): startupProbe gate liveness/readiness
  nên liveness KHÔNG giết pod oan lúc khởi động chậm; bỏ được `initialDelaySeconds` dài trên
  readiness/liveness.

```yaml
startupProbe:
  tcpSocket: { port: 8119 }
  periodSeconds: 5
  failureThreshold: 30        # tối đa ~150s để app mở cổng gRPC
```

```bash
helm upgrade prediction-svc deploy/helm/prediction-svc -n stock    # chart prediction-svc (revision riêng)
p=$(kubectl get pod -n stock -l app=prediction-svc -o jsonpath='{.items[0].metadata.name}')
kubectl get pod $p -n stock -o jsonpath='{.spec.containers[?(@.name=="prediction-svc")].startupProbe}'
#   {"failureThreshold":30,"periodSeconds":5,"successThreshold":1,"tcpSocket":{"port":8119},"timeoutSeconds":1}
```
Điểm chốt: **thứ tự probe** = startup → (readiness ∥ liveness). Trong lúc startupProbe chưa
pass: readiness/liveness BỊ HOÃN → container slow-start không bị liveness restart loop. Khi
startup pass 1 lần → chuyển sang readiness/liveness bình thường. `2/2` Available, 0 restart.

> Ghi chú: liveness/readiness "file-based" của đề (`cat /tmp/healthy`) là kiểu `exec` —
> stack thật dùng httpGet/tcpSocket/grpc (đúng bản chất service) nên không minh hoạ exec-file;
> cơ chế self-heal (probe fail → restart/notReady) là như nhau.

## Lab 5.2 — CLI Observability
Duration: ~45 min | CKAD domain: Application Observability and Maintenance (15%)
Use kubectl logs with -c and --previous
Read Events from describe and get events
Use kubectl top for resource usage

### ✅ Đã thực hiện (2026-07-24) — trên pod thật (ambassador multi-container)

```bash
# --- logs -c (chọn container trong pod nhiều container) ---
POD=$(kubectl get pod -n stock -l app=api-svc -o jsonpath='{.items[0].metadata.name}')
kubectl get pod $POD -n stock -o jsonpath='{range .spec.containers[*]}{.name} {end}'
#   api-svc nginx log-sidecar          ← 3 container/pod (ambassador)
kubectl logs $POD -n stock -c log-sidecar --tail=2
#   ...HTTP server listening on 0.0.0.0:8118     ← phải chỉ -c khi pod >1 container

# --- logs --previous (log của INSTANCE TRƯỚC sau khi container restart/crash) ---
kubectl logs $POD -n stock -c <container> --previous
#   (đọc log của lần chạy trước lần restart hiện tại — điều tra CrashLoopBackOff,
#    vd bug nginx CAP_CHOWN ở Lab 3.2 bắt được bằng logs --previous)

# --- Events (chẩn đoán schedule/pull/probe) ---
kubectl get events -n stock --sort-by=.lastTimestamp | tail -4
#   Normal SawCompletedJob   cronjob/pipeline-sp500  Saw completed job ... Complete
#   Normal WaitForFirstConsumer  pvc/backup-data  waiting for first consumer ...
kubectl describe pod $POD -n stock          # section Events cuối = timeline của riêng pod đó

# --- top (metrics-server) ---
kubectl top pods  -n stock -l app=prediction-svc
#   prediction-svc-...-hs826   2m   131Mi     ← CPU cores / RAM thực (cần metrics-server)
kubectl top nodes
#   ckad-control-plane  164m  1%  1535Mi 10%   ...
```
Điểm chốt:
- Pod nhiều container ⇒ `kubectl logs` **bắt buộc `-c <container>`** (thiếu → lỗi/chọn mặc định).
  `-f` follow, `--tail=N`, `--previous` (instance trước restart), `--since=10m`.
- **Events là nguồn chẩn đoán #1** cho ImagePullBackOff / FailedScheduling / probe fail /
  quota reject. `describe` = Events của 1 object; `get events` = toàn ns (sort theo thời gian).
- `kubectl top` cần **metrics-server** (kind bật `--kubelet-insecure-tls`); số là **usage thật**
  (khác `resources.requests` là "đặt trước").

## Lab 5.3 — Broken YAML Triage
Duration: ~45 min | CKAD domain: Application Observability and Maintenance (15%)
Fix selector mismatch in Deployment
Fix Service targetPort mismatch
Fix invalid image name

### ✅ Đã thực hiện (2026-07-24) — `deploy/k8s/ckad-labs/day_5/broken.yaml`

Manifest cố tình có 3 lỗi kinh điển; chữa theo trình tự (mỗi lỗi lộ ra ở tầng khác nhau):

```bash
# BUG 1 — selector không khớp template labels → apply BỊ TỪ CHỐI ngay (validation):
kubectl apply -f deploy/k8s/ckad-labs/day_5/broken.yaml
#   service/triage created
#   The Deployment "triage" is invalid: spec.template.metadata.labels: Invalid value:
#   {"app":"webapp"}: `selector` does not match template `labels`
# → sửa selector.matchLabels = app:webapp (khớp template) → apply lại → Deployment created

# BUG 3 — image sai chính tả (nginx:latst) → Pod ImagePullBackOff/ErrImagePull:
kubectl get pod -n stock -l app=webapp        # STATUS ErrImagePull → ImagePullBackOff
kubectl describe pod -l app=webapp -n stock   # Events: Failed to pull image "nginx:latst"
kubectl set image deploy/triage web=nginx:1.27-alpine -n stock   # → rolled out

# BUG 2 — Service targetPort 8080 nhưng container cổng 80 → có endpoint nhưng curl fail:
kubectl get endpoints triage -n stock
#   triage   10.244.2.63:8080     ← endpoint trỏ SAI cổng (8080)
kubectl patch svc triage -n stock -p '{"spec":{"ports":[{"port":80,"targetPort":80}]}}'
kubectl get endpoints triage -n stock
#   triage   10.244.2.63:80       ← khớp cổng container → service reachable
```
Điểm chốt (3 tầng lỗi khác nhau — kỹ năng triage):
- **selector≠template** = lỗi **admission** (apply fail ngay, đọc message). Selector Deployment
  IMMUTABLE nên sửa = phải khớp template từ đầu.
- **image sai** = lỗi **runtime** (Pod ImagePullBackOff) — đọc `describe pod` Events.
- **targetPort sai** = lỗi **kết nối** (Pod Running, Endpoints CÓ nhưng sai cổng) — `get endpoints`
  soi cổng; đây là lỗi "ngấm ngầm" nhất vì mọi thứ trông Running.

## Lab 5.4 — Helm Deploy & Rollback
Duration: ~45 min | CKAD domain: Application Deployment (20%)
Install Helm chart with value overrides
Upgrade release and rollback to previous revision

### ✅ Đã thực hiện (2026-07-24) — trên MỘT chart per-service THẬT (vd `api-svc`, helm v3.16.2 local)

Nay mỗi service = 1 chart độc lập → upgrade/rollback vòng đời riêng, `helm history` **per-chart**.
Day 2/3 tạo ra **lịch sử revision thật** cho từng chart (cùng cơ chế lab này):
```bash
helm history api-svc -n stock
#   1   Install complete   (cài lần đầu)
#   2   Upgrade complete   (3.2 securityContext hardening)
#   3   Upgrade complete   (blue/green color selector)
#   4   deployed           (--set hpa.enabled=true)

# value override lúc upgrade (--set ghi đè values.yaml của chart đó):
helm upgrade api-svc deploy/helm/api-svc -n stock --set hpa.enabled=true
helm upgrade api-svc deploy/helm/api-svc -n stock --set imageTag=v2   # bump tag image chart này

# ROLLBACK về revision trước (chỉ ảnh hưởng chart api-svc):
helm rollback api-svc 2 -n stock
#   Rollback was a success! Happy Helming!
helm history api-svc -n stock | tail -1
#   5   Rollback to 2      ← rollback tạo REVISION MỚI (không xoá lịch sử)
```

**⚠️ Bẫy quan trọng (bài học):** `helm rollback api-svc 2` = **revert TOÀN BỘ state của CHART đó về
đúng snapshot rev2**, KHÔNG phải "undo lệnh cuối". Rev2 chưa có blue/green color selector (thêm ở
rev3) → rollback về 2 **hoàn nguyên selector của chart api-svc**:
```bash
kubectl get svc api-svc -n stock -o jsonpath='{.spec.selector}'   # sau rollback→2: mất color
helm upgrade api-svc deploy/helm/api-svc -n stock    # restore từ chart (có đủ lại)
```
> Ưu điểm per-service charts: rollback `api-svc` KHÔNG đụng chart khác (bootstrap RBAC/quota,
> db, prediction-svc...) — blast radius gói gọn trong 1 chart, khác hẳn umbrella cũ (1 rollback
> revert cả stack). Nếu cần lùi nhiều service → rollback từng chart.

Điểm chốt:
- `helm upgrade --set k=v` (hoặc `-f <chart>-secret.yaml`) = override value; `--reuse-values` giữ
  set cũ (⚠️ dễ nhầm — dùng `--reset-values` hoặc upgrade sạch từ chart khi muốn chuẩn).
- **Mỗi upgrade/rollback = 1 revision** (`helm history <chart>`); `helm rollback <chart> <rev>` nhảy
  về snapshot đó và **tạo revision mới** (audit trail liền mạch).
- **Rollback là toàn-cục TRONG CHART, không chọn lọc**: resource thêm ở revision sau bị XOÁ khi lùi
  về revision trước — nhưng chỉ trong phạm vi chart đó (không chạm chart anh em).
- `helm rollback` an toàn hơn `kubectl rollout undo` ở chỗ nó revert CẢ manifest set của chart
  (không lệch last-applied) — nhưng chính vì "cả set" nên phải hiểu nó xoá gì.