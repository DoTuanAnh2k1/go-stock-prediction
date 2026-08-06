# Helm charts (kind — cluster CKAD)

Toàn bộ stack k8s đóng gói thành **các Helm chart per-service độc lập** — mỗi service = 1 chart
top-level, cài/upgrade/rollback riêng từng chart. Về sau chỉnh sửa chỉ dùng `helm upgrade <chart>`.

```
deploy/helm/
├── common/          # LIBRARY CHART (type: library) — partial ambassador/sidecar/HA;
│                    #   backend chart phụ thuộc qua `file://../common`
├── bootstrap/       # ns governance (cài ĐẦU TIÊN): ResourceQuota + LimitRange, pod-reader RBAC,
│                    #   PDBs, netpol default-deny + 3 policy multi-target
├── db/              # StatefulSet TimescaleDB + netpol allow-backends-to-db (single-target)
├── minio/           # object storage (bucket models)
├── service-mgt/     # gRPC registry
├── prediction-svc/  # Python ML/crawl (replicas 2)
├── auth-svc/        # Java auth
├── api-svc/         # Go API + HPA (api + blue/green) + bluegreen
├── gateway-svc/     # Rust gateway + HPA + Ingress
├── web-svc/         # React frontend + bluegreen
├── cli-svc/         # SSH TUI
├── pgadmin/         # cài-hay-không (không còn toggle enabled)
├── cronjobs/        # 8 pipeline cronjob + db backup + job-manual (gated manualJob.enabled)
└── observability/   # ns observability — OTel Collector, Prometheus, Grafana, Tempo,
                     #   node-exporter, kube-state-metrics (vòng đời độc lập)
```

> Mỗi chart tự chứa values (imageTag, image, secrets nó cần, toggle riêng) — KHÔNG còn umbrella,
> KHÔNG còn `global:`. `deploy/k8s/ckad-labs/` vẫn là manifest thô (bài tập CKAD, không đóng gói Helm).

## Yêu cầu

- `helm` v3.x, `kubectl`, cluster kind tên `ckad`, context `kind-ckad`.
- Image `*:dev` phải build + `kind load` (chart chỉ tham chiếu, không build).
- Backend chart (`api-svc, auth-svc, prediction-svc, service-mgt, cli-svc`) phụ thuộc library
  `common` → cần `helm dependency build deploy/helm/<svc>` (vendor vào `charts/`) TRƯỚC khi install.
  `make helm-deps` chạy cho cả 5 chart; `scripts/deploy.sh` tự lo.

## Chuẩn bị (1 lần)

```bash
# từ repo root — build đúng context/Dockerfile như compose
docker build -t api-svc:dev        -f deploy/api-svc.Dockerfile        .
docker build -t prediction-svc:dev -f deploy/prediction-svc.Dockerfile .
docker build -t auth-svc:dev       -f deploy/auth-svc.Dockerfile       auth-svc
docker build -t service-mgt:dev    -f deploy/service-mgt.Dockerfile    service-mgt
docker build -t gateway-svc:dev    -f deploy/gateway-svc.Dockerfile    gateway-svc
docker build -t web-svc:dev        -f deploy/web-svc.Dockerfile        web-svc
docker build -t cli-svc:dev        -f deploy/cli-svc.Dockerfile        cli-svc
for i in api-svc prediction-svc auth-svc service-mgt gateway-svc web-svc cli-svc; do
  kind load docker-image $i:dev --name ckad
done

# vendor library `common` vào các backend chart
make helm-deps      # = helm dependency build cho api/auth/prediction/service-mgt/cli
```

## Cài đặt

Namespace `stock` phải có TRƯỚC (để tạo `db-schema` ConfigMap — KHÔNG do Helm quản, DB StatefulSet
mount từ nó). Các chart cài theo THỨ TỰ phụ thuộc:
`bootstrap → db → minio → service-mgt → prediction-svc → auth-svc → api-svc → gateway-svc → web-svc → cli-svc → pgadmin → cronjobs`.

```bash
# 1) namespace + schema DB (pre-req cho db StatefulSet)
kubectl create namespace stock
kubectl create configmap db-schema -n stock --from-file=01-schema.sql=database.sql

# 2) app stack — dùng script (loop qua mọi chart đúng thứ tự + helm dependency build)
./scripts/deploy.sh

#    ...hoặc install tay từng chart, ví dụ:
helm dependency build deploy/helm/api-svc
helm install bootstrap deploy/helm/bootstrap -n stock
helm install db        deploy/helm/db        -n stock
helm install api-svc   deploy/helm/api-svc   -n stock
# ...tiếp tục theo thứ tự phụ thuộc cho các chart còn lại

