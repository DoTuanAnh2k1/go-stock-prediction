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

### ✅ Lời giải (mentor yêu cầu) — 1 Deployment, KHÔNG tạo 2 blue/green riêng

> ⚠️ Đề gốc ghi *"Run two Deployments (blue and green)"* + *"single Service selector flip"* —
> đó là blue/green KINH ĐIỂN (2 môi trường + lật selector). Nhưng mentor yêu cầu **chỉ dùng
> 1 Deployment**. Với 1 Deployment thì KHÔNG có 2 bộ nhãn để "lật selector" giữa blue↔green,
> nên bản 1-Deployment thực chất là **rolling update cấu hình blue/green-style**
> (`maxSurge:100% maxUnavailable:0`): bung nguyên bộ GREEN song song BLUE, green Ready hết rồi
> mới gỡ blue → cutover zero-downtime; `rollout undo` = rollback. KHÔNG phải blue/green "thật"
> (không gate/test green trước, không lật tức thì) nhưng thỏa ràng buộc "1 Deployment".

Áp trong Helm (default, `bluegreen.enabled=false`): `deploy/helm/stock/templates/web-svc-deployment.yaml`
+ `api-svc-deployment.yaml` đã đặt `strategy.rollingUpdate.maxSurge:100% maxUnavailable:0`.

```bash
kubectl -n stock set image deploy/web-svc web-svc=web-svc:v2   # cutover: green lên đủ rồi gỡ blue
kubectl -n stock rollout status deploy/web-svc                 # AVAILABLE giữ nguyên = zero-downtime
kubectl -n stock rollout undo deploy/web-svc                   # rollback tức thì
```

Kiểm chứng đã chạy thật (ns nháp, 4 replica): set image → **8 pod** (4 blue Ready + 4 green surge),
`AVAILABLE` giữ **4**; green Ready → blue gỡ (RS cũ 0, RS mới 4/4); `rollout undo` về blue sạch.

### 🔬 Biến thể nâng cao (blue/green "thật", 2 Deployment) — giữ tham khảo

Manifest: `deploy/k8s/ckad-labs/day_2/web-bluegreen.yaml` — 2 Deployment:
`web-svc-blue` (web-svc:dev) + `web-svc-green` (web-svc:v2), mỗi cái selector gồm
`color` (KHÔNG overlap).

```bash
kubectl delete deploy web-svc -n stock            # xóa web-svc đơn (selector {app} overlap)
kubectl apply -f deploy/k8s/ckad-labs/day_2/web-bluegreen.yaml   # blue 2/2, green 2/2

# trỏ Service sang BLUE
kubectl patch svc web-svc -n stock -p '{"spec":{"selector":{"app":"web-svc","color":"blue"}}}'
kubectl get endpoints web-svc -n stock
#   endpoints = 10.244.1.46 10.244.2.44   ← đúng 2 IP pod blue · curl = 200

# FLIP sang GREEN (poller đo xuyên suốt)
kubectl patch svc web-svc -n stock -p '{"spec":{"selector":{"app":"web-svc","color":"green"}}}'
#   [poller] RESULT ok=2033 fail=0        → zero-downtime ✅
kubectl get endpoints web-svc -n stock
#   endpoints = 10.244.1.45 10.244.2.45   ← đã đổi sang 2 IP pod green
```

Điểm chốt:
- **Selector 2 Deployment phải khác nhau (gồm `color`)** để không tranh pod; selector
  Deployment IMMUTABLE nên phải TẠO MỚI blue/green (không sửa được web-svc đơn thành blue).
- **Đổi selector Service = switch tức thời** (Endpoints trỏ pod set khác ngay), zero-downtime
  vì bộ pod màu mới đã Ready sẵn. Rollback = lật selector về màu cũ.
- Nhược: tốn 2× tài nguyên khi cả 2 màu cùng chạy.

**⚠️ Bẫy imperative-patch vs declarative-apply (gặp khi khôi phục):**
`color` được thêm vào selector bằng `kubectl patch`. Khi khôi phục, `kubectl apply -f
web-svc/service.yaml` (selector `{app: web-svc}`) **KHÔNG xoá** key `color` — apply chỉ
quản key nó biết trong last-applied-config. Kết quả: selector còn `{app, color:green}`,
mà web-svc đơn không có label color ⇒ **Endpoints RỖNG → frontend down**. Phải xoá tay:
```bash
kubectl patch svc web-svc -n stock --type=json -p '[{"op":"remove","path":"/spec/selector/color"}]'
#   selector={"app":"web-svc"} · endpoints=3 IP · curl=200
```

Khôi phục base: `kubectl apply -f deploy/k8s/web-svc/deployment.yaml` (web-svc đơn) +
xoá key color khỏi selector (trên) + `kubectl delete deploy web-svc-blue web-svc-green`.

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

## Lab 2.4 — Kustomize Overlay
- Duration: ~45 min | CKAD domain: Application Deployment (20%)
- Structure base and overlay directories
- Patch image tag and replica count without duplicating manifests

### ✅ Đã thực hiện (2026-07-11) — ns `stock`, app cô lập `web-kz`

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

**Patch image tag + replica count KHÔNG nhân đôi manifest** (overlay chỉ chứa DELTA):
```bash
kubectl kustomize kustomize/overlays/dev     # dev-web-kz  ns=stock replicas=1 image=web-svc:dev
kubectl kustomize kustomize/overlays/prod    # prod-web-kz ns=stock replicas=5 image=web-svc:v2
./e2e-test.sh kustomize-diff kustomize/overlays/dev kustomize/overlays/prod
#   dev  -> image=web-svc:dev replicas=1
#   prod -> image=web-svc:v2 replicas=5
#   PASS ✅ image tag VÀ replica count khác nhau — overlay patch không nhân đôi manifest
```

**Apply -k thật (bonus) + chứng minh cô lập khỏi frontend prod:**
```bash
kubectl apply -k kustomize/overlays/prod          # service/prod-web-kz + deployment/prod-web-kz
kubectl rollout status deploy/prod-web-kz -n stock # 5/5, image web-svc:v2
#   web-svc thật GIỮ NGUYÊN 3/3 (prod-web-kz label app=web-kz ≠ selector web-svc app=web-svc)
kubectl delete -k kustomize/overlays/prod         # dọn sạch
```

Điểm chốt:
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
- **Base cô lập (`app=web-kz`)** thay vì trỏ thẳng `deploy/k8s/web-svc/`: tránh `apply -k`
  làm bẩn Endpoints của frontend prod (selector overlap).