# Day 3 — Configuration & security

> Cách tiếp cận (như day 1/2): **áp thẳng lên deploy THẬT** trong Helm chart `stock`
> (subchart per-service + library `common`), KHÔNG tạo pod demo cô lập. Mọi thay đổi
> qua `helm upgrade stock deploy/helm/stock -n stock` rồi verify trên pod đang chạy.
> Helm CLI local: `/home/chronical/.local/bin/helm` (v3.16.2).

## Lab 3.1 — ConfigMap & Secret Injection
Duration: ~45 min | CKAD domain: Application Environment, Configuration & Security (25%)
Create Secret from file and ConfigMap from literal
Inject Secret as env var and ConfigMap as mounted volume in one Pod

### ✅ Đã hiện thực sẵn trong stack thật (chỉ document)

Cả HAI nửa của bài đã nằm trong mọi backend pod — không cần thêm:

- **Secret → env var**: mỗi service có `secret.yaml` (`charts/<svc>/templates/secret.yaml`)
  nạp qua `envFrom.secretRef` trong deployment (vd `api-secret`, `prediction-secret`).
- **ConfigMap → mounted volume**: `nginx.conf` của ambassador là ConfigMap
  (`<svc>-nginx`) **mount làm volume** vào container nginx:
  ```yaml
  volumeMounts: [{ name: nginx-conf, mountPath: /etc/nginx/nginx.conf, subPath: nginx.conf }]
  volumes:      [{ name: nginx-conf, configMap: { name: prediction-svc-nginx } }]
  ```
- **ConfigMap → env**: `configMapRef` (vd `api-config`, `prediction-config`).

Verify nhanh (pod thật) — dùng `describe | grep` (dễ nhớ hơn `-o jsonpath`):
```bash
kubectl get cm,secret -n stock | grep -E 'nginx|config|secret'

# envFrom (ConfigMap + Secret nạp làm env):
kubectl describe deploy prediction-svc -n stock | grep -A2 'Environment Variables from'
#   prediction-config  ConfigMap  Optional: false
#   prediction-secret  Secret     Optional: false

# ConfigMap → mounted volume (nginx.conf):
kubectl describe deploy prediction-svc -n stock | grep 'nginx.conf'
#   /etc/nginx/nginx.conf from nginx-conf (rw,path="nginx.conf")
```

## Lab 3.2 — Security Context Lockdown
Duration: ~45 min | CKAD domain: Application Environment, Configuration & Security (25%)
Run Pod as non-root with read-only root filesystem
Drop all capabilities; disable privilege escalation

### ✅ Đã thực hiện (2026-07-24) — áp lên CẢ 5 backend (api/auth/prediction/service-mgt/cli)

Hardening gom vào **library chart** `charts/common/templates/_ambassador.tpl` (dùng chung
cho nginx/sidecar/wait-db) + app container inline per-service (vì UID/khả năng readOnly
KHÁC nhau theo image).

**Đã áp (verify trên pod live):**

| Thành phần | allowPrivEsc:false + drop ALL | runAsNonRoot + runAsUser | readOnlyRootFS | seccomp |
|---|:---:|:---:|:---:|:---:|
| pod-level (cả 5) | — | — | — | RuntimeDefault |
| app api-svc | ✅ | ✖ (appuser phi số) | ✖ | ✅ |
| app auth-svc | ✅ | ✖ (image USER root) | ✖ (JVM ghi /tmp) | ✅ |
| app prediction-svc | ✅ | ✅ uid **1000** | ✖ (torch cache) | ✅ |
| app service-mgt | ✅ | ✖ (appuser phi số) | ✅ **+ /tmp emptyDir** | ✅ |
| app cli-svc | ✅ | ✅ uid **10001** | ✖ (ssh host key) | ✅ |
| nginx ambassador (×5) | ✅ drop ALL **+ add CHOWN/SETUID/SETGID** | (root khi cli) | ✖ | ✅ |
| log-sidecar (×5) | ✅ | — | ✅ (chỉ `tail -f`) | ✅ |
| init wait-db (×5) | ✅ | — | — | ✅ |

