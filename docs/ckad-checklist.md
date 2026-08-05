# CKAD Capstone Checklist — go-stock-prediction

> **Mục đích:** ánh xạ từng mục **Required** trong `deploy/k8s/ckad-labs/capstone-requirements.md` §4 → **resource K8s thật + đường dẫn file + lệnh verify**. Đây là deliverable §6.1 (`docs/ckad-checklist.md`).
>
> **Nguồn triển khai:** umbrella Helm chart [`deploy/helm/stock/`](../deploy/helm/stock/) (12 subchart). Kiến trúc tổng thể xem [DESIGN.md](../DESIGN.md); cài đặt tầng mã xem [IMPLEMENTATION.md](../IMPLEMENTATION.md).
>
> **Namespace:** `stock`. **Cluster:** kind `ckad` (policy-capable CNI = kindnet; ingress-nginx + metrics-server cài thêm).

---

## 0. Trạng thái tổng quan

| Domain | Điểm | Trạng thái |
|---|---|---|
| §4.1 Design & Build | 20 | ✅ đủ 6/6 Required |
| §4.2 Deployment | 20 | ✅ đủ 6/6 (HPA/Kustomize xem ghi chú) |
| §4.3 Config & Security | 25 | ✅ đủ 6/6 |
| §4.4 Networking | 20 | ✅ đủ 5/5 (Ingress/NetPol demo-gated) |
| §4.5 Observability | 15 | ✅ đủ (probes + runbook README) |

**Toggle demo-gated** (default OFF vì hạ tầng cần cài trước / tránh nhiễu idle) — BẬT khi demo/nộp:

```bash
helm upgrade stock deploy/helm/stock -n stock \
  --set global.hpa.enabled=true \
  --set global.ingress.enabled=true \
  --set global.networkPolicy.enabled=true
```

Toggle default ON sẵn: `quota`, `rbac`, `pdb`, `bluegreen`, `pgadmin`.

---

## §4.1 Application Design and Build (20%)

| # | Yêu cầu | Resource | File | Verify |
|---|---|---|---|---|
| **D1** | Custom image mỗi service | 7 image `*-svc:dev` | [`deploy/*.Dockerfile`](../deploy/) | `docker images \| grep svc` |
| **D2** | Deployment + Job/CronJob | 8 CronJob + Job | [`charts/cronjobs/templates/`](../deploy/helm/stock/charts/cronjobs/templates/) | `kubectl get cronjob,deploy -n stock` |
| **D3** | init + sidecar | init `wait-db` + `nginx` ambassador + `log-sidecar` | `charts/<svc>/templates/deployment.yaml` + [`charts/common/templates/_ambassador.tpl`](../deploy/helm/stock/charts/common/templates/_ambassador.tpl) | `kubectl get pod <p> -n stock -o jsonpath='{.spec.initContainers[*].name} \| {.spec.containers[*].name}'` |
| **D4** | emptyDir chia log | volume `logs` (emptyDir) app↔sidecar | idem deployment.yaml | `kubectl get pod <p> -n stock -o jsonpath='{.spec.volumes[*].name}'` |
| **D5** | PVC bền | `postgres_data`, `backup-data` (2Gi), MinIO PVC | `charts/db/templates/statefulset.yaml`, `charts/cronjobs/`, `charts/minio/` | `kubectl get pvc -n stock` → xóa pod db → data còn |
| **D6** | Label cho blue/green | selector `color: {blue,green}` | `charts/api-svc/templates/bluegreen.yaml`, `charts/web-svc/templates/bluegreen.yaml` | `kubectl get pods -n stock -L color,app` |

**Ghi chú D3:** 5 backend (`api/auth/prediction/service-mgt/cli`) chạy pod ≥4 container (init + app + ambassador nginx + log-sidecar) — vừa init **vừa** sidecar (mục "both preferred").

---

## §4.2 Application Deployment (20%)