# 3) observability (chart riêng, namespace riêng)
helm install observability deploy/helm/observability -n observability --create-namespace
```

pgAdmin nay là **cài-hay-không** — muốn có thì `helm install pgadmin deploy/helm/pgadmin -n stock`;
không thì đơn giản bỏ qua (không còn `--set pgadmin.enabled`).

## Chỉnh sửa về sau (chỉ helm upgrade — per chart)

```bash
helm upgrade api-svc deploy/helm/api-svc -n stock                     # sau khi sửa template/values
helm upgrade api-svc deploy/helm/api-svc -n stock --set imageTag=v2   # bump image chart này
helm upgrade observability deploy/helm/observability -n observability

# xem sẽ đổi gì trước khi apply
helm diff upgrade api-svc deploy/helm/api-svc -n stock     # cần plugin helm-diff, tuỳ chọn
helm template api-svc deploy/helm/api-svc -n stock | kubectl diff -f -
```

Bump image mọi service = chạy `--set imageTag=v2` cho từng chart (script `deploy.sh` truyền
`imageTag` đồng nhất cho cả loop).

## Toggle (per-chart `--set`)

| `--set` | Chart | Default | Tác dụng |
|---------|-------|---------|----------|
| `imageTag` | mỗi chart (app image) | `dev` | Tag image chart đó |
| `replicas` | mỗi service chart | 1/2 | Số replica service đó (vd `--set replicas=4`) |
| `bluegreen.enabled` / `bluegreen.activeColor` | `api-svc`, `web-svc` | `true` / `green` | 2-màu blue/green + màu active |
| `hpa.enabled` | `api-svc` **và** `gateway-svc` | `false` | Bật HPA (cần metrics-server) |
| `ingress.enabled` | `gateway-svc` | `false` | Bật Ingress `/`→web-svc, `/api`→api-svc |
| `networkPolicy.enabled` | `bootstrap` **và** `db` | `false` | Bật netpol (graph tách 2 chart — set CẢ hai) |
| `quota.enabled` / `rbac.enabled` / `pdb.enabled` | `bootstrap` | `true` | ns governance |
| `manualJob.enabled` | `cronjobs` | `false` | Bật Job chạy tay (`pipeline-manual`) |
| `secrets.*` | chart có secret | dev values | Mật khẩu/JWT — override qua `--set` hoặc `-f <chart>-secret.yaml` |

> **NetworkPolicy tách 2 chart:** default-deny + 3 policy multi-target ở `bootstrap`; policy
> single-target `allow-backends-to-db` ở `db`. Bật demo phải set `--set networkPolicy.enabled=true`
> cho CẢ `bootstrap` lẫn `db` — nếu thiếu 1 thì graph khuyết (db bị default-deny chặn).

Observability: `grafana/prometheus/tempo/otelCollector/nodeExporter/kubeStateMetrics .enabled` (mặc định bật hết).

## Truy cập

```bash
kubectl port-forward -n stock svc/gateway-svc 8443:443    # https://localhost:8443 (web) + /api (self-signed => -k)
kubectl port-forward -n stock svc/pgadmin 8081:80         # pgAdmin
kubectl port-forward -n stock svc/cli-svc 2345:2345       # ssh -p 2345 user@localhost
kubectl port-forward -n observability svc/grafana 3000:3000
```

Login: `chon` / `Ch1nch2n@` (super_admin) hoặc `admin` / `admin123`.

## Upgrade + rollback (CKAD P6 — trên MỘT chart)

Vòng đời độc lập per-service = điểm ăn tiền của per-service charts:

```bash
helm upgrade api-svc deploy/helm/api-svc -n stock --set imageTag=v2   # revision mới
helm history api-svc -n stock                                         # lịch sử revision chart này
helm rollback api-svc 1 -n stock                                      # lùi về revision 1
```

## Gỡ

```bash
# uninstall từng chart (thứ tự ngược lại tuỳ ý — Helm không ép)
for c in cronjobs pgadmin cli-svc web-svc gateway-svc api-svc auth-svc prediction-svc service-mgt minio db bootstrap; do
  helm uninstall $c -n stock
done
helm uninstall observability -n observability
kubectl delete configmap db-schema -n stock      # không do Helm quản
kubectl delete namespace stock observability      # PVC reclaim Delete tự dọn
```

## Verify chart

```bash
for c in bootstrap common db minio service-mgt prediction-svc auth-svc api-svc gateway-svc web-svc cli-svc pgadmin cronjobs; do
  helm lint deploy/helm/$c
done
helm lint deploy/helm/observability
helm template api-svc deploy/helm/api-svc -n stock | kubectl apply --dry-run=server -f -
```
