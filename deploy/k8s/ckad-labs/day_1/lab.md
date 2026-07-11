# Day 1 — Pod design & imperative mastery

## Lab 1.1 — The 60-Second Pod
- Duration: ~45 min | CKAD domain: Application Design and Build (20%)
- Create a Pod imperatively with labels, environment variables, and resource - requests/limits
- Export a manifest using --dry-run=client -o yaml
- Verify Pod state without opening an editor (exam speed path)

## Lab 1.2 — Init + Sidecar Pattern
- Duration: ~60 min | CKAD domain: Application Design and Build (20%)
- Build a multi-container Pod with init container, app container, and sidecar
- Share data via emptyDir volume
- Tail application logs from sidecar using kubectl logs -c

### ✅ Đã thực hiện (2026-07-11)

Thay vì tạo Pod lab rời, **nhúng thẳng pattern vào Deployment thật** api-svc
(`deploy/k8s/api-svc/deployment.yaml`) — vừa học pattern vừa xài được production:

- **initContainer `wait-db`** — `pg_isready -h db` loop tới khi DB Ready mới cho app
  start (chạy tuần tự, xong mới tới containers). Bonus: hết restart lúc DB chưa lên.
- **app container `api`** — `sh -c "set -o pipefail; ./api-server 2>&1 | tee /var/log/app/api.log"`;
  log vừa ra stdout vừa ghi file vào emptyDir. `pipefail` để api-server chết thì shell
  trả mã lỗi (không bị `tee` nuốt) → liveness/restart đúng.
- **sidecar `log-sidecar`** — `tail -f /var/log/app/api.log`, phơi log app qua
  `kubectl logs -c log-sidecar` (chạy song song app, cùng Pod).
- **emptyDir `logs`** mount `/var/log/app` ở CẢ hai container = kênh chia sẻ.
- **`securityContext.fsGroup: 2000`** — image chạy `appuser` (uid 100/gid 101), không phải
  2000; fsGroup chown emptyDir về group 2000 (group-writable) + thêm 2000 làm *supplemental
  group* nên app non-root vẫn ghi được. (gid 2000 tùy chọn, không cần khớp user image.)

Verify:
```bash
kubectl apply -f deploy/k8s/api-svc/deployment.yaml
kubectl rollout status deploy/api-svc -n stock
kubectl get pods -n stock -l app=api-svc
POD=$(kubectl get pod -n stock -l app=api-svc -o jsonpath='{.items[0].metadata.name}')
kubectl logs $POD -n stock -c wait-db          # init chờ DB
kubectl logs $POD -n stock -c log-sidecar -f   # đọc log app QUA sidecar
```

Kết quả:
```
=== rollout ===
deployment "api-svc" successfully rolled out

=== pod (2/2 = api + log-sidecar) ===
NAME                       READY   STATUS    RESTARTS   AGE
api-svc-8476f74597-v8rg8   2/2     Running   0          13s

=== containers ===
init:  wait-db(ready=true)
main: api(ready=true) log-sidecar(ready=true)

=== wait-db init log (chứng minh chờ DB) ===
waiting for db:5432 ...
db:5432 - accepting connections
db is ready

=== log app QUA sidecar (kubectl logs -c log-sidecar) ===
... HTTP server listening on 0.0.0.0:8118

=== end-to-end (login chon -> /api/training/status qua gateway) ===
{"is_training":false,...,"current_phase":"idle","progress":0}
HTTP 200
```

Ghi chú: dùng sidecar dạng **container thường** (classic pattern) cho khớp bài học.
K8s ≥1.28 còn có *native sidecar* (initContainer + `restartPolicy: Always`) — để dành lab nâng cao.

## Lab 1.3 — Jobs & CronJobs
- Duration: ~45 min | CKAD domain: Application Design and Build (20%)
- Run a one-off Job to completion with backoffLimit
- Create a CronJob that spawns Jobs on a schedule
- Distinguish Job/CronJob from Deployment

### Bối cảnh: chuyển pipeline crawl/predict theo lịch sang k8s

Project vốn chạy pipeline 4 market bằng **APScheduler nội bộ** prediction-svc
(`crawler_gold/nasdaq/crypto/sp500` trong `DEFAULT_SCHEDULES`). Lab này chuyển việc
lên-lịch đó sang **CronJob** của k8s. Có 2 cách tiếp cận:

**Cách 1 — CronJob "mỏng" gọi trigger endpoint (không sửa code)**
```
CronJob → Pod(curl) → api-svc → gRPC → prediction-svc LÀM VIỆC
```
Pod chỉ `curl POST /api/trigger/{market}-crawler` với JWT admin. Nhanh, thuần YAML,
nhưng: việc thật vẫn nằm trong service always-on; `Job Complete` chỉ nghĩa là *curl
được 202*, không phản ánh crawl thành/bại; `backoffLimit` chỉ retry cú curl.
→ Đã thử nghiệm để hiểu cơ chế, KHÔNG giữ lại.

