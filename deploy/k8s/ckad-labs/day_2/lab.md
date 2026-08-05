# Day 2 — Deployments & rollouts

## Lab 2.1 — Rolling Update & Rollback
- Duration: ~45 min | CKAD domain: Application Deployment (20%)
- Perform rolling update from v1 to v2
- Monitor rollout status
- Roll back after simulated bad deployment

### ✅ Đã thực hiện (2026-07-11) — ns `stock`, app demo `web` (nginx)

App demo: `deploy/k8s/ckad-labs/day_2/web.yaml` — Deployment 4 replica nginx:1.25 +
Service, `RollingUpdate maxSurge=1 maxUnavailable=0`, `readinessProbe`, `preStop sleep 5`,
`resources.requests.cpu` (dùng lại cho 2.2/2.3).

**Rolling update v1→v2 + monitor:**
```bash
kubectl set image deploy/web web=nginx:1.27 -n stock
kubectl rollout status deploy/web -n stock      # 1→2→3 of 4 new updated → successfully rolled out
# version: nginx/1.25.5 → nginx/1.27.5
```

**Monitor rollout — 5 góc nhìn (đo thật trên web-svc, roll v2→dev, 3 replica):**

1. **`kubectl rollout status deploy/web-svc`** — BLOCK + stream tiến độ tới khi xong.
   Quan trọng: **exit code = tín hiệu lập trình** (`0`=success, ≠0=fail/timeout — dùng gate CI).
   ```
   deployment "web-svc" successfully rolled out
   exit=0
   ```

2. **Cột `AVAILABLE` không bao giờ tụt** = zero-downtime nhìn thấy được
   (`kubectl get deploy web-svc -n stock -w`):
   ```
   [t=0s]  ready=3 updated=1 available=3
   [t=6s]  ready=3 updated=2 available=3
   [t=12s] ready=3 updated=3 available=3      # updated 1→2→3, available LUÔN = 3
   ```

3. **ReplicaSet** (`kubectl get rs -l app=web-svc`) — RS mới scale up, RS cũ scale
   xuống 0 (giữ để undo). `revisionHistoryLimit=10` = giữ tối đa 10 RS cũ.
   ```
   web-svc-59bfbdcc4b   3   web-svc:dev    rev6   (active)
   web-svc-dfd99f7      0   web-svc:v2     rev5
   web-svc-6d85b88565   0   web-svc:nope   rev4
   ```

4. **Events** (`kubectl describe deploy web-svc`) — cơ chế rolling hiện nguyên hình
   (controller XEN KẼ scale mới lên / cũ xuống):
   ```
   ScalingReplicaSet  Scaled up   web-svc-59bfbdcc4b 0→1→2→3   (RS mới)
   ScalingReplicaSet  Scaled down web-svc-58644dfcbd 3→2→1→0   (RS cũ)
   ```

5. **Conditions** (`kubectl get deploy web-svc -o jsonpath='{.status.conditions[*]}'`):
   ```
   Available=True    reason=MinimumReplicasAvailable
   Progressing=True  reason=NewReplicaSetAvailable      # đã xong
   ```
   - Đang chạy: `Progressing=True reason=ReplicaSetUpdated`.
   - KẸT quá `progressDeadlineSeconds` (mặc định **600s**): `Progressing=False`
     `reason=ProgressDeadlineExceeded` ⇒ rollout coi là **Failed**.
   - ⚠️ Phân biệt: `--timeout` của `rollout status` chỉ là timeout của LỆNH chờ
     (bad deploy báo "timed out" ở 20s là do đây) — KHÁC `progressDeadlineSeconds`
     (mới là ngưỡng để chính Deployment tự đánh dấu Failed).

**Zero-downtime — bài học đo lường (quan trọng):**
- `port-forward svc` để đo là SAI: nó ghim 1 pod, pod bị update giết ⇒ **FAIL=20 (giả)**.
- Phải poll TỪ TRONG cluster (Service ClusterIP load-balance). Harness `e2e-test.sh`
  đã sửa sang chạy poller pod in-cluster.