**Cơ sở quyết định (UID lấy từ Dockerfile — KHÔNG đoán):**
```
api-svc      adduser -S appuser        → UID hệ thống PHI SỐ  → runAsNonRoot fail (kubelet không verify được) → chỉ drop caps
auth-svc     (không USER)              → chạy ROOT            → không runAsNonRoot
prediction   useradd -u 1000 appuser   → UID 1000 (số)        → runAsNonRoot:true + runAsUser:1000 ✅
cli-svc      adduser -u 10001 cli      → UID 10001 (số)       → runAsNonRoot:true ✅
```
> Bài học: `runAsNonRoot: true` với image mà USER là **tên** (alpine `adduser -S`, không
> `-u <số>`) sẽ **fail admission** ("non-numeric user, cannot verify"). Phải có `runAsUser`
> số. → chỉ enforce ở prediction/cli; api/service-mgt/auth chỉ drop caps + no-priv-esc.

**⚠️ Bẫy đã gặp & fix (nginx CAP_CHOWN):** lần `helm upgrade` đầu, MỌI pod mới `2/3
CrashLoopBackOff` — chỉ container **nginx** chết:
```
[emerg] 1#1: chown("/var/cache/nginx/client_temp", 101) failed (1: Operation not permitted)
```
Official nginx image, entrypoint (root) `chown /var/cache/nginx/*` sang user nginx (uid 101)
lúc start → `drop: [ALL]` mất `CAP_CHOWN` → nginx abort. App + sidecar VẪN Ready (2/3 =
chỉ nginx hỏng) ⇒ hardening app/sidecar chuẩn. **Fix**: nginx giữ drop-ALL nhưng `add`
lại đúng 3 cap official image cần: `CHOWN` (entrypoint chown cache), `SETUID`/`SETGID`
(master hạ quyền worker). Listen cổng cao (>1024) nên KHÔNG cần `NET_BIND_SERVICE`.

```bash
helm upgrade stock deploy/helm/stock -n stock          # rev6 (sau fix)
# rollout zero-downtime (maxUnavailable:0 giữ pod cũ tới khi pod mới Ready)
```

Verify pod live — **`kubectl describe pod` KHÔNG show securityContext của container**
(chỉ có dòng `SeccompProfile: RuntimeDefault`). Dùng `get -o yaml | grep` theo TÊN field
(dễ nhớ hơn jsonpath, và thấy được cả cấu trúc):
```bash
p=$(kubectl get pod -n stock -l app=prediction-svc -o name | head -1)
kubectl get $p -n stock -o yaml | grep -E \
  'securityContext:|fsGroup:|runAsNonRoot:|runAsUser:|allowPrivilegeEscalation:|readOnlyRootFilesystem:|drop:|add:|- ALL|- CHOWN|- SETUID|- SETGID'
```
```
  securityContext:               # pod-level
    fsGroup: 2000
    seccompProfile: RuntimeDefault  (dòng riêng — không match ở grep trên, xem describe)
  - securityContext:             # app prediction-svc
      allowPrivilegeEscalation: false
      capabilities:
        drop:
        - ALL
      runAsNonRoot: true
      runAsUser: 1000
  - securityContext:             # nginx — drop ALL rồi add lại 3 cap official image cần
      allowPrivilegeEscalation: false
      capabilities:
        add:
        - CHOWN
        - SETUID
        - SETGID
        drop:
        - ALL
  - securityContext:             # log-sidecar — chỉ tail -f nên readOnlyRootFilesystem
      allowPrivilegeEscalation: false
      capabilities:
        drop:
        - ALL
      readOnlyRootFilesystem: true
```
Đọc grep: app **drop ALL + runAsNonRoot + runAsUser 1000**; nginx **add CHOWN/SETUID/SETGID**;
sidecar **readOnlyRootFilesystem**; pod-level **fsGroup 2000 + seccomp RuntimeDefault**.
Kết quả: cả 5 backend `2/2` Available, mọi pod `3/3` Running, **0 restart** — hardening không
vỡ runtime.