**Cách 2 — Job/CronJob TỰ chạy pipeline (k8s-native) — ĐANG DÙNG ✅**
```
CronJob → Job → Pod(python -m src.jobs_cli <market>) TỰ crawl→predict→reconcile → exit 0
```
Pod dùng **image prediction-svc**, tự nối DB, chạy pipeline một lượt rồi thoát.
`Job Complete` ⟺ pipeline thật sự xong; `backoffLimit`, `concurrencyPolicy`, `ttl`,
history đều điều khiển công việc thật. Lịch nằm ở CronJob YAML (đổi lịch = sửa manifest).

### ✅ Đã thực hiện Cách 2 (2026-07-11) — nhánh `k8s`

Code (prediction-svc):
- Thêm entrypoint run-once **`prediction-svc/src/jobs_cli.py`** — init tối thiểu
  (config → timezone ICT → logger → DB), gọi lại `job_crawl_<market>` có sẵn, rồi
  `sys.exit(0/1)`. KHÔNG bật gRPC/scheduler.
- **`scheduler/manager.py`**: đổi 4 `crawler_*` trong `DEFAULT_SCHEDULES` sang
  `enabled=False` (tránh chạy trùng — nếu để True, seed lúc startup ghi đè DB về
  True mỗi lần restart nên PUT /api/schedules không bền).

Manifest (`deploy/k8s/pipeline/`):
- `cronjob-{gold,crypto,nasdaq,sp500}.yaml` — image `prediction-svc:dev`,
  `command: ["python","-m","src.jobs_cli","<market>"]`, `envFrom` prediction-config +
  prediction-secret, `timeZone: Asia/Ho_Chi_Minh`, `concurrencyPolicy: Forbid`,
  `backoffLimit: 2`, `ttlSecondsAfterFinished`, history limits, `restartPolicy: Never`.
- `job-manual.yaml` — Job chạy tay (KHÔNG apply cùng CronJob).

Triển khai: rebuild image prediction-svc → `kind load` → `rollout restart` →
`kubectl apply` 4 CronJob (trừ job-manual).

Verify:
```bash
kubectl get cronjob -n stock
# tạo Job tức thời từ CronJob để test ngay (không chờ tới giờ):
kubectl create job verify-crypto --from=cronjob/pipeline-crypto -n stock
kubectl wait --for=condition=complete job/verify-crypto -n stock --timeout=240s
kubectl logs -n stock -l job-name=verify-crypto
```

Kết quả:
```
=== scheduler prediction-svc: KHÔNG còn add crawler ===
job_key=crawler_fundamentals  daily_reconcile  train_*   (crawler_gold/nasdaq/crypto/sp500 vắng mặt)

=== CronJobs ===
NAME              SCHEDULE         TIMEZONE           SUSPEND
pipeline-crypto   0 * * * *        Asia/Ho_Chi_Minh   False
pipeline-gold     0 * * * *        Asia/Ho_Chi_Minh   False
pipeline-nasdaq   15 * * * 1-5     Asia/Ho_Chi_Minh   False
pipeline-sp500    0,30 * * * 1-5   Asia/Ho_Chi_Minh   False

=== verify-crypto Job (pipeline TỰ chạy trong Pod) ===
Complete 1/1 (32s)
  jobs_cli.start market=crypto
  crypto.crawl.done errors=0 saved=3
  pipeline.crawl.done market=CRYPTO saved=3
  predict.crypto.start algorithms=13 coins=3
  sim.live_step.market_start bots=146 market=CRYPTO
  pipeline.reconcile.done market=CRYPTO scored=0
  crypto.intraday.done saved=49
  jobs_cli.done market=crypto            → exit 0 → Job Complete
```
(verify-gold cũng Complete nhưng `pipeline.skip.market_closed` vì GOLD đóng cuối tuần
— đúng logic, chứng tỏ entrypoint chạy đủ nhánh.)

Giới hạn đã biết: `_run_pipeline` nuốt lỗi từng-bước (ghi PipelineReport rồi return),
nên `Job Failed` chỉ bắt lỗi hạ tầng (DB down, import fail...). Muốn "crawl fail ⇒ Job
fail" chuẩn hơn thì cho `_run_pipeline` trả status — để dành. CronJob pod không mount
PVC `rl-models` (RWO, tránh tranh chấp với prediction-svc) → train mỗi-10-lần ghi
`/models` ephemeral; đủ cho lab, không ảnh hưởng crawl/predict/reconcile.

## Lab 1.4 — Label & Annotation Drill
- Duration: ~30 min | CKAD domain: Application Design and Build (20%)
- Bulk-create Pods and update labels across many objects
- Query resources with label selectors
- Use --overwrite for label changes

