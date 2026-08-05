# Helm charts (kind — cluster CKAD)

Toàn bộ stack k8s đóng gói thành **2 Helm chart**. Về sau chỉnh sửa chỉ dùng `helm upgrade`.

```
deploy/helm/
├── stock/            # App stack — ns stock (db, service-mgt, prediction-svc, auth-svc,
│                     #   api-svc, web-svc, gateway-svc, cli-svc, pgadmin, minio, 8 pipeline cronjob file)
└── observability/    # ns observability — OTel Collector, Prometheus, Grafana, Tempo,
                      #   node-exporter, kube-state-metrics
```

> Port 1-1 từ `deploy/k8s/` cũ: mỗi resource một template, giữ nguyên comment.
> `deploy/k8s/ckad-labs/` vẫn là manifest thô (bài tập CKAD, không đóng gói Helm).

## Yêu cầu

- `helm` v3.x, `kubectl`, cluster kind tên `ckad`, context `kind-ckad`.
- Image `*:dev` phải build + `kind load` (chart chỉ tham chiếu, không build).

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
```

## Cài đặt

Chart `stock` KHÔNG quản `db-schema` ConfigMap (database.sql tạo bằng lệnh riêng).
Namespace `stock` phải có TRƯỚC để tạo ConfigMap đó → không dùng `--create-namespace` cho chart stock.

```bash
# 1) namespace + schema DB (pre-req cho db StatefulSet)
kubectl create namespace stock
kubectl create configmap db-schema -n stock --from-file=01-schema.sql=database.sql

# 2) app stack
helm install stock deploy/helm/stock -n stock

# 3) observability (chart riêng, namespace riêng)
helm install observability deploy/helm/observability -n observability --create-namespace
```

## Chỉnh sửa về sau (chỉ helm upgrade)

```bash
helm upgrade stock deploy/helm/stock -n stock                          # sau khi sửa template/values
helm upgrade stock deploy/helm/stock -n stock --set global.imageTag=v2 # bump image mọi app service
helm upgrade observability deploy/helm/observability -n observability

# xem sẽ đổi gì trước khi apply
helm diff upgrade stock deploy/helm/stock -n stock     # cần plugin helm-diff, tuỳ chọn
helm template stock deploy/helm/stock -n stock | kubectl diff -f -
```

## Toggle (values.yaml — chart stock)

| Key | Default | Tác dụng |
|-----|---------|----------|
| `global.imageTag` | `dev` | Tag chung mọi app image (`*-svc`) |
| `replicas.{api,auth,cli,gateway,prediction,serviceMgt,web}` | 1/1/1/1/2/1/3 | Số replica từng service |
| `pgadmin.enabled` | `true` | Bật/tắt pgAdmin |
| `bluegreen.enabled` | `false` | `false` = api-svc/web-svc chạy biến thể single; `true` = 2-màu (CKAD demo, cần gateway route màu active) |
| `manualJob.enabled` | `false` | Bật Job chạy tay (`pipeline-manual`) |
| `secrets.*` | dev values | Mật khẩu/JWT — override qua `--set` hoặc `values-prod.yaml` |

Observability: `grafana/prometheus/tempo/otelCollector/nodeExporter/kubeStateMetrics .enabled` (mặc định bật hết).

## Truy cập

```bash
kubectl port-forward -n stock svc/gateway-svc 8443:443    # https://localhost:8443 (web) + /api (self-signed => -k)
kubectl port-forward -n stock svc/pgadmin 8081:80         # pgAdmin
kubectl port-forward -n stock svc/cli-svc 2345:2345       # ssh -p 2345 user@localhost
kubectl port-forward -n observability svc/grafana 3000:3000
```

Login: `chon` / `Ch1nch2n@` (super_admin) hoặc `admin` / `admin123`.

## Gỡ

```bash
helm uninstall stock -n stock
helm uninstall observability -n observability
kubectl delete configmap db-schema -n stock      # không do Helm quản
kubectl delete namespace stock observability      # PVC reclaim Delete tự dọn
```

## Verify chart

```bash
helm lint deploy/helm/stock deploy/helm/observability
helm template stock deploy/helm/stock -n stock | kubectl apply --dry-run=server -f -
```