## Lab 3.3 — ServiceAccount & RBAC
Duration: ~60 min | CKAD domain: Application Environment, Configuration & Security (25%)
Create ServiceAccount, Role, RoleBinding
Pod uses SA token to list Pods in namespace via API

### ✅ Đã thực hiện (2026-07-24) — `deploy/helm/stock/templates/serviceaccounts.yaml`

**(A) Per-service SA + automountServiceAccountToken:false (hardening thật):** 5 backend
trước chạy SA `default` với token k8s API **tự động mount** vào pod dù KHÔNG service nào
gọi API (chúng chỉ nói chuyện qua Service DNS + gRPC). → mỗi service 1 SA riêng, `automount
=false` → không còn projected token. `serviceAccountName: <svc>` wire vào từng deployment.

**(B) pod-reader Role + RoleBinding (artifact RBAC):** SA `pod-reader` + Role chỉ
`get/list/watch` `pods/services/endpoints` (đọc, scope ns stock) + RoleBinding.

```bash
helm upgrade stock deploy/helm/stock -n stock          # rev8

# ── (A) Token KHÔNG còn mount vào pod (automount:false) ────────────────────────
p=$(kubectl get pod -n stock -l app=prediction-svc -o name | head -1)
kubectl describe $p -n stock | grep -iE 'Service Account:|kube-api-access'
#   Service Account:  prediction-svc          ← CHỈ dòng này, KHÔNG có mount kube-api-access-*
#                                                (token không được projected vào pod)
kubectl get sa prediction-svc -n stock -o yaml | grep automount
#   automountServiceAccountToken: false

# ── (B) RBAC — oracle (can-i) rồi DEMO THẬT (impersonation --as) ───────────────
# Oracle: hỏi API-server "SA này làm được gì":
kubectl auth can-i list pods --as=system:serviceaccount:stock:pod-reader -n stock   # yes
kubectl auth can-i --list        --as=system:serviceaccount:stock:pod-reader -n stock
#   Resources   Non-Resource URLs   Resource Names   Verbs
#   pods        []                  []               [get list watch]
#   services    []                  []               [get list watch]
#   endpoints   []                  []               [get list watch]

# Demo THẬT — chạy lệnh AS the SA (không chỉ boolean oracle):
kubectl get pods --as=system:serviceaccount:stock:pod-reader -n stock
#   NAME                 READY  STATUS   ...        ← LIỆT KÊ ĐƯỢC (real pod list)

kubectl get pods --as=system:serviceaccount:stock:auth-svc   -n stock
#   Error from server (Forbidden): pods is forbidden: User
#   "system:serviceaccount:stock:auth-svc" cannot list resource "pods" in API group ""
#   in the namespace "stock"                        ← SA khác KHÔNG bind Role → chặn

kubectl delete pod $(basename $p) --as=system:serviceaccount:stock:pod-reader -n stock --dry-run=server
#   Error from server (Forbidden): pods "..." is forbidden: User
#   "system:serviceaccount:stock:pod-reader" cannot delete resource "pods" in API group ""
#   in the namespace "stock"                        ← Role chỉ get/list/watch → xóa bị chặn
```
Điểm chốt:
- `automountServiceAccountToken:false` đặt ở **SA object** → mọi pod dùng SA đó không mount
  token; verify bằng `describe pod | grep` KHÔNG thấy `kube-api-access-*`.
