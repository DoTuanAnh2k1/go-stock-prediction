# Day 4 — Networking & storage

> Cách tiếp cận: áp/verify trên stack THẬT (ns stock). Ingress là gated template chart
> `gateway-svc`; NetworkPolicy tách 2 chart — default-deny + policy multi-target ở chart
> `bootstrap`, `allow-backends-to-db` ở chart `db` (đều default off), bật bằng
> `--set ingress.enabled=true` (gateway-svc) và `--set networkPolicy.enabled=true` (CẢ bootstrap
> + db). Drill selector/PVC dùng object nháp (throwaway) rồi dọn. Helm local: `/home/chronical/.local/bin/helm`.

## Lab 4.1 — ClusterIP & NodePort
Duration: ~45 min | CKAD domain: Services and Networking (20%)
Create ClusterIP backend and NodePort frontend
Diagnose and fix selector mismatch
Verify Endpoints

### ✅ Đã thực hiện (2026-07-24)

**Đã có sẵn cả 2 loại Service trên stack thật:**
```bash
kubectl get svc -n stock -o custom-columns='NAME:.metadata.name,TYPE:.spec.type,PORT:.spec.ports[0].port,NODEPORT:.spec.ports[0].nodePort'
#   api-svc         ClusterIP   8118   <none>     ← backend nội bộ
#   prediction-svc  ClusterIP   8119   <none>
#   db              ClusterIP   5432   <none>     (headless, clusterIP=None)
#   cli-svc         NodePort    2345   32345      ← expose ra ngoài (SSH)
#   gateway-svc     NodePort    80     30080      ← frontend proxy
```

**Diagnose + fix selector mismatch (drill throwaway):**
```bash
kubectl create deploy ep-demo --image=nginx:1.27-alpine -n stock   # pod label app=ep-demo
# Service selector SAI (app=WRONG):
kubectl get endpoints ep-demo -n stock
#   ep-demo   <none>          ← ENDPOINTS RỖNG: selector không khớp pod nào
# FIX: sửa selector về app=ep-demo
kubectl patch svc ep-demo -n stock -p '{"spec":{"selector":{"app":"ep-demo"}}}'
kubectl get endpoints ep-demo -n stock
#   ep-demo   10.244.2.57:80  ← Endpoints controller nhồi IP:port pod Ready khớp selector
```
Điểm chốt:
- **Service selector = tiêu chí chọn pod**; Endpoints controller theo dõi pod `Ready` khớp
  selector → nhồi IP:port vào Endpoints. Selector sai / pod chưa Ready ⇒ **Endpoints rỗng =
  nguyên nhân #1 "service không tới được"**. `kubectl get endpoints <svc>` là lệnh debug đầu tiên.
- **ClusterIP** (default): IP ảo nội bộ, load-balance tới Endpoints. **NodePort**: mở cổng
  30000–32767 trên MỌI node → ClusterIP (đường expose thô, không cần LB).
- headless (`clusterIP: None`, db): DNS trả thẳng IP pod (StatefulSet cần).

## Lab 4.2 — Ingress Routing
Duration: ~60 min | CKAD domain: Services and Networking (20%)
Route / to frontend and /api to backend via Ingress
Verify via ingress controller endpoint

### ✅ Đã thực hiện (2026-07-24)

Cluster kind CHƯA có ingress controller → **cài ingress-nginx** (baremetal/NodePort, vì kind
không tạo với extraPortMappings 80/443):
```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.12.1/deploy/static/provider/baremetal/deploy.yaml
kubectl get pods -n ingress-nginx     # controller 1/1 Running; 2 admission Job Completed
kubectl get svc  ingress-nginx-controller -n ingress-nginx   # NodePort http=31825 https=30376
```

Ingress = **gated Helm template** `gateway-svc/templates/ingress.yaml` (host `stock.local`,
`/`→web-svc:3000, `/api`→api-svc:8118, KHÔNG rewrite path). Bật:
```bash
helm upgrade gateway-svc deploy/helm/gateway-svc -n stock --set ingress.enabled=true
kubectl get ingress -n stock
#   stock   nginx   stock.local   80
```

Verify routing (kind nodeIP không routable từ host → dùng port-forward controller):
```bash
kubectl port-forward -n ingress-nginx svc/ingress-nginx-controller 18080:80 &
curl -s -H "Host: stock.local" http://localhost:18080/api/version
#   {"service":"api-svc","git_sha":"unknown",...,"consistent":true,...}   HTTP 200  ✅
curl -s -H "Host: nope.invalid" http://localhost:18080/api/version -o /dev/null -w '%{http_code}\n'
#   404   ← host không khớp rule → default backend
```
Điểm chốt:
- **Ingress cần Ingress CONTROLLER** (Ingress object chỉ là "cấu hình mong muốn"; controller
  mới thực thi routing L7). `ingressClassName` phải khớp controller (`nginx`).
- Là bản **k8s-native SONG SONG với gateway-svc** (Rust) đang làm longest-prefix routing qua
  NodePort — cùng mục tiêu `/`→fe `/api`→be, khác cơ chế (controller chuẩn vs proxy tự viết).
- kind không map hostPort 80/443 (cluster tạo sẵn) → verify qua NodePort/port-forward thay vì
  `http://localhost`.

## Lab 4.3 — NetworkPolicy Isolation
Duration: ~45 min | CKAD domain: Services and Networking (20%)
Allow frontend → backend traffic only
Deny backend egress to internet (0.0.0.0/0)

