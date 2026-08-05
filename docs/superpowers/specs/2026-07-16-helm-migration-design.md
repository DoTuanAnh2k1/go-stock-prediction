# Helm migration — design spec

**Date:** 2026-07-16
**Goal:** Chuyển toàn bộ manifest k8s thô (`deploy/k8s/`, apply bằng `kubectl`) sang Helm. Về sau chỉnh sửa chỉ dùng `helm upgrade`.

## Quyết định đã chốt

| Vấn đề | Quyết định |
|---|---|
| Cấu trúc chart | **1 template / resource** (port 1-1, giữ comment tiếng Việt/CKAD). KHÔNG DRY generic template. |
| Manifest cũ | **Xóa** stack chính trong `deploy/k8s/`. **Giữ** `deploy/k8s/ckad-labs/`. |
| db-schema | **Giữ bước `kubectl create configmap` thủ công** (database.sql = 47KB, dưới 1MB nên bundle được, nhưng vẫn chọn manual). |
| Blue/green (api-svc, web-svc) | Mặc định **single variant**; bluegreen là template **gated `bluegreen.enabled=false`**. |
| job-manual | Template **gated `manualJob.enabled=false`** (Job bare chạy lại mỗi upgrade → immutable lỗi). |
| Observability | **Chart RIÊNG** `deploy/helm/observability/` (ns `observability`, vòng đời độc lập). |

## Phạm vi (54 yaml, ngoài ckad-labs)

### Chart `stock` (ns `stock`) — `deploy/helm/stock/`
- Services: db (3), service-mgt (4), prediction-svc (5), auth-svc (4), api-svc (5), web-svc (3), gateway-svc (4), cli-svc (4), pgadmin (4)
- minio (1 file, nhiều resource: Secret, PVC, Deployment, Service, createbucket Job)
- pipeline (9): cronjob-{backup,crypto,gold,nasdaq,simulation,sp500,train,weekly}, job-manual
- `namespace.yaml` bị bỏ (dùng `-n stock`, pre-create thủ công cho bước db-schema)

### Chart `observability` (ns `observability`) — `deploy/helm/observability/`
- grafana, prometheus, tempo, otel-collector, node-exporter, kube-state-metrics (namespace.yaml bỏ, dùng `--create-namespace`)

## Layout

```
deploy/helm/
├── stock/
│   ├── Chart.yaml
│   ├── values.yaml
│   ├── README.md
│   └── templates/
│       ├── NOTES.txt
│       ├── db-{statefulset,service,secret}.yaml
│       ├── service-mgt-{deployment,service,configmap,secret}.yaml
│       ├── prediction-svc-{deployment,service,configmap,secret,pvc}.yaml
│       ├── auth-svc-{deployment,service,configmap,secret}.yaml
│       ├── api-svc-{deployment,service,configmap,secret,bluegreen}.yaml
│       ├── web-svc-{deployment,service,bluegreen}.yaml
│       ├── gateway-svc-{deployment,service,rbac,configmap}.yaml
│       ├── cli-svc-{deployment,service,configmap,secret}.yaml
│       ├── pgadmin-{deployment,service,configmap,secret}.yaml
│       ├── minio.yaml
│       └── cronjob-{...}.yaml, job-manual.yaml
└── observability/
    ├── Chart.yaml
    ├── values.yaml
    └── templates/{grafana,prometheus,tempo,otel-collector,node-exporter,kube-state-metrics}.yaml
```

Ánh xạ: mỗi `deploy/k8s/<svc>/<file>.yaml` → `templates/<svc>-<file>.yaml`, **giữ nguyên nội dung + comment**, chỉ thay các trường dưới.

## Convention parameter hóa (NHẸ — chỉ 5 loại thay đổi)

1. **namespace:** `namespace: stock` → `namespace: {{ .Release.Namespace }}` (mọi resource). Tên DNS liên-service (vd `otel-collector.observability:4317`, `api-svc:8118`) **giữ literal**.
2. **image (chỉ app image `*-svc:dev`):** `image: api-svc:dev` → `image: "{{ .Values.images.apiSvc }}:{{ .Values.global.imageTag }}"`. Áp cho init/app/sidecar dùng cùng image. **Image bên thứ ba** (timescaledb, nginx:1.27-alpine, pgadmin, minio, curl, grafana, prometheus, tempo, otel, node-exporter, kube-state-metrics) **giữ literal**. `web-svc:v2` (green demo) giữ literal trong template bluegreen gated.
3. **replicas:** → `{{ .Values.<svcKey>.replicas }}` (default: api=1, prediction=2, web=3, còn lại=1). replicas trong bluegreen gated giữ literal (=2).
4. **imagePullPolicy:** giữ literal (`IfNotPresent`) — không đổi.
5. **secret stringData:** thay giá trị bằng `{{ .Values.secrets.* }}` (xem key bên dưới).

Gate: `{{- if .Values.pgadmin.enabled }}…{{- end }}` (pgadmin), `manualJob.enabled` (job-manual), `bluegreen.enabled` (api-svc-bluegreen, web-svc-bluegreen). ConfigMap (nginx conf, env, probes, OTel, dnsConfig ndots) **giữ verbatim**.

### values.yaml (stock) — key chuẩn

```yaml
global:
  imageTag: dev
  imagePullPolicy: IfNotPresent   # tham chiếu tuỳ chọn; template giữ literal
images:
  apiSvc: api-svc
  authSvc: auth-svc
  cliSvc: cli-svc
  gatewaySvc: gateway-svc
  predictionSvc: prediction-svc
  serviceMgt: service-mgt
  webSvc: web-svc
replicas:
  api: 1
  auth: 1
  cli: 1
  gateway: 1
  prediction: 2
  serviceMgt: 1
  web: 3
secrets:
  postgresUser: postgres
  postgresPassword: "123"
  postgresDb: go_stock_prediction
  jwtSecret: change-me-in-dev
  internalSecret: change-me-in-dev
  adminPassword: admin123
  s3AccessKey: minioadmin
  s3SecretKey: minioadmin123
  minioRootUser: minioadmin
  minioRootPassword: minioadmin123
  pgadminPassword: admin
pgadmin:
  enabled: true
manualJob:
  enabled: false
bluegreen:
  enabled: false
```

Map secret → values.secrets:
- db-secret: POSTGRES_USER→postgresUser, POSTGRES_PASSWORD→postgresPassword, POSTGRES_DB→postgresDb
- api-secret: POSTGRES_PASSWORD→postgresPassword, ADMIN_PASSWORD→adminPassword, JWT_SECRET→jwtSecret, INTERNAL_SECRET→internalSecret
- auth-secret: DB_PASSWORD→postgresPassword, JWT_SECRET→jwtSecret, INTERNAL_SECRET→internalSecret
- prediction-secret: POSTGRES_PASSWORD→postgresPassword, S3_ACCESS_KEY→s3AccessKey, S3_SECRET_KEY→s3SecretKey
- service-mgt-secret: POSTGRES_PASSWORD→postgresPassword
- cli-secret: INTERNAL_SECRET→internalSecret
- pgadmin-secret: PGADMIN_DEFAULT_PASSWORD→pgadminPassword
- minio-secret: MINIO_ROOT_USER→minioRootUser, MINIO_ROOT_PASSWORD→minioRootPassword

## Flow deploy mới

```bash
# build + kind load image *:dev (không đổi)
kubectl create namespace stock
kubectl create configmap db-schema -n stock --from-file=01-schema.sql=database.sql
helm install stock deploy/helm/stock -n stock
helm install observability deploy/helm/observability -n observability --create-namespace

# chỉnh sửa về sau
helm upgrade stock deploy/helm/stock -n stock
helm upgrade stock deploy/helm/stock -n stock --set global.imageTag=v2   # bump image
```

## Verify
- `helm lint deploy/helm/stock` + `deploy/helm/observability`
- `helm template ... | kubectl apply --dry-run=client -f -` (render + validate)
- So khớp resource count với manifest gốc.
```
```