| # | Yêu cầu | Resource | File | Verify |
|---|---|---|---|---|
| **P1** | Deployment ≥1 replica | mọi service stateless ≥2 | `charts/<svc>/values.yaml` (`replicas`) | `kubectl get deploy -n stock` |
| **P2** | Rolling update có tài liệu | strategy RollingUpdate | [`ckad-labs/DEMO.md`](../deploy/k8s/ckad-labs/DEMO.md) §2.1 | `kubectl set image deploy/... && kubectl rollout status ...` |
| **P3** | blue/green **hoặc** canary | blue/green flip selector Service chung | `charts/api-svc/templates/bluegreen.yaml` (`activeColor`) | `kubectl patch svc api-svc -n stock -p '{"spec":{"selector":{"color":"blue"}}}'` |
| **P4** | HPA | 2 HPA (api-svc, gateway-svc) CPU 60% | [`templates/hpa.yaml`](../deploy/helm/stock/templates/hpa.yaml) | `--set global.hpa.enabled=true` → `kubectl get hpa -n stock` |
| **P5** | Kustomize base + overlay | base + overlays dev/prod (patch image+replicas) | [`deploy/k8s/kustomize/`](../deploy/k8s/kustomize/) (api-svc thật) · [`ckad-labs/day_2/kustomize/`](../deploy/k8s/ckad-labs/day_2/kustomize/) (lab) | `kubectl kustomize deploy/k8s/kustomize/overlays/prod` |
| **P6** | Helm upgrade + rollback | umbrella `stock` | [`deploy/helm/stock/`](../deploy/helm/stock/) | `helm upgrade ... && helm history stock -n stock && helm rollback stock <rev> -n stock` |

**Ghi chú P4:** default OFF vì service idle → CPU-HPA chỉ scale khi có traffic thật; khi bật, Deployment omit `.spec.replicas` (HPA sở hữu, tránh flapping).

**Ghi chú P5:** `deploy/k8s/kustomize/` kustomize **image api-svc thật** (base `api-kz` + dev/prod patch `images:` tag & `replicas:`); day_2 lab kustomize nginx demo. Cả hai đều minh hoạ base+overlay+patch.

---

## §4.3 Application Environment, Configuration & Security (25%)

| # | Yêu cầu | Resource | File | Verify |
|---|---|---|---|---|
| **C1** | ConfigMap (env/volume) | ConfigMap mỗi backend + nginx ConfigMap | `charts/<svc>/templates/configmap.yaml` | `kubectl get cm -n stock` |
| **C2** | Secret (credentials) | Secret mỗi backend + db + minio | `charts/<svc>/templates/secret.yaml` | `kubectl get secret -n stock` |
| **C3** | SecurityContext | `runAsNonRoot`, `allowPrivilegeEscalation:false`, drop ALL | `charts/<svc>/templates/deployment.yaml` (`securityContext`) | `kubectl get pod <p> -n stock -o jsonpath='{.spec.containers[0].securityContext}'` |
| **C4** | SA + Role + RoleBinding | `pod-reader` SA/Role/RB + 5 SA per-service (automount:false) + gateway RBAC | [`templates/serviceaccounts.yaml`](../deploy/helm/stock/templates/serviceaccounts.yaml), `charts/gateway-svc/templates/rbac.yaml` | `kubectl auth can-i list pods --as=system:serviceaccount:stock:pod-reader -n stock` = yes |
| **C5** | ResourceQuota + LimitRange | `stock-quota` + `stock-defaults` | [`templates/quota.yaml`](../deploy/helm/stock/templates/quota.yaml) | `kubectl get resourcequota,limitrange -n stock` |
| **C6** | requests/limits mọi container | resources trên app + LimitRange vá container thiếu | deployment.yaml + quota.yaml | `kubectl describe resourcequota stock-quota -n stock` |

**Ghi chú C2 (secret & git):** `global.secrets.*` trong `values.yaml` chỉ là **placeholder dev** (`change-me-in-dev`, `123`, `minioadmin`) — KHÔNG phải credential thật. Secret production externalize qua [`values-secret.yaml`](../deploy/helm/stock/values-secret.yaml.example) (gitignore, override bằng `-f`). Không có credential thật commit vào git → không chạm điều kiện auto-fail "Secrets committed in plaintext".

---

## §4.4 Services and Networking (20%)