- `maxUnavailable=0` CẦN nhưng CHƯA ĐỦ: termination race (SIGTERM tới nginx trước khi
  kube-proxy gỡ endpoint) ⇒ **17/6784 fail**. Thêm `lifecycle.preStop: sleep 5` ⇒ **0/7309 = PASS ✅**.
```bash
DUR=50 ./e2e-test.sh zero-downtime web 80 web nginx:1.26 /
#   RESULT ok=7309 fail=0   → PASS
```

**rollout history + ReplicaSet:**
```bash
kubectl rollout history deploy/web -n stock     # mỗi revision + CHANGE-CAUSE
kubectl get rs -n stock -l app=web              # mỗi đổi = RS mới; RS cũ giữ 0 replica để undo
```
- `change-cause` = SNAPSHOT annotation lúc revision tạo ra ⇒ phải
  `kubectl annotate deploy/web kubernetes.io/change-cause="..."` KÈM mỗi lần đổi mới có nghĩa
  (nếu không, history dính cause cũ như đã gặp).

**Bad deploy + rollback:**
```bash
kubectl set image deploy/web web=nginx:9.9.9-nope -n stock
kubectl rollout status deploy/web -n stock --timeout=25s   # error: timed out (KẸT)
kubectl get pods -n stock -l app=web
#   web-...  0/1  ImagePullBackOff   ← pod mới kẹt
#   web-...  1/1  Running (x4)        ← pod cũ vẫn sống → Service KHÔNG sập (maxUnavailable=0)
kubectl rollout undo deploy/web -n stock                   # rolled back → nginx/1.25.5
```
- Lưu ý: `rollout undo` cảnh báo KHÔNG cập nhật `last-applied-configuration` ⇒ GitOps
  nên re-apply manifest cũ thay vì undo.

Kết quả: rolling update thành công, zero-downtime chứng minh (0/7309 với preStop),
bad deploy bị `maxUnavailable=0` chặn không sập service, rollback hồi phục sạch.

### ✅ Áp dụng THẲNG trên app thật `web-svc` (frontend ns stock)

Nâng `deploy/k8s/web-svc/deployment.yaml`: `replicas: 3`, `RollingUpdate maxUnavailable=0`,
`readinessProbe httpGet / :3000`, `preStop sleep 5` (cải tiến thật cho service).

```bash
kubectl apply -f deploy/k8s/web-svc/deployment.yaml          # 3/3 Running
docker tag web-svc:dev web-svc:v2 && kind load docker-image web-svc:v2 --name ckad
kubectl set image deploy/web-svc web-svc=web-svc:v2 -n stock # rolling update dev→v2
DUR=45 ./e2e-test.sh zero-downtime web-svc 3000 web-svc web-svc:v2 /
#   RESULT ok=6566 fail=0 → PASS ✅  (zero-downtime trên frontend thật)

kubectl set image deploy/web-svc web-svc=web-svc:nope -n stock   # BAD deploy
kubectl rollout status deploy/web-svc -n stock --timeout=20s     # timed out (KẸT)
#   web-svc-6d85b88565  0/1  ImagePullBackOff   ← pod mới
#   web-svc-dfd99f7     1/1  Running (x3)        ← pod cũ; curl web-svc:3000 = HTTP 200
kubectl rollout undo deploy/web-svc -n stock                     # rollback → 3/3 web-svc:v2
```

'v2' ở đây là retag same-bits (đủ tạo revision mới để luyện cơ chế) — muốn version
nhìn thấy được thì phải rebuild image có thay đổi. Cơ chế rollout/RS/undo/zero-downtime
đều thật. web-svc GIỮ LẠI để làm tiếp 2.2 (blue/green) và 2.3 (HPA).

