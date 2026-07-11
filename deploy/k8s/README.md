# k8s manifests (kind — cluster CKAD)

Manifest tách theo service, mỗi service 1 folder, mỗi resource 1 file.

```
deploy/k8s/
├── namespace.yaml          # Namespace stock (apply đầu tiên)
├── db/                     # TimescaleDB/Postgres (StatefulSet + headless svc)
├── service-mgt/            # Go registry gRPC :8121 (mặc định tắt, deploy cho đủ bộ)
├── prediction-svc/         # Python gRPC :8119 (ML) + PVC rl-models
├── auth-svc/               # Java gRPC :8120 (auth/RBAC, Flyway)
├── api-svc/                # Go HTTP :8118
├── web-svc/                # React/Nginx :3000
├── gateway-svc/            # Rust :80/:443 — điểm vào (NodePort)
├── cli-svc/                # Go SSH :2345 (NodePort)
├── pgadmin/                # UI quản trị DB (tùy chọn)
└── ckad-labs/              # Bài tập CKAD theo ngày (day_1, day_2, ... — không thuộc stack chính)
```

Mỗi folder service gồm (tuỳ service): `configmap.yaml`, `secret.yaml`,
`deployment.yaml`/`statefulset.yaml`, `service.yaml`, `pvc.yaml`.

## Khác biệt so với docker-compose (do chạy trên kind)

- **`version_data` bị bỏ.** Compose mount volume chung cho mọi service ghi
  `/versions/<svc>.json` rồi api-svc đọc tổng hợp (`GET /api/version`). Volume này
  cần **RWX**, mà StorageClass `standard` của kind chỉ **RWO** → bỏ mount;
  `/api/version` chỉ mất phần tổng hợp cross-service, không ảnh hưởng chức năng.
- **`rl_models`** (chỉ prediction-svc ghi) → PVC RWO (`prediction-svc/pvc.yaml`).
- **gateway certs** → `emptyDir`; entrypoint tự sinh self-signed mỗi lần khởi động.
- **cli-svc host key** → `emptyDir` (sinh lại mỗi restart) + `fsGroup: 10001`.
- **Điểm vào**: gateway-svc dùng **NodePort** (30080/30443); cli-svc NodePort 32345.
  Các service nội bộ dùng **ClusterIP**.

## Chuẩn bị (1 lần)

Kind không thấy image trên host — phải build và `kind load` từng image `*:dev`.
Chỉ `db` (timescaledb) và `pgadmin` là public image, không cần load.

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

# ConfigMap schema DB (database.sql quá lớn nên tạo bằng lệnh, không static yaml).
# SAU khi tạo namespace, TRƯỚC khi apply db/.
kubectl create configmap db-schema -n stock --from-file=01-schema.sql=database.sql
```

## Thứ tự apply

```bash
kubectl apply -f deploy/k8s/namespace.yaml
kubectl create configmap db-schema -n stock --from-file=01-schema.sql=database.sql
kubectl apply -f deploy/k8s/db/            # DB lên trước (mọi service cần Postgres)
kubectl apply -f deploy/k8s/service-mgt/
kubectl apply -f deploy/k8s/prediction-svc/
kubectl apply -f deploy/k8s/auth-svc/
kubectl apply -f deploy/k8s/api-svc/
kubectl apply -f deploy/k8s/web-svc/
kubectl apply -f deploy/k8s/gateway-svc/
kubectl apply -f deploy/k8s/cli-svc/
kubectl apply -f deploy/k8s/pgadmin/       # tùy chọn
```

Hoặc apply cả cây (namespace + db-schema phải có TRƯỚC):

```bash
kubectl apply -f deploy/k8s/namespace.yaml
kubectl create configmap db-schema -n stock --from-file=01-schema.sql=database.sql
kubectl apply -R -f deploy/k8s/
```

> Không cần lệ thuộc thứ tự tuyệt đối: gRPC client dial lazy, pod khởi động lại
> tới khi dependency sẵn sàng. DB nên có trước để tránh CrashLoop nhiều vòng.

## Truy cập

```bash
# Web + API qua gateway (self-signed => -k)
kubectl port-forward -n stock svc/gateway-svc 8443:443
#   https://localhost:8443           (web)
#   https://localhost:8443/api/...   (api)

kubectl port-forward -n stock svc/pgadmin 8081:80     # pgAdmin
kubectl port-forward -n stock svc/cli-svc 2345:2345   # ssh -p 2345 user@localhost
```

Login mặc định: `chon` / `Ch1nch2n@` (super_admin, seed bởi auth-svc) hoặc `admin` / `admin123`.

## Dọn sạch

```bash
kubectl delete namespace stock   # cascade toàn bộ; PV reclaim policy Delete tự dọn
```
