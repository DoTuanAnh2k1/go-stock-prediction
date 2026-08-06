# App-level Kustomize — CKAD P5 (image api-svc thật)

Minh hoạ kỹ năng Kustomize **base + overlay** (§4.2 P5) trên **image `api-svc` thật**,
song song với các Helm chart per-service (`deploy/helm/<svc>/`, ví dụ `deploy/helm/api-svc/`).
Khác lab `day_2/kustomize/` (dùng nginx demo).

```
base/                 name=api-kz · replicas=2 · image=api-svc:dev · KHÔNG namespace
overlays/dev/         + namespace=stock · namePrefix dev-  · replicas=1 · tag dev
overlays/prod/        + namespace=stock · namePrefix prod- · replicas=3 · tag v2 · strategy zero-downtime
```

## Xem diff overlay (không apply)

```bash
kubectl kustomize deploy/k8s/kustomize/overlays/dev
kubectl kustomize deploy/k8s/kustomize/overlays/prod
diff <(kubectl kustomize deploy/k8s/kustomize/overlays/dev) \
     <(kubectl kustomize deploy/k8s/kustomize/overlays/prod)
# PASS: image tag (dev↔v2) VÀ replica count (1↔3) khác nhau — patch, không nhân đôi manifest.
```

## Apply (yêu cầu Helm stack đã chạy trong ns stock)

`api-kz` dùng `envFrom` Secret `api-svc` (do Helm tạo) + trỏ Service DNS thật
(`db`, `auth-svc`, `prediction-svc`) → chỉ Ready khi stack Helm đã lên.

```bash
kubectl apply -k deploy/k8s/kustomize/overlays/dev
kubectl rollout status deploy/dev-api-kz -n stock
kubectl delete -k deploy/k8s/kustomize/overlays/dev     # dọn
```