## Lab 2.2 — Blue/Green Switch
- Duration: ~45 min | CKAD domain: Application Deployment (20%)
- Run two Deployments (blue and green)
- Route traffic via single Service selector flip

### ✅ Lời giải chính — blue/green KINH ĐIỂN trên app CÓ SẴN `api-svc` (Service chung kèm `color` + flip)

> Bối cảnh: Service chung `api-svc`/`web-svc` trước đây selector chỉ `{app: api-svc}` (KHÔNG color)
> → **hit CẢ blue lẫn green cùng lúc** (4 endpoints) ⇒ KHÔNG phải blue/green thật; blue hỏng vẫn
> nhận 50% traffic. Đã sửa: selector kèm `color=<activeColor>` (driven `global.bluegreen.activeColor`,
> default green) → Service chung trỏ ĐÚNG 1 màu, **cutover = FLIP selector, rollback = flip lại**.

Sửa `charts/{api-svc,web-svc}/templates/service.yaml` (selector thêm `color` khi bluegreen bật) +
`values.yaml` (`global.bluegreen.activeColor: green`).

```bash
helm upgrade stock deploy/helm/stock -n stock          # rev18
kubectl get svc api-svc -n stock -o jsonpath='{.spec.selector}'
#   {"app":"api-svc","color":"green"}          ← kèm color
kubectl get endpoints api-svc -n stock
#   api-svc   10.244.1.32:18118,10.244.2.75:18118    ← CHỈ 2 green pod (không còn 4)

# CUTOVER green→blue:
#   declarative: helm upgrade ... --set global.bluegreen.activeColor=blue   (bền qua helm upgrade)
#   imperative : kubectl patch svc api-svc -n stock -p '{"spec":{"selector":{"app":"api-svc","color":"blue"}}}'
kubectl get endpoints api-svc -n stock
#   api-svc   10.244.1.31:18118,10.244.2.73:18118    ← đổi sang blue pods (cutover tức thời)
# ROLLBACK: flip lại về green → endpoints về green pods
```

**Chứng minh route ĐÚNG (không chỉ nhìn endpoints — soi APP-log pod thật):** flip sang màu X, curl
5 lần vào Service chung từ pod in-cluster (qua kube-proxy→selector) bằng **PATH ĐỘC NHẤT**
`/api/version-<marker>`, rồi đếm request trong **app-log** (Go zerolog access-log của container
`api-svc`, KHÔNG phải nginx ambassador) mỗi màu — chứng minh request tới ĐÚNG APP của đúng màu
(xuyên qua nginx ambassador → app container), không chỉ tới đúng endpoints:
```bash
MARKER=bgprobe-blue-$$
kubectl patch svc api-svc -n stock -p '{"spec":{"selector":{"app":"api-svc","color":"blue"}}}'
kubectl run bg-curl --image=nginx:1.27-alpine --restart=Never -n stock --command -- \
  sh -c "for i in 1 2 3 4 5; do wget -qO- \"http://api-svc:8118/api/version-$MARKER\"; done"
kubectl logs -n stock -l app=api-svc,color=blue  -c api-svc --tail=150 | grep -c "$MARKER"   # → 5
kubectl logs -n stock -l app=api-svc,color=green -c api-svc --tail=150 | grep -c "$MARKER"   # → 0
#   ⇒ BLUE hits=5 | GREEN hits=0  →  request CHỈ vào app blue = selector flip route đúng
#   dòng app-log thật: request method=GET path=/api/version-bgprobe-blue-<pid> status=404 dur_ms=0 ip=... user=-
# flip sang green rồi lặp lại: GREEN hits=5 | BLUE hits=0.
```
(Đã đo thật: blue=5/green=0 khi active=blue; green=5/blue=0 khi active=green. Chạy tự động:
`ONLY=2.2 ./run-day2.sh` — hàm `_bg_prove` in đủ get svc + endpoints + curl + app-log 2 màu.)

