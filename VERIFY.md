# CKAD Capstone — Verify & Run Guide

> One-stop guide to **deploy** the stack on Kubernetes and **verify** every CKAD
> capstone requirement + the day 1–5 labs. Verified live on **kind Kubernetes v1.35.0**.
>
> Hướng dẫn một-cửa để **triển khai** stack lên Kubernetes và **kiểm chứng** mọi yêu cầu
> capstone CKAD + lab ngày 1–5. Đã verify trực tiếp trên **kind Kubernetes v1.35.0**.

---

## Where everything lives / Mọi thứ nằm ở đâu

| What / Cái gì | Path | Note |
|---|---|---|
| **Verify/deploy scripts** | [`scripts/`](scripts/) | `build.sh` · `deploy.sh` · `smoke-test.sh` · `run-labs.sh` |
| **Cluster up/down** | [`deploy/k8s/cluster-start.sh`](deploy/k8s/cluster-start.sh) · [`cluster-stop.sh`](deploy/k8s/cluster-stop.sh) | start/stop kind `ckad` |
| **CKAD day labs 1–5** | [`deploy/k8s/ckad-labs/`](deploy/k8s/ckad-labs/) | `day_1..day_5/` each has `run-dayN.sh` + `lab.md`; `DEMO.md` runbook; `index.html` viewer |
| **§4 checklist** (req → resource → verify) | [`docs/ckad-checklist.md`](docs/ckad-checklist.md) | maps every mandatory item |
| **Capstone requirements** | [`deploy/k8s/ckad-labs/capstone-requirements.md`](deploy/k8s/ckad-labs/capstone-requirements.md) | the graded spec |
| **Helm umbrella chart** | [`deploy/helm/stock/`](deploy/helm/stock/) | 12 subcharts; toggles in `values.yaml` |
| **Kustomize (P5)** | [`deploy/k8s/kustomize/`](deploy/k8s/kustomize/) | base + overlays dev/prod |
| **Design / Implementation** | [`DESIGN.md`](DESIGN.md) · [`IMPLEMENTATION.md`](IMPLEMENTATION.md) | architecture + code map |

---

# English

## 0. Prerequisites

- Docker, `kubectl`, `helm` v3, `kind` **v0.32+** (ships Kubernetes v1.35 node image)
- A kind cluster named `ckad` (3 nodes) on `kindest/node:v1.35.0`
- In-cluster add-ons: **ingress-nginx** (Ingress) and **metrics-server** (HPA)
- `helm` may live at `~/.local/bin/helm` (not on PATH); scripts fall back to it automatically

Create the cluster (first time only) — 3 nodes, ingress port-mapping:

```bash
cat > /tmp/kind-ckad.yaml <<'YAML'
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: ckad
nodes:
  - role: control-plane
    kubeadmConfigPatches:
      - |
        kind: InitConfiguration
        nodeRegistration:
          kubeletExtraArgs: { node-labels: "ingress-ready=true" }
    extraPortMappings:
      - { containerPort: 80,  hostPort: 80,  protocol: TCP }
      - { containerPort: 443, hostPort: 443, protocol: TCP }
  - role: worker
  - role: worker
YAML
kind create cluster --config /tmp/kind-ckad.yaml --image kindest/node:v1.35.0

# add-ons
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
kubectl patch -n kube-system deployment metrics-server --type='json' \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'
kubectl apply -f https://kind.sigs.k8s.io/examples/ingress/deploy-ingress-nginx.yaml
# pin ingress controller onto the ingress-ready (port-mapped) node
kubectl -n ingress-nginx patch deploy ingress-nginx-controller --type merge \
  -p '{"spec":{"template":{"spec":{"nodeSelector":{"ingress-ready":"true"},"tolerations":[{"key":"node-role.kubernetes.io/control-plane","operator":"Exists","effect":"NoSchedule"}]}}}}'
```

Already have the cluster but nodes are stopped? Just bring it back:

```bash
deploy/k8s/cluster-start.sh      # docker start 3 nodes + fix inotify + restart CoreDNS
```

## 1. Build, deploy, smoke-test (3 commands)

```bash
./scripts/build.sh          # build 7 images :dev + `kind load` into cluster "ckad"
./scripts/deploy.sh         # ns stock + db-schema ConfigMap + helm install (HPA/Ingress/NetPol ON)
./scripts/smoke-test.sh     # E2E: /api/version → login → monitoring → gold → frontend
```

`smoke-test.sh` targets `http://localhost` by default. For kind, port-forward the gateway first:

```bash
kubectl -n stock port-forward svc/gateway-svc 8080:80 &
BASE_URL=http://localhost:8080 USER=chon PASS='Ch1nch2n@' ./scripts/smoke-test.sh
```

## 2. Verify the capstone §4 requirements

The full requirement → resource → command mapping is in
[`docs/ckad-checklist.md`](docs/ckad-checklist.md). Quick inventory:

```bash
kubectl get pods,svc,endpoints -n stock          # D1/N1/N5 — Ready, no orphan endpoints
kubectl get deploy,cronjob -n stock              # D2/P1
kubectl get hpa,ingress,netpol -n stock          # P4/N3/N4
kubectl get resourcequota,limitrange -n stock    # C5
kubectl auth can-i list pods --as=system:serviceaccount:stock:pod-reader -n stock   # C4 → yes
kubectl get pvc -n stock                         # D5
helm history stock -n stock                      # P6
# Ingress from host (add "stock.local" resolves to 127.0.0.1, or use --resolve):
curl -H 'Host: stock.local' http://localhost/api/version   # → 200 (api-svc)
curl -H 'Host: stock.local' http://localhost/              # → 200 (web-svc)
```