- **Impersonation (`--as`) chứng minh RBAC enforce THẬT ở API-server**, không chỉ boolean
  `can-i`: cùng SA `pod-reader` **đọc OK** (`get pods` liệt kê) nhưng **xóa bị Forbidden**
  (Role chỉ get/list/watch); SA khác (`auth-svc`, không bind Role) **list cũng Forbidden**
  → least-privilege quan sát trực tiếp. `--dry-run=server` cho lệnh delete chạy đủ admission
  RBAC mà không thực xóa.
- gateway-svc GIỮ SA + Role riêng (`charts/gateway-svc/templates/rbac.yaml`) vì nó THỰC SỰ
  gọi API (blue/green controller patch Deployment) — không đụng.

## Lab 3.4 — Namespace Quotas
Duration: ~45 min | CKAD domain: Application Environment, Configuration & Security (25%)
Apply ResourceQuota and LimitRange
Observe Pod rejection when quota exceeded

### ✅ Đã thực hiện (2026-07-24) — `deploy/helm/stock/templates/quota.yaml`

Áp **LimitRange + ResourceQuota vào ns `stock` THẬT** (không dùng ns nháp). Vì ns live đang
chạy 26 pod, phải size quota an toàn để KHÔNG chặn rollout/blue-green.

**Sizing (đo tại thời điểm set):** requests 2.25 core / 3.75Gi, limits 13.2 core / 11Gi →
quota ~2.5–3x. **LimitRange BẮT BUỘC đi kèm**: tiêm default request cho pod không khai
resources (init, CronJob pod) → chúng thỏa quota (nếu không có → quota reject).

```
LimitRange stock-defaults:  defaultRequest 50m/64Mi · default 250m/256Mi · max 2/3Gi
ResourceQuota stock-quota:  requests.cpu=6 requests.memory=10Gi limits.cpu=30 limits.memory=24Gi pods=60
```

```bash
helm upgrade stock deploy/helm/stock -n stock          # rev7
kubectl describe quota stock-quota -n stock
#   requests.cpu 2250m/6 · requests.memory 3840Mi/10Gi · limits.cpu 13200m/30 · pods 26/60   (đều dưới trần)
```

**Demo REJECT (CKAD objective) — `--dry-run=server` chạy admission thật, không tạo pod:**
```bash
# (1) Vượt ResourceQuota: 2 container × request cpu=2 (mỗi cái ≤ max) → tổng 4 > remaining 3.75:
#   Error: pods "quota-test" is forbidden: exceeded quota: stock-quota,
#          requested: requests.cpu=4, used: requests.cpu=2250m, limited: requests.cpu=6
# (2) Vượt LimitRange max: 1 container request cpu=3 (> max 2):
#   Error: pods "max-test" is forbidden: maximum cpu usage per Container is 2, but limit is 3
# (3) Default injection: pod KHÔNG khai resources → LimitRange tự vá:
#   {"limits":{"cpu":"250m","memory":"256Mi"},"requests":{"cpu":"50m","memory":"64Mi"}}
```
Điểm chốt:
- Quota `requests.*`/`limits.*` yêu cầu MỌI pod khai resources → **phải pair LimitRange**
  (default) nếu không CronJob/pod thiếu resources bị reject.
- Reject xảy ra ở **API server admission** (không phải scheduler) → pod không được tạo.
- `--dry-run=server` = kênh demo sạch: chạy đủ admission (quota + limitrange) nhưng không
  persist → không cần dọn.

---

### Tổng kết Day 3 (helm revisions)
```
rev5  3.2 (lần đầu — nginx crashloop do drop CAP_CHOWN)
rev6  3.2 fix (nginx add CHOWN/SETUID/SETGID) → xanh
rev7  3.4 LimitRange + ResourceQuota
rev8  3.3 per-service SA + pod-reader RBAC  (gộp 5.1 startupProbe — xem day_5)
```
Rollback bất kỳ bước: `helm rollback stock <rev> -n stock` (xem Lab 5.4).
Tắt từng feature: `--set global.quota.enabled=false` / `--set global.rbac.enabled=false`.