Điểm chốt:
- Đây là **blue/green kinh điển**: 2 Deployment `api-svc-blue` + `api-svc-green` (đã deploy sẵn bởi
  Helm `global.bluegreen.enabled=true`) DÙNG CHUNG 1 Service `api-svc` selector `{app, color}` + FLIP.
  Cả 2 màu Ready sẵn ⇒ cutover **tức thời zero-downtime**; rollback = flip lại (không rebuild/rollout).
- ⚠️ **Imperative-patch vs declarative-apply:** `kubectl patch` selector chỉ BỀN tới lần `helm upgrade`
  kế (helm reset selector về `activeColor`). Cutover LÂU DÀI phải `--set global.bluegreen.activeColor`.
- **Ingress (Lab 4.2) route `/api`→Service chung `api-svc`** ⇒ nay tự động theo màu active (trước hit cả 2).
- Song song với **gateway-native blue/green** (Rust controller + ConfigMap `gateway-bluegreen-state`,
  route per-color `api-svc-<color>` cho `/api`,`/`). Hai cơ chế độc lập — giữ `activeColor` khớp ConfigMap
  gateway. (Gateway route baked trong image nên KHÔNG gộp được nếu không rebuild.)
- **Biến thể 1-Deployment (tham khảo, KHÔNG phải demo của lab):** khi chỉ có 1 Deployment thì không
  có 2 bộ nhãn để lật selector — thay bằng **rolling update cấu hình blue/green-style**
  (`strategy.rollingUpdate.maxSurge:100% maxUnavailable:0`): bung nguyên bộ mới song song bộ cũ, Ready
  hết mới gỡ cũ (`set image` → cutover zero-downtime, `rollout undo` = rollback). Không gate/test được
  màu mới trước, không lật tức thì ⇒ không phải blue/green "thật".

## Lab 2.3 — Scale & HPA
- Duration: ~45 min | CKAD domain: Application Deployment (20%)
- Manually scale Deployment to 10 replicas
- Configure HPA at 50% CPU target

### ✅ Đã thực hiện (2026-07-11) — ns `stock`, app `cpu-burn` (registry.k8s.io/hpa-example)

App demo: `deploy/k8s/ckad-labs/day_2/cpu-burn.yaml` (Apache+PHP, mỗi GET chạy vòng
`sqrt()` đốt CPU) — nginx web-svc quá rẻ CPU nên không kích được HPA. `requests.cpu=100m`
(BẮT BUỘC cho HPA %). HPA declarative: `deploy/k8s/ckad-labs/day_2/cpu-burn-hpa.yaml`.

**Objective 1 — manual scale → 10 (làm TRƯỚC khi attach HPA):**
```bash
kubectl scale deploy/cpu-burn --replicas=10 -n stock     # 10/10 Running
kubectl get pods -n stock -l app=cpu-burn -o wide         # 10 pod, chỉ trên 2 worker
#   ckad-control-plane có taint NoSchedule => pod không lên control-plane
kubectl get rs -n stock -l app=cpu-burn
#   cpu-burn-64f94747b   10   ← CÙNG 1 ReplicaSet scale 1->10 (KHÔNG tạo RS mới,
#                                khác 2.1 nơi đổi image sinh RS mới)
kubectl scale deploy/cpu-burn --replicas=1 -n stock       # reset trước khi tạo HPA
```
- Image `registry.k8s.io/hpa-example` cache sẵn chỉ ở 1 node nhưng registry public →
  các node khác **pull thành công** khi scale-out (KHÔNG ImagePullBackOff).