| # | Yêu cầu | Resource | File | Verify |
|---|---|---|---|---|
| **N1** | ClusterIP nội bộ | Service ClusterIP mọi backend | `charts/<svc>/templates/service.yaml` | `kubectl get svc -n stock` |
| **N2** | NodePort/Ingress | gateway-svc LoadBalancer :80/:443 + Ingress | `charts/gateway-svc/templates/service.yaml`, `templates/ingress.yaml` | `kubectl get svc gateway-svc -n stock` |
| **N3** | Ingress ≥2 rule | `/` → web-svc:3000, `/api` → api-svc:8118 | [`templates/ingress.yaml`](../deploy/helm/stock/templates/ingress.yaml) | `--set global.ingress.enabled=true` → `kubectl get ingress stock -n stock` |
| **N4** | NetworkPolicy | 5 policy (default-deny + allow-graph) | [`templates/networkpolicy.yaml`](../deploy/helm/stock/templates/networkpolicy.yaml) | `--set global.networkPolicy.enabled=true` → `kubectl get netpol -n stock` |
| **N5** | Endpoints không mồ côi | selector khớp Pod label | tất cả service.yaml | `kubectl get endpoints -n stock` (không có `<none>`) |

**Ghi chú N2:** gateway-svc là điểm vào thật (Rust longest-prefix routing). Ingress là bản k8s-native **song song** để học/demo, cùng route `/`+`/api`.

---

## §4.5 Application Observability and Maintenance (15%)

| # | Yêu cầu | Resource | File | Verify |
|---|---|---|---|---|
| **O1** | Liveness mọi Deployment | `livenessProbe` (httpGet/tcpSocket/grpc) | `charts/<svc>/templates/deployment.yaml` | `kubectl get pod <p> -n stock -o jsonpath='{..livenessProbe}'` |
| **O2** | Readiness mọi Deployment | `readinessProbe` | idem | `kubectl describe pod <p> -n stock \| grep -i readiness` |
| **O3** | Startup probe (slow start) | `startupProbe` prediction-svc (torch ~150s, failureThreshold 30) | `charts/prediction-svc/templates/deployment.yaml` | `kubectl get pod <pred> -n stock -o jsonpath='{..startupProbe}'` |
| **O4** | Debug runbook | mục "Kubernetes / CKAD" | [`README.md`](../README.md#kubernetes--ckad-capstone) | đọc README |
| **O5** | API stable | `networking.k8s.io/v1`, `autoscaling/v2`, `apps/v1` | mọi template | `helm template stock deploy/helm/stock \| grep -i apiVersion \| sort -u` |

---

## Điều kiện auto-fail — trạng thái

| Điều kiện rớt tự động | Trạng thái |
|---|---|
| < 3 service độc lập | ✅ tránh (7 service `*-svc` + db + minio) |
| Chỉ chạy `docker compose`, không có Deployment k8s | ✅ tránh (Helm umbrella đủ Deployment) |
| Secret plaintext commit vào git | ✅ tránh (chỉ placeholder dev; secret thật externalize `values-secret.yaml` gitignore) |
| Không Ingress **và** không NodePort/LB | ✅ tránh (gateway LB + Ingress) |
| Không show Pod Ready trong ns | ✅ `kubectl get pods -n stock` |

---

## Live demo — 9 bước (§6.3)

| # | Bước | Lệnh |
|---|---|---|
| 1 | Pod Running/Ready + Endpoints | `kubectl get pods,svc,endpoints -n stock` |
| 2 | Ingress/gateway hit frontend/API | `curl -H 'Host: stock.local' http://<ingress-ip>/api/version` |
| 3 | ConfigMap/Secret injection | `kubectl exec <api-pod> -c api-svc -n stock -- env \| grep -E 'GRPC_TARGET\|JWT'` |
| 4 | Probe drop endpoint | sửa readiness fail → `kubectl get endpoints api-svc -n stock` (rớt) |
| 5 | Rolling update / blue-green | `kubectl patch svc api-svc -n stock -p '{"spec":{"selector":{"color":"blue"}}}'` |
| 6 | HPA hiện diện | `kubectl get hpa -n stock` |
| 7 | NetworkPolicy allow/deny | pod ngoài graph curl backend → timeout; trong graph → OK |
| 8 | PVC persist | `kubectl delete pod <db> -n stock` → data còn |
| 9 | Helm rollback / Kustomize overlay | `helm rollback stock <rev> -n stock` · `kubectl apply -k deploy/k8s/kustomize/overlays/prod` |

---

*Bám chart tại 2026-08-02. Chart đổi → cập nhật kèm.*