### ✅ Đã thực hiện (2026-07-24) — **kindnet CÓ enforce** (đã verify, lật giả định ban đầu)

> ⚠️ Ban đầu tôi cho rằng kindnet KHÔNG enforce NetworkPolicy (kiến thức cũ). **Test thật lật
> lại:** bật default-deny → Ingress `/api` trả **HTTP 000** (backend không tới được); tắt →
> **200**. ⇒ kindnet của cluster này **enforce thật**. (Bài học: chạy lệnh kiểm chứng, đừng tin
> giả định.)

NetworkPolicy = gated template tách 2 chart (default off vì default-deny cần allow đủ mọi traffic
hợp lệ — Prometheus scrape :9464... — nếu không sẽ cắt nhầm metrics): `bootstrap/templates/networkpolicy.yaml`
(default-deny + 3 policy multi-target) và `db/templates/networkpolicy.yaml` (`allow-backends-to-db`).
5 policy: `default-deny-ingress` + allow gateway/ingress-nginx→frontend + api→auth/prediction +
backends→db + deny-backend-egress-internet.

```bash
# netpol graph tách 2 chart → bật CẢ hai (thiếu 1 thì db bị default-deny chặn):
helm upgrade bootstrap   deploy/helm/bootstrap   -n stock --set networkPolicy.enabled=true
helm upgrade db          deploy/helm/db          -n stock --set networkPolicy.enabled=true
helm upgrade gateway-svc deploy/helm/gateway-svc -n stock --set ingress.enabled=true
kubectl get networkpolicy -n stock     # 5 policy

# (A) Ingress vẫn 200 vì template ĐÃ allow ns ingress-nginx (namespaceSelector):
curl -s -H "Host: stock.local" http://localhost:18082/api/version -w '\nHTTP %{http_code}\n'
#   {"service":"api-svc",...}   HTTP 200   ← allow-gateway-to-frontend cho ingress-nginx qua

# (B) ISOLATION — pod lạ (app=np-test, KHÔNG trong allow-list) curl api-svc → BỊ CHẶN:
kubectl run np-test --image=nginx:1.27-alpine --restart=Never -n stock --labels=app=np-test \
  --command -- sh -c 'wget -T 5 -qO- http://api-svc:8118/health/ready; echo "wget_exit=$?"'
kubectl logs np-test -n stock
#   wget: download timed out
#   wget_exit=1              ← default-deny + không có allow cho app=np-test → TIMEOUT (chặn thật)
```
Điểm chốt:
- **default-deny-ingress** (`podSelector: {}`, `policyTypes:[Ingress]`, không có `ingress:`) = từ
  chối MỌI ingress; các policy `allow-*` **cộng dồn** (OR) mở đúng call-graph.
- Policy chỉ tác dụng khi **CNI hỗ trợ** — verify bằng test thật (pod trong allow-list qua, pod
  ngoài bị timeout), KHÔNG suy đoán.
- Cho ingress controller qua backend cần allow **ns của controller** (`namespaceSelector
  kubernetes.io/metadata.name: ingress-nginx`), nếu không Ingress 000 dù controller chạy.
- `deny-backend-egress-internet` (auth-svc/service-mgt): egress chỉ intra-cluster (10.0.0.0/8) +
  DNS :53 → chặn ra ngoài. KHÔNG áp prediction-svc (nó crawl external thật).
- Đã tắt lại (default off) sau demo để không chặn Prometheus scrape.

## Lab 4.4 — Persistent Volume Claims
Duration: ~45 min | CKAD domain: Services and Networking / Design and Build
Provision 1Gi PVC with dynamic provisioning
Mount in Pod, write data, delete Pod, recreate, verify persistence

### ✅ Đã thực hiện (2026-07-24)

StorageClass `standard` (rancher.io/local-path, `WaitForFirstConsumer`) đã có sẵn (db/minio/
rl-models đang dùng). Demo persistence bằng PVC nháp:
```bash
# (1) PVC 1Gi + writer pod ghi data
kubectl apply -f -  # PVC ck-data (RWO 1Gi) + Pod pvc-writer (echo ... > /data/proof.txt)
kubectl get pvc ck-data -n stock
#   ck-data   Bound   pvc-7d6b...   1Gi   RWO   standard   ← bind KHI pod schedule (WaitForFirstConsumer)
kubectl logs pvc-writer -n stock
#   ckad-4.4-persisted-02:49:06

# (2) XÓA writer pod
kubectl delete pod pvc-writer -n stock

# (3) reader pod mount CÙNG PVC (ck-data) → data còn nguyên
kubectl logs pvc-reader -n stock
#   READ-BACK: ckad-4.4-persisted-02:49:06     ← DATA PERSIST qua vòng đời pod ✅
```
Điểm chốt:
- **PVC = yêu cầu storage**; StorageClass **dynamic provisioner** tự tạo PV khớp (không cần
  admin tạo PV tay). `WaitForFirstConsumer` = hoãn bind PV tới khi pod đầu schedule (để chọn
  node/zone đúng — vì thế `backup-data` PVC "Pending/waiting for first consumer" là ĐÚNG, không
  phải bug).
- **PV vòng đời TÁCH KHỎI pod**: xóa pod, PVC/PV giữ nguyên; pod mới mount lại thấy data. Khác
  `emptyDir` (chết theo pod).
- RWO = mount ReadWriteOnce trên 1 node; reclaimPolicy `Delete` = xóa PVC ⇒ xóa PV+data.