**Objective 2 — HPA 50% CPU (declarative, autoscaling/v2):**
```bash
kubectl apply -f deploy/k8s/ckad-labs/day_2/cpu-burn-hpa.yaml
kubectl get hpa cpu-burn -n stock
#   TARGETS cpu: <unknown>/50%   ← ~15s đầu metrics chưa về
#   TARGETS cpu: 1%/50%          ← sau khi metric-server có mẫu (idle 1m/100m ≈ 1%)
kubectl get hpa cpu-burn -n stock -o jsonpath='{range .status.conditions[*]}{.type}={.status} {.reason}{"\n"}{end}'
#   AbleToScale=True ReadyForNewScale · ScalingActive=True ValidMetricFound
```
- Lệnh imperative tương đương (kubectl mới): `kubectl autoscale deploy/cpu-burn --cpu=50%
  --min=1 --max=10 -n stock` — `--cpu-percent` đã **deprecated**, dùng `--cpu=50%`.

**Bắn tải + scale-UP (2 terminal — cả loadgen lẫn watch-hpa đều BLOCK):**
```bash
# TERM A (mở TRƯỚC): ./e2e-test.sh watch-hpa cpu-burn
# TERM B: NS=stock ./e2e-test.sh loadgen cpu-burn 80 30 150 /   # 30 luồng x150s qua Service ClusterIP
kubectl top pods -n stock -l app=cpu-burn
#   CPU từng pod 194m–491m  = 194%–491% của REQUEST 100m (KHÔNG phải limit 500m)
```
Events (`kubectl describe hpa cpu-burn`) — **scale-up theo BẬC, KHÔNG nhảy thẳng 1→10:**
```
SuccessfulRescale  New size: 4    reason: cpu utilization (percentage of request) above target
SuccessfulRescale  New size: 8    (~15s sau)
SuccessfulRescale  New size: 10   (~15s sau)   ← chạm maxReplicas
```
- Vì scaleUp default cap ở max(+4 pod, ×2)/window ⇒ 1→4→8→10 qua ~3 chu kỳ sync (~15s),
  không phải 1→10 một nhịp dù metric ratio (≈500%/50%) tính ra desired=10 ngay.

**Scale-DOWN (cửa sổ ổn định 300s):**
```
[t≈0]    load dừng → TARGETS tụt 51% → 1%/50% NGAY, nhưng REPLICAS GIỮ 10
[t≈+5m]  SuccessfulRescale  New size: 1  reason: All metrics below target
```
- CPU về 1% từ ~20:51 nhưng HPA giữ 10 tới ~20:55:40 (**≈300s** `scaleDown.stabilizationWindowSeconds`
  mặc định) rồi mới hạ về `minReplicas=1`. scaleUp nhanh (stab 0s) / scaleDown chậm (stab 300s).

**Bài học (CKAD):**
- **HPA SỞ HỮU `spec.replicas` sau khi attach**: manual scale khi đã có HPA sẽ bị kéo về
  theo recommendation (idle → min=1) — nhưng do cửa sổ 300s nên KHÔNG tức thì (giữ ~5 phút).
  Vì vậy demo objective-1 làm trên `cpu-burn` khi CHƯA có HPA.
- **utilization = usage / `requests.cpu`** (100m), không phải limit ⇒ số đọc ~500% mới nhắm max.
  Thiếu `requests.cpu` ⇒ `TARGETS <unknown>/50%`, `ScalingActive=False FailedGetResourceMetric`.
- metrics-server trên kind cần `--kubelet-insecure-tls` (đã cài); `kubectl top pods` trả data
  = HPA đọc được PodMetrics.

Kết quả: manual scale 1→10 (cùng RS) OK; HPA scale-up 1→4→8→10 theo tải thật (TARGETS
~500%/50%), scale-down về 1 sau cửa sổ 300s — cả hai chiều chứng minh bằng SuccessfulRescale events.

**Scale-DOWN đo lại (2026-07-24, chu kỳ live đầy đủ):**
```
scale-UP  : New size 4 (t=40s) → 8 (t=60s) → 10 (t=80s)   reason: cpu above target
tải dừng ~t=150s → CPU về 1% tại t=200s NHƯNG HPA GIỮ 10 replicas
scale-DOWN: giữ đỉnh ~280s (cửa sổ 300s) → New size 4 → New size 1  reason: All metrics below target
```
scaleUp nhanh (stab 0s) / scaleDown chậm bất-đối-xứng (stab **300s**) — xác nhận lại bằng events.

