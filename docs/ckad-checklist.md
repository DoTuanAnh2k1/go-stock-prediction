# CKAD Capstone Checklist — go-stock-prediction

> **Mục đích:** ánh xạ từng mục **Required** trong `deploy/k8s/ckad-labs/capstone-requirements.md` §4 → **resource K8s thật + đường dẫn file + lệnh verify**. Đây là deliverable §6.1 (`docs/ckad-checklist.md`).
>
> **Nguồn triển khai:** các Helm chart per-service độc lập [`deploy/helm/`](../deploy/helm/) — mỗi service 1 chart + library `common` + chart governance `bootstrap` + chart `cronjobs`. Kiến trúc tổng thể xem [DESIGN.md](../DESIGN.md); cài đặt tầng mã xem [IMPLEMENTATION.md](../IMPLEMENTATION.md).
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
helm upgrade api-svc     deploy/helm/api-svc     -n stock --set hpa.enabled=true
helm upgrade gateway-svc deploy/helm/gateway-svc -n stock --set hpa.enabled=true --set ingress.enabled=true
# netpol graph tách 2 chart — set CẢ hai
helm upgrade bootstrap   deploy/helm/bootstrap   -n stock --set networkPolicy.enabled=true
helm upgrade db          deploy/helm/db          -n stock --set networkPolicy.enabled=true
```

Toggle default ON sẵn: `quota`/`rbac`/`pdb` (chart `bootstrap`), `bluegreen` (chart `api-svc`/`web-svc`). pgAdmin nay là cài-hay-không (bỏ `helm install pgadmin` nếu không cần).

---

## §4.1 Application Design and Build (20%)

| # | Yêu cầu | Resource | File | Verify |
|---|---|---|---|---|
| **D1** | Custom image mỗi service | 7 image `*-svc:dev` | [`deploy/*.Dockerfile`](../deploy/) | `docker images \| grep svc` |
| **D2** | Deployment + Job/CronJob | 8 CronJob + Job | [`cronjobs/templates/`](../deploy/helm/cronjobs/templates/) | `kubectl get cronjob,deploy -n stock` |
| **D3** | init + sidecar | init `wait-db` + `nginx` ambassador + `log-sidecar` | `<svc>/templates/deployment.yaml` + [`common/templates/_ambassador.tpl`](../deploy/helm/common/templates/_ambassador.tpl) | `kubectl get pod <p> -n stock -o jsonpath='{.spec.initContainers[*].name} \| {.spec.containers[*].name}'` |
| **D4** | emptyDir chia log | volume `logs` (emptyDir) app↔sidecar | idem deployment.yaml | `kubectl get pod <p> -n stock -o jsonpath='{.spec.volumes[*].name}'` |
| **D5** | PVC bền | `postgres_data`, `backup-data` (2Gi), MinIO PVC | `db/templates/statefulset.yaml`, `cronjobs/`, `minio/` | `kubectl get pvc -n stock` → xóa pod db → data còn |
| **D6** | Label cho blue/green | selector `color: {blue,green}` | `api-svc/templates/bluegreen.yaml`, `web-svc/templates/bluegreen.yaml` | `kubectl get pods -n stock -L color,app` |

**Ghi chú D3:** 5 backend (`api/auth/prediction/service-mgt/cli`) chạy pod ≥4 container (init + app + ambassador nginx + log-sidecar) — vừa init **vừa** sidecar (mục "both preferred").

---

## §4.2 Application Deployment (20%)

| # | Yêu cầu | Resource | File | Verify |
|---|---|---|---|---|
| **P1** | Deployment ≥1 replica | mọi service stateless ≥2 | `<svc>/values.yaml` (`replicas`) | `kubectl get deploy -n stock` |
| **P2** | Rolling update có tài liệu | strategy RollingUpdate | [`ckad-labs/DEMO.md`](../deploy/k8s/ckad-labs/DEMO.md) §2.1 | `kubectl set image deploy/... && kubectl rollout status ...` |
| **P3** | blue/green **hoặc** canary | blue/green flip selector Service chung | `api-svc/templates/bluegreen.yaml` (`activeColor`) | `kubectl patch svc api-svc -n stock -p '{"spec":{"selector":{"color":"blue"}}}'` |
| **P4** | HPA | 2 HPA (api-svc, gateway-svc) CPU 60% | [`api-svc/templates/hpa.yaml`](../deploy/helm/api-svc/templates/hpa.yaml), [`gateway-svc/templates/hpa.yaml`](../deploy/helm/gateway-svc/templates/hpa.yaml) | `helm upgrade api-svc ... --set hpa.enabled=true` + `gateway-svc` → `kubectl get hpa -n stock` |
| **P5** | Kustomize base + overlay | base + overlays dev/prod (patch image+replicas) | [`deploy/k8s/kustomize/`](../deploy/k8s/kustomize/) (api-svc thật) · [`ckad-labs/day_2/kustomize/`](../deploy/k8s/ckad-labs/day_2/kustomize/) (lab) | `kubectl kustomize deploy/k8s/kustomize/overlays/prod` |
| **P6** | Helm upgrade + rollback | per-service chart (vd `api-svc`) | [`deploy/helm/api-svc/`](../deploy/helm/api-svc/) | `helm upgrade api-svc deploy/helm/api-svc -n stock --set imageTag=v2 && helm history api-svc -n stock && helm rollback api-svc <rev> -n stock` |

**Ghi chú P4:** default OFF vì service idle → CPU-HPA chỉ scale khi có traffic thật; khi bật, Deployment omit `.spec.replicas` (HPA sở hữu, tránh flapping).

**Ghi chú P5:** `deploy/k8s/kustomize/` kustomize **image api-svc thật** (base `api-kz` + dev/prod patch `images:` tag & `replicas:`); day_2 lab kustomize nginx demo. Cả hai đều minh hoạ base+overlay+patch.

---

## §4.3 Application Environment, Configuration & Security (25%)

| # | Yêu cầu | Resource | File | Verify |
|---|---|---|---|---|
| **C1** | ConfigMap (env/volume) | ConfigMap mỗi backend + nginx ConfigMap | `<svc>/templates/configmap.yaml` | `kubectl get cm -n stock` |
| **C2** | Secret (credentials) | Secret mỗi backend + db + minio | `<svc>/templates/secret.yaml` | `kubectl get secret -n stock` |
| **C3** | SecurityContext | `runAsNonRoot`, `allowPrivilegeEscalation:false`, drop ALL | `<svc>/templates/deployment.yaml` (`securityContext`) | `kubectl get pod <p> -n stock -o jsonpath='{.spec.containers[0].securityContext}'` |
| **C4** | SA + Role + RoleBinding | `pod-reader` SA/Role/RB (chart `bootstrap`) + 5 SA per-service (mỗi service chart, automount:false) + gateway RBAC | [`bootstrap/templates/rbac.yaml`](../deploy/helm/bootstrap/templates/rbac.yaml), `<svc>/templates/serviceaccount.yaml`, `gateway-svc/templates/rbac.yaml` | `kubectl auth can-i list pods --as=system:serviceaccount:stock:pod-reader -n stock` = yes |
| **C5** | ResourceQuota + LimitRange | `stock-quota` + `stock-defaults` | [`bootstrap/templates/quota.yaml`](../deploy/helm/bootstrap/templates/quota.yaml) | `kubectl get resourcequota,limitrange -n stock` |
| **C6** | requests/limits mọi container | resources trên app + LimitRange vá container thiếu | deployment.yaml + quota.yaml | `kubectl describe resourcequota stock-quota -n stock` |

**Ghi chú C2 (secret & git):** `secrets.*` trong `values.yaml` của mỗi chart chỉ là **placeholder dev** (`change-me-in-dev`, `123`, `minioadmin`) — KHÔNG phải credential thật. Secret production externalize qua file secret riêng từng chart (gitignore, override bằng `-f <chart>-secret.yaml` lúc `helm upgrade`). Không có credential thật commit vào git → không chạm điều kiện auto-fail "Secrets committed in plaintext".

---

## §4.4 Services and Networking (20%)

| # | Yêu cầu | Resource | File | Verify |
|---|---|---|---|---|
| **N1** | ClusterIP nội bộ | Service ClusterIP mọi backend | `<svc>/templates/service.yaml` | `kubectl get svc -n stock` |
| **N2** | NodePort/Ingress | gateway-svc LoadBalancer :80/:443 + Ingress | `gateway-svc/templates/service.yaml`, `gateway-svc/templates/ingress.yaml` | `kubectl get svc gateway-svc -n stock` |
| **N3** | Ingress ≥2 rule | `/` → web-svc:3000, `/api` → api-svc:8118 | [`gateway-svc/templates/ingress.yaml`](../deploy/helm/gateway-svc/templates/ingress.yaml) | `helm upgrade gateway-svc ... --set ingress.enabled=true` → `kubectl get ingress -n stock` |
| **N4** | NetworkPolicy | 5 policy (default-deny + allow-graph) | [`bootstrap/templates/networkpolicy.yaml`](../deploy/helm/bootstrap/templates/networkpolicy.yaml) (default-deny + 3 multi-target), [`db/templates/networkpolicy.yaml`](../deploy/helm/db/templates/networkpolicy.yaml) (allow-backends-to-db) | `helm upgrade bootstrap ... --set networkPolicy.enabled=true` + `db` → `kubectl get netpol -n stock` |
| **N5** | Endpoints không mồ côi | selector khớp Pod label | tất cả service.yaml | `kubectl get endpoints -n stock` (không có `<none>`) |

**Ghi chú N2:** gateway-svc là điểm vào thật (Rust longest-prefix routing). Ingress là bản k8s-native **song song** để học/demo, cùng route `/`+`/api`.

---

## §4.5 Application Observability and Maintenance (15%)

| # | Yêu cầu | Resource | File | Verify |
|---|---|---|---|---|
| **O1** | Liveness mọi Deployment | `livenessProbe` (httpGet/tcpSocket/grpc) | `<svc>/templates/deployment.yaml` | `kubectl get pod <p> -n stock -o jsonpath='{..livenessProbe}'` |
| **O2** | Readiness mọi Deployment | `readinessProbe` | idem | `kubectl describe pod <p> -n stock \| grep -i readiness` |
| **O3** | Startup probe (slow start) | `startupProbe` prediction-svc (torch ~150s, failureThreshold 30) | `prediction-svc/templates/deployment.yaml` | `kubectl get pod <pred> -n stock -o jsonpath='{..startupProbe}'` |
| **O4** | Debug runbook | mục "Kubernetes / CKAD" | [`README.md`](../README.md#kubernetes--ckad-capstone) | đọc README |
| **O5** | API stable | `networking.k8s.io/v1`, `autoscaling/v2`, `apps/v1` | mọi template | `helm template api-svc deploy/helm/api-svc \| grep -i apiVersion \| sort -u` (lặp cho chart khác) |

---

## Điều kiện auto-fail — trạng thái

| Điều kiện rớt tự động | Trạng thái |
|---|---|
| < 3 service độc lập | ✅ tránh (7 service `*-svc` + db + minio) |
| Chỉ chạy `docker compose`, không có Deployment k8s | ✅ tránh (các Helm chart per-service đủ Deployment) |
| Secret plaintext commit vào git | ✅ tránh (chỉ placeholder dev; secret thật externalize file secret per-chart gitignore) |
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
| 9 | Helm rollback / Kustomize overlay | `helm rollback api-svc <rev> -n stock` · `kubectl apply -k deploy/k8s/kustomize/overlays/prod` |

---

*Bám chart tại 2026-08-02. Chart đổi → cập nhật kèm.*