## 3. Run the CKAD day labs (1–5)

All labs at once from repo root:

```bash
./scripts/run-labs.sh            # runs day 1→5, handles the NetworkPolicy toggle
./scripts/run-labs.sh 3          # run a single day (1..5)
```

Or run a single day directly (each is self-contained, self-cleaning):

```bash
bash deploy/k8s/ckad-labs/day_1/run-day1.sh     # Jobs/CronJobs + Labels
RUN_LOAD=0 bash deploy/k8s/ckad-labs/day_2/run-day2.sh   # Rolling/BlueGreen/HPA/Kustomize
bash deploy/k8s/ckad-labs/day_3/run-day3.sh     # ConfigMap/Secret/SecurityContext/RBAC/Quota
bash deploy/k8s/ckad-labs/day_4/run-day4.sh     # ClusterIP/Ingress/NetworkPolicy/PVC
bash deploy/k8s/ckad-labs/day_5/run-day5.sh     # Probes/Observability/Triage/Helm
```

**NetworkPolicy note:** the labs are designed with NetworkPolicy **off** (chart default); day 4
toggles it on→demo→off itself. The capstone "submitted" state keeps it **on**. `run-labs.sh`
disables NetPol for the lab run and re-enables it at the end. Each `run-dayN.sh` accepts
`ONLY=<n.n>`, `KEEP=1`, and other flags — see the header of each script.

## 4. Tear down

```bash
deploy/k8s/cluster-stop.sh       # docker stop nodes (state preserved; cluster-start.sh restores)
```

---

# Tiếng Việt

## 0. Yêu cầu

- Docker, `kubectl`, `helm` v3, `kind` **v0.32+** (kèm node image Kubernetes v1.35)
- Cluster kind tên `ckad` (3 node) trên `kindest/node:v1.35.0`
- Add-on trong cluster: **ingress-nginx** (Ingress) và **metrics-server** (HPA)
- `helm` có thể nằm ở `~/.local/bin/helm` (không trên PATH); script tự fallback sang đó

Tạo cluster (lần đầu) — xem lệnh `kind create` ở phần English §0 (3 node + ingress port-mapping
+ cài metrics-server/ingress-nginx). Nếu cluster đã có nhưng node đang tắt:

```bash
deploy/k8s/cluster-start.sh      # docker start 3 node + vá inotify + restart CoreDNS
```

## 1. Build, deploy, smoke-test (3 lệnh)

```bash
./scripts/build.sh          # build 7 image :dev + `kind load` vào cluster "ckad"
./scripts/deploy.sh         # ns stock + ConfigMap db-schema + helm install (bật HPA/Ingress/NetPol)
./scripts/smoke-test.sh     # E2E: /api/version → đăng nhập → monitoring → gold → frontend
```

`smoke-test.sh` mặc định trỏ `http://localhost`. Với kind, port-forward gateway trước:

```bash
kubectl -n stock port-forward svc/gateway-svc 8080:80 &
BASE_URL=http://localhost:8080 USER=chon PASS='Ch1nch2n@' ./scripts/smoke-test.sh
```

## 2. Kiểm chứng yêu cầu capstone §4

Bảng ánh xạ đầy đủ yêu cầu → resource → lệnh nằm ở
[`docs/ckad-checklist.md`](docs/ckad-checklist.md). Kiểm nhanh: dùng các lệnh `kubectl get ...`
ở phần English §2 (pods/deploy/cronjob/hpa/ingress/netpol/quota/limitrange/pvc + `auth can-i` +
`helm history` + `curl` Ingress từ host).

Ý nghĩa cột: mỗi mã (D1, P4, C4, N3, D5, P6...) tương ứng một mục **Required** trong
[`capstone-requirements.md`](deploy/k8s/ckad-labs/capstone-requirements.md) §4.

## 3. Chạy các bài lab CKAD (1–5)

Chạy cả 5 ngày từ repo root:

```bash
./scripts/run-labs.sh            # chạy ngày 1→5, tự xử lý bật/tắt NetworkPolicy
./scripts/run-labs.sh 3          # chạy 1 ngày (1..5)
```

Hoặc chạy từng ngày trực tiếp (mỗi script tự chứa, tự dọn) — xem lệnh ở phần English §3.

**Lưu ý NetworkPolicy:** các lab thiết kế với NetworkPolicy **tắt** (mặc định chart); ngày 4 tự
bật→demo→tắt. Trạng thái "nộp bài" của capstone giữ NetPol **bật**. `run-labs.sh` tắt NetPol khi
chạy lab rồi bật lại ở cuối. Mỗi `run-dayN.sh` nhận cờ `ONLY=<n.n>`, `KEEP=1`... — xem header
từng script.

## 4. Tắt cluster

```bash
deploy/k8s/cluster-stop.sh       # docker stop node (giữ nguyên state; cluster-start.sh bật lại)
```

---

## Expected results / Kết quả mong đợi

| Check | Expected |
|---|---|
| `kubectl get nodes` | 3× `Ready v1.35.0` |
| Pods in `stock` | all `Running` (backend pods `3/3` = ambassador) |
| `scripts/smoke-test.sh` | **ALL PASS** (version, login, monitoring, gold, frontend) |
| Ingress from host | `/api/version` → **200**, `/` → **200** |
| Day 1–5 labs | each prints **`✔ XONG Day N (Lab all)`** |
| Day 2 rolling update | `RESULT ok=<n> fail=0` (zero-downtime) |

*See [`docs/ckad-checklist.md`](docs/ckad-checklist.md) for the full §4 evidence matrix.*