### ✅ Áp HPA THẲNG lên service thật (2026-07-24) — api-svc + gateway-svc

`cpu-burn` (hpa-example) chỉ là app demo CPU-bound để kích HPA. Service project **I/O-bound/idle**
(api 1m, gateway 1-2m CPU vs requests 100m/50m ≈ 1-2%) nên CPU-HPA chỉ scale khi **traffic spike
thật** — vẫn áp được như feature HA. Gated Helm template `deploy/helm/stock/templates/hpa.yaml`
(`global.hpa.enabled`, default off):

```bash
helm upgrade stock deploy/helm/stock -n stock --set global.hpa.enabled=true
kubectl get hpa -n stock
#   api-svc-blue    Deployment/api-svc-blue    cpu: 1%/60%   2   6   2
#   api-svc-green   Deployment/api-svc-green   cpu: 1%/60%   2   6   2
#   gateway-svc     Deployment/gateway-svc     cpu: 2%/60%   2   5   2
```
Điểm chốt (áp HPA lên deployment ĐÃ có):
- **HPA SỞ HỮU `.spec.replicas`** → deployment phải **OMIT `replicas`** khi HPA bật (template dùng
  `{{ if not .Values.global.hpa.enabled }}replicas: ...{{ end }}`). Nếu vẫn hardcode replicas thì
  mỗi `helm upgrade` giành lại → **flapping** với HPA.
- ⚠️ Omit replicas ⇒ lúc BẬT lần đầu deployment default về **1** (2→1) rồi HPA `minReplicas=2` kéo
  lại 2 (self-heal ~vài giây). Các upgrade sau không đụng replicas nữa → ổn định.
- **api-svc blue/green** = 2 Deployment → HPA mỗi màu; màu idle nằm ở minReplicas.
- Service idle ⇒ HPA nằm im ở min; muốn thấy scale-up phải sinh tải thật (loadgen) — khác cpu-burn
  tự đốt CPU. Default off để portable; bật khi cần HA co giãn theo traffic.

## Lab 2.4 — Kustomize Overlay
- Duration: ~45 min | CKAD domain: Application Deployment (20%)
- Structure base and overlay directories
- Patch image tag and replica count without duplicating manifests

### ✅ Đã thực hiện — app `web-kz` trên **namespace RIÊNG `ckad-kustomize`** (2026-07-24: đổi từ ns `stock` sang ns riêng để output sạch, không lẫn 26 pod stack thật)

Manifest: `deploy/k8s/ckad-labs/day_2/kustomize/`
```
kustomize/
├── base/                # app cô lập web-kz (KHÔNG đụng web-svc thật), KHÔNG hardcode namespace
│   ├── kustomization.yaml   # resources: [deployment.yaml, service.yaml]
│   ├── deployment.yaml      # replicas 2, image web-svc:dev (tái dùng image đã kind-load)
│   └── service.yaml
└── overlays/
    ├── dev/  kustomization.yaml   # namePrefix dev-, replicas 1, images newTag: dev
    └── prod/ kustomization.yaml   # namePrefix prod-, replicas 5, images newTag: v2
```

**Tooling — chỉ có kubectl-embedded Kustomize (KHÔNG có binary rời):**
```bash
which kustomize                       # not found
kubectl version --client | grep -i kustomize
#   Kustomize Version: v5.8.1         # dùng `kubectl kustomize` / `kubectl apply -k`
```