### Khái niệm
- **Label**: metadata để **chọn/nhóm** object — k8s dùng làm selector (Service→Pod,
  Deployment/RS→Pod, Job→Pod...), query được (`-l`). Nhỏ, ≤63 ký tự.
- **Annotation**: ghi chú tự do (JSON/cert/config), **KHÔNG** query bằng selector.
- ⚠️ `.spec.selector` của Deployment/StatefulSet là **immutable** (đổi phải xóa-tạo lại).
- ⚠️ Sửa label đã tồn tại **bắt buộc `--overwrite`**, không thì lỗi.

### Câu lệnh copy-paste (đã demo 2026-07-11 THẲNG trên ns `stock` — coi như sandbox)

Chỉ nghịch Pod nháp `demo-*` tự tạo; KHÔNG `--overwrite app=` lên Pod thật
(api-svc/db/gateway) vì đổi label *selector* của workload thật làm Service rớt
endpoint + RS đẻ Pod bù → roll back không sạch. Read-query chạy thoải mái trên đồ thật.
Cuối cùng xóa `demo-*` là roll back xong.

```bash
# --- setup: Pod nháp NGAY trong stock (coi stock là sandbox) ---
kubectl run demo-web --image=api-svc:dev -n stock --labels="app=demo,tier=frontend,env=dev" --command -- sleep 3600
kubectl run demo-api --image=api-svc:dev -n stock --labels="app=demo,tier=backend,env=prod" --command -- sleep 3600

# --- QUERY bằng selector ---
kubectl get pods -n stock -l app=demo --show-labels    # xem hết label
kubectl get pods -n stock -l app=demo -L tier,env      # -L (hoa): label thành CỘT
kubectl get pods -n stock -l 'app=demo,env in (prod)'  # set-based + AND
kubectl get pods -n stock -l 'tier!=frontend'          # bất đẳng thức
kubectl get pods -n stock -l '!release'                # key KHÔNG tồn tại

# --- THÊM / SỬA / XÓA label (CHỈ trên demo-*, đừng --overwrite app= lên Pod thật) ---
kubectl label pod demo-web release=canary -n stock            # thêm
kubectl label pod demo-web tier=backend -n stock              # LỖI: đã có value
kubectl label pod demo-web tier=backend --overwrite -n stock  # sửa (đúng)
kubectl label pod demo-web release- -n stock                  # xóa (hậu tố -)
kubectl label pods -l app=demo reviewed=yes -n stock          # hàng loạt theo selector

# --- ANNOTATION ---
kubectl annotate pod demo-web owner="team-x" description="demo" -n stock
kubectl annotate pod demo-web owner- -n stock                 # xóa annotation
kubectl get pod demo-web -n stock -o jsonpath='{.metadata.annotations}'

# --- Kỹ thuật: TÁCH 1 Pod khỏi ReplicaSet (deployment nháp demo-dep) ---
kubectl create deployment demo-dep --image=api-svc:dev -n stock -- sleep 3600
kubectl scale deployment demo-dep --replicas=3 -n stock
V=$(kubectl get pods -n stock -l app=demo-dep -o jsonpath='{.items[0].metadata.name}')
kubectl label pod "$V" app=quarantine --overwrite -n stock   # RS mất Pod -> đẻ Pod mới bù
kubectl get pods -n stock -l app=demo-dep -L app     # vẫn 3 (có 1 mới)
kubectl get pods -n stock -l app=quarantine -L app   # Pod bị tách, vẫn chạy để debug

# --- QUERY trên RESOURCE THẬT (chỉ đọc, an toàn) ---
kubectl get cronjob -n stock -l market=gold          # CronJob pipeline theo market
kubectl get cronjob -n stock -l app=pipeline         # cả 4 pipeline CronJob
kubectl get pods -n stock -l app=api-svc -L app      # Pod do Service api-svc chọn
kubectl get pods -n stock -l job-name=<job> -L job-name  # Job tự gắn label job-name=

# --- ROLL BACK: xóa mọi thứ nháp (đồ thật giữ nguyên) ---
kubectl delete deployment demo-dep -n stock
kubectl delete pod -n stock -l 'app in (demo,quarantine)'
```

Kết quả then chốt đã kiểm chứng (chạy thật trên ns `stock`):
- `kubectl label ... tier=backend` (không `--overwrite`) → `error: 'tier' already has a value (frontend)`.
- Tách Pod: relabel `app=demo-dep`→`quarantine` → RS đẻ Pod mới ngay (`app=demo-dep` count=3),
  Pod tách `app=quarantine` count=1, vẫn `Running`.
- Sau roll back: `kubectl get pods,deploy -n stock -l 'app in (demo,demo-dep,quarantine)'` → rỗng;
  api-svc/db/gateway/prediction-svc nguyên vẹn.