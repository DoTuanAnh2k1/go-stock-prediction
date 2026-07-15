# CKAD Labs — Demo Runbook

Hướng dẫn demo lại từng lab trên cluster kind `ckad`, ns `stock`.
Lệnh chi tiết + output nằm trong **viewer** (`index.html`); file này là **thứ tự chạy +
chuẩn bị + dọn + bẫy**. Chạy `kubectl` từ **repo root**; harness ở `deploy/k8s/ckad-labs/day_2/e2e-test.sh`.

```bash
# Mở viewer (nút copy per-lệnh):
cd deploy/k8s/ckad-labs && python3 -m http.server 8090   # → http://localhost:8090
```

**Trạng thái sẵn có (khỏi dựng lại):** full stack (auth/cli/pgadmin/prediction/service-mgt/gateway),
blue/green (web-svc-blue/green, api-svc-blue/green), cpu-burn, 4 CronJob pipeline, metrics-server.
Kiểm nhanh: `kubectl get deploy,cronjob -n stock`.

**Mẹo xem log:** k9s có thể chết watcher (`fsnotify: too many open files` = giới hạn inotify của HOST,
không phải lỗi pod). Xem trọn thì dùng `kubectl logs`, hoặc nâng limit:
`sudo sysctl fs.inotify.max_user_instances=512 fs.inotify.max_user_watches=524288`.

---

## Day 1 — Pod design & imperative

### 1.1 — The 60-Second Pod  (drill imperative)
**Chạy:**
```bash
kubectl run web --image=nginx --restart=Never --labels=app=web,tier=frontend \
  --env=ENV=dev --dry-run=client -o yaml > /tmp/pod.yaml   # sinh manifest, chưa tạo
# (thêm resources.requests/limits vào /tmp/pod.yaml)
kubectl apply -f /tmp/pod.yaml
kubectl get pod web -o wide --show-labels
```
**Mong đợi:** Pod `web` Running, thấy labels.
**Dọn:** `kubectl delete pod web`

### 1.2 — Init + Sidecar  ⚠️ có bẫy
Pattern init+app+sidecar nhúng trong `deploy/k8s/api-svc/deployment.yaml`.
⚠️ **Bẫy:** api-svc giờ đã thành blue/green → label `app=api-svc` **overlap**; apply bản đơn sẽ
cảnh báo và lẫn với blue/green.
**Demo sạch (khuyến nghị):** apply vào ns riêng, hoặc xem nhanh rồi xóa ngay:
```bash
kubectl apply -f deploy/k8s/api-svc/deployment.yaml     # (cảnh báo overlap — chấp nhận)
kubectl rollout status deploy/api-svc -n stock          # chờ 2/2 (init xong + sidecar)
kubectl logs deploy/api-svc -n stock -c wait-db         # init chờ DB → "db is ready"
kubectl logs deploy/api-svc -n stock -c log-sidecar     # sidecar tail log app
kubectl delete deploy api-svc -n stock                  # XÓA NGAY kẻo lẫn blue/green
```

### 1.3 — Jobs & CronJobs
4 CronJob pipeline **đang chạy sẵn** — xem chúng tự đẻ Job theo lịch:
```bash
kubectl get job -n stock          # pipeline-<market>-<slot>  (hậu tố = phút unix của lịch)
```
**Chạy tay 1 Job on-demand (không đợi lịch):**
```bash
kubectl create job verify-crypto --from=cronjob/pipeline-crypto -n stock
kubectl wait --for=condition=complete job/verify-crypto -n stock --timeout=240s
kubectl logs -n stock -l job-name=verify-crypto        # pipeline TỰ chạy trong Pod
```
- `--from=cronjob/X` = sao chép `jobTemplate` của CronJob → Job chạy NGAY.
**Mong đợi:** `Complete 1/1`; log: crawl.done → predict → sim → reconcile → intraday → `jobs_cli.done`.
**Demo retry (backoffLimit):** apply Job `exit 1` (khối trong viewer) → 3 Pod Error → `Failed reason=BackoffLimitExceeded`.
**Dọn:** `kubectl delete job verify-crypto -n stock`

### 1.4 — Label & Annotation  (Pod nháp trong stock)
**Chạy** (copy khối 1.4 trong viewer):
```bash
kubectl run demo-web --image=api-svc:dev -n stock --labels=app=demo,tier=frontend,env=dev --command -- sleep 3600
kubectl get pods -n stock -l app=demo -L tier,env            # -L: label thành cột
kubectl get pods -n stock -l 'app=demo,env in (prod)'        # set-based
kubectl label pod demo-web tier=backend --overwrite -n stock # thiếu --overwrite ⇒ lỗi
# tách Pod khỏi RS: tạo demo-dep, scale 3, relabel 1 pod app=quarantine → RS đẻ bù
```
**Dọn:** `kubectl delete deploy demo-dep -n stock; kubectl delete pod -n stock -l 'app in (demo,quarantine)'`

---

## Day 2 — Deployments & rollouts

### 2.1 — Rolling Update & Rollback  (nginx demo `web`)
**Chuẩn bị:** `kubectl apply -f deploy/k8s/ckad-labs/day_2/web.yaml`  (4 replica, maxUnavailable=0, preStop)
**Chạy — zero-downtime (đo từ TRONG cluster):**
```bash
DUR=45 ./deploy/k8s/ckad-labs/day_2/e2e-test.sh zero-downtime web 80 web nginx:1.27 /
#   RESULT ok=... fail=0  → PASS (nhờ preStop; port-forward-đo là SAI vì ghim 1 pod)
```
**Bad deploy + rollback:**
```bash
kubectl set image deploy/web web=nginx:9.9.9-nope -n stock
kubectl rollout status deploy/web -n stock --timeout=20s      # timed out (KẸT, ImagePullBackOff)
kubectl get pods -n stock -l app=web                          # pod cũ vẫn Running → không sập
kubectl rollout undo deploy/web -n stock                      # rollback
```
- Monitor: `kubectl get deploy web -w` (cột AVAILABLE không tụt), `describe` Events + Conditions,
  `progressDeadlineSeconds` (600s) khác `--timeout` của lệnh chờ.
**Dọn:** `kubectl delete -f deploy/k8s/ckad-labs/day_2/web.yaml`

### 2.2 — Blue/Green  (đang chạy sẵn) — tay → tự động
**Cách TAY (patch selector):**
```bash
./deploy/k8s/ckad-labs/day_2/e2e-test.sh bluegreen web-svc color blue
./deploy/k8s/ckad-labs/day_2/e2e-test.sh bluegreen web-svc color green   # xem endpoints đổi
# ⚠️ dọn key color kẹt (bẫy patch↔apply):
kubectl patch svc web-svc -n stock --type=json -p '[{"op":"remove","path":"/spec/selector/color"}]'
```
**Cách TỰ ĐỘNG (gateway):**  (idle: web-svc active=blue→idle=green; api-svc active=green→idle=blue)
```bash
kubectl get cm gateway-bluegreen-state -n stock -o jsonpath='{.data}'   # màu active per app
kubectl set image deploy/web-svc-green web-svc=web-svc:v2 -n stock       # set image màu IDLE
kubectl logs -n stock deploy/gateway-svc -f | grep bluegreen            # rollout.start→flip→promote
```
- Rollback thật: patch web-svc-<idle> sang image Ready-nhưng-503 → gateway `rollback` (log total/fail).
- Bad deploy: set image màu idle = `web-svc:nope` → không Ready → controller KHÔNG flip.

### 2.3 — Scale & HPA  (cpu-burn sẵn, metrics-server sẵn, CHƯA có HPA)
**Manual scale (làm TRƯỚC khi tạo HPA):**
```bash
kubectl scale deploy/cpu-burn --replicas=10 -n stock
kubectl get rs -n stock -l app=cpu-burn        # CÙNG 1 RS scale 1→10 (không tạo RS mới)
kubectl scale deploy/cpu-burn --replicas=1 -n stock
```
**HPA + tải (2 terminal, cả hai BLOCK):**
```bash
kubectl apply -f deploy/k8s/ckad-labs/day_2/cpu-burn-hpa.yaml     # autoscaling/v2, cpu 50%
# tab A (mở TRƯỚC):
./deploy/k8s/ckad-labs/day_2/e2e-test.sh watch-hpa cpu-burn
# tab B:
NS=stock ./deploy/k8s/ckad-labs/day_2/e2e-test.sh loadgen cpu-burn 80 30 150 /
```
**Mong đợi:** scale-up 1→4→8→10 (~15s/bậc); tải dừng → CPU về 1% NGAY nhưng REPLICAS giữ 10
tới ~5 phút (stabilization window 300s) → về 1.
**Dọn:** `kubectl delete hpa cpu-burn -n stock`  (cpu-burn về 1/1)

### 2.4 — Kustomize Overlay  (overlays sẵn ở `day_2/kustomize/`)
```bash
KZ=deploy/k8s/ckad-labs/day_2/kustomize
kubectl kustomize $KZ/overlays/prod          # xem YAML render (image tag + replicas đã patch)
./deploy/k8s/ckad-labs/day_2/e2e-test.sh kustomize-diff $KZ/overlays/dev $KZ/overlays/prod
#   PASS: image tag VÀ replica count khác nhau — overlay patch không nhân đôi manifest
```
- Không apply thì không cần dọn. Muốn apply: `kubectl apply -k $KZ/overlays/prod`.

---

## Thứ tự demo khuyến nghị
1.3 (nhẹ, sạch) → 1.4 → 1.1 → 1.2 (nhớ xóa ngay) → 2.1 → 2.2 → 2.4 → 2.3 (lâu nhất, ~7 phút vì window HPA).

## Dọn tổng (về baseline)
```bash
kubectl delete job -n stock -l job-name          # job tay lỡ tạo
kubectl delete hpa cpu-burn -n stock --ignore-not-found
kubectl delete -f deploy/k8s/ckad-labs/day_2/web.yaml --ignore-not-found
kubectl delete pod -n stock -l 'app in (demo,quarantine,web)' --ignore-not-found
# blue/green + CronJob + cpu-burn để nguyên (là feature/baseline).
```