**Render BASE trước — rồi DIFF base↔từng overlay để thấy overlay PATCH gì lên base**
(overlay chỉ chứa DELTA, KHÔNG nhân đôi manifest):
```bash
# 1) render BASE riêng: name=web-kz, replicas=2, image=web-svc:dev, KHÔNG namespace
kubectl kustomize kustomize/base

# 2) diff base ↔ dev → dev PATCH gì?
diff <(kubectl kustomize kustomize/base) <(kubectl kustomize kustomize/overlays/dev)
#   name web-kz -> dev-web-kz  ·  + namespace: ckad-kustomize  ·  replicas 2 -> 1  ·  GIỮ tag dev

# 3) diff base ↔ prod → prod PATCH gì?
diff <(kubectl kustomize kustomize/base) <(kubectl kustomize kustomize/overlays/prod)
#   name web-kz -> prod-web-kz ·  + namespace: ckad-kustomize  ·  replicas 2 -> 5  ·  image web-svc:dev -> web-svc:v2
```

**Bảng đối chiếu 3 render (base + 2 overlay):**

| Render | name         | namespace       | replicas | image        |
|--------|--------------|-----------------|----------|--------------|
| base   | web-kz       | (không)         | 2        | web-svc:dev  |
| dev    | dev-web-kz   | ckad-kustomize  | 1        | web-svc:dev  |
| prod   | prod-web-kz  | ckad-kustomize  | 5        | web-svc:v2   |

⇒ overlay patch không nhân đôi manifest — chỉ khai `namePrefix`/`namespace`/`replicas`/`images`.

**Apply -k thật vào NAMESPACE RIÊNG → output sạch:**
```bash
kubectl create namespace ckad-kustomize           # Kustomize set .metadata.namespace nhưng KHÔNG tự tạo ns
kubectl apply -k kustomize/overlays/dev           # dev-web-kz  (replicas 1)
kubectl apply -k kustomize/overlays/prod          # prod-web-kz (replicas 5)
kubectl get deploy,svc,pod -n ckad-kustomize      # OUTPUT SẠCH — chỉ đồ Kustomize, không lẫn stack
#   dev-web-kz   1/1   ...   prod-web-kz  5/5   (2 overlay cùng ns, khác namePrefix)
kubectl delete namespace ckad-kustomize           # dọn: xóa nguyên ns = xóa sạch mọi thứ Kustomize tạo
```

Điểm chốt:
- **Diff BASE↔overlay (không phải overlay↔overlay):** render base riêng rồi
  `diff <(kubectl kustomize base) <(kubectl kustomize overlays/dev)` cho thấy đúng DELTA mỗi
  overlay patch lên base (namePrefix, +namespace, replicas, images) — rõ hơn so 2 overlay với nhau.
- **`images: [{name: web-svc, newTag: v2}]`** — match theo IMAGE NAME (`web-svc`), KHÔNG phải
  container name (`web`). **`replicas: [{name: web-kz, count: 5}]`** — match theo DEPLOYMENT
  NAME *trước* `namePrefix`. `namespace:`/`namePrefix:` transform tên + ns. Overlay KHÔNG
  chép lại Deployment => DRY, đúng mục tiêu "without duplicating manifests".
- **BASE không hardcode `metadata.namespace`** (mùi kustomize): để overlay set qua `namespace:`.
  (Đã kiểm chứng overlay `namespace:` override được cả base có hardcode — nhưng idiomatic là
  base namespace-agnostic.)
- **Tag phải kind-load trước khi apply**: `web-svc:v2` đã có sẵn từ lab 2.1 → `apply -k` chạy
  ngay; nếu patch sang tag chưa `kind load` thì pod mới **ImagePullBackOff** (render vẫn đúng,
  chỉ apply-chạy mới cần image thật).
- **Cô lập bằng NAMESPACE RIÊNG (`ckad-kustomize`)**: overlay set `namespace:` → mọi resource vào
  ns riêng ⇒ `get -n ckad-kustomize` chỉ thấy đồ demo (dev-* + prod-*), KHÔNG lẫn 26 pod stack thật
  ở ns `stock`; dọn = `delete namespace` một phát. (Trước đây apply vào `stock` nên output rối +
  đụng ResourceQuota lab 3.4.) App vẫn dùng label `app=web-kz` riêng (không đụng `web-svc`).