# Helm: umbrella → per-service independent charts

- **Date:** 2026-08-06
- **Status:** Design (approved) — implement code trước, apply cluster sau
- **Bối cảnh:** hiện `deploy/helm/stock/` là umbrella chart (1 chart cho toàn system,
  12 subchart trong `charts/`, giá trị chung ở `global:`, library `common`, cross-cutting
  ns-scoped trong `stock/templates/`). CKAD labs (day 2/3/4/5), `VERIFY.md §4`,
  `scripts/{deploy,run-labs}.sh` đều drive qua `helm ... stock deploy/helm/stock --set global.*`.

## Mục tiêu

Đổi thành **mỗi service = 1 Helm chart độc lập**, cài/upgrade/rollback riêng từng chart.
Bỏ hẳn umbrella `stock`. Đây cũng thỏa capstone **P6** ("chart own/wrapped, installable
with values override, upgrade + rollback — may package one critical service") rõ hơn umbrella.

## Quyết định (đã chốt với user)

1. **Bỏ umbrella hoàn toàn** — mỗi service thành chart top-level `deploy/helm/<svc>/`,
   `helm install <svc> deploy/helm/<svc> -n stock`.
2. **Self-contained values** — mỗi chart tự chứa values (imageTag, image, secrets nó cần,
   toggle riêng). KHÔNG còn `global:` chung, KHÔNG file values chung. Chấp nhận duplication
   của `imageTag`/`secrets` giữa các chart.
3. **Cross-cutting rải vào chart service liên quan**; ns-global singleton (quota, pod-reader
   RBAC, PDB, default-deny + multi-target netpol) gom vào 1 chart nhỏ `bootstrap`.
4. **NetworkPolicy rải theo podSelector target** — policy single-target về chart của pod nó
   bảo vệ; default-deny + policy multi-target ở `bootstrap`.
5. **Thi hành:** spec → writing-plans → subagent song song. **Không apply cluster** (verify
   bằng `helm lint` + `helm template` render sạch, diff vs umbrella hiện tại).

## Target layout (`deploy/helm/`)

```
deploy/helm/
  common/          library chart (type: library) — partial ambassador/sidecar/HA (giữ nguyên _ambassador.tpl, tên template stock.*)
  bootstrap/       ns governance (cài ĐẦU TIÊN): quota, pod-reader RBAC, PDBs, netpol (default-deny + multi-target)
  db/              + netpol allow-backends-to-db (single-target)
  minio/
  service-mgt/     + ServiceAccount riêng
  prediction-svc/  + ServiceAccount riêng
  auth-svc/        + ServiceAccount riêng
  api-svc/         + ServiceAccount riêng + HPA (api + blue/green) + bluegreen
  gateway-svc/     + gateway SA/Role (đã có) + HPA gateway + Ingress
  web-svc/         + bluegreen
  cli-svc/         + ServiceAccount riêng
  pgadmin/
  cronjobs/        batch: cronjob prediction (jobs_cli) + db backup
  observability/   (đã độc lập — KHÔNG đụng)
```

Mỗi chart `Chart.yaml` là `type: application`, `version 0.1.0`. Không còn `dependencies`
liệt kê subchart trong umbrella.

## Self-contained values — migration `.Values.global.X` → `.Values.X`

Mỗi chart giữ ĐÚNG các key nó dùng (dựa trên grep `.Values.global.*` hiện tại):

| Chart | Keys trong values.yaml |
|---|---|
| api-svc | `replicas`, `imageTag`, `image: api-svc`, `secrets:{postgresPassword,jwtSecret,internalSecret,adminPassword}`, `bluegreen:{enabled,activeColor}`, `hpa:{enabled,apiSvc:{min,max,targetCPU}}`, `serviceAccount:{create}` |
| auth-svc | `replicas`, `imageTag`, `image`, `secrets:{postgresPassword,jwtSecret,internalSecret}`, `serviceAccount:{create}` |
| prediction-svc | `replicas`, `imageTag`, `image`, `secrets:{postgresPassword,s3AccessKey,s3SecretKey}`, `serviceAccount:{create}` |
| service-mgt | `replicas`, `imageTag`, `image`, `secrets:{postgresPassword}`, `serviceAccount:{create}` |
| cli-svc | `replicas`, `imageTag`, `image`, `secrets:{internalSecret}`, `serviceAccount:{create}` |
| gateway-svc | `replicas`, `imageTag`, `image`, `hpa:{enabled,gatewaySvc:{...}}`, `bluegreen:{enabled}` (đọc để route/HPA target), `ingress:{enabled,className,host}` |
| web-svc | `replicas`, `imageTag`, `image`, `bluegreen:{enabled,activeColor}` |
| db | `secrets:{postgresUser,postgresPassword,postgresDb}` |
| minio | `secrets:{minioRootUser,minioRootPassword}` |
| pgadmin | `secrets:{pgadminPassword}` (cài-hay-không thay cho `pgadmin.enabled`) |
| cronjobs | `imageTag`, `image: prediction-svc`, `manualJob:{enabled}` |
| bootstrap | `quota:{enabled}`, `rbac:{enabled}` (pod-reader), `pdb:{enabled}`, `networkPolicy:{enabled}` |

`--set global.imageTag=v2` (umbrella) → `--set imageTag=v2` (mỗi chart). `--set
global.hpa.enabled=true` → `--set hpa.enabled=true` (api-svc + gateway-svc). v.v.

`common/_ambassador.tpl` KHÔNG tham chiếu `.Values.global` (nhận image qua `dict "image"
...`) nên nội dung giữ nguyên; chỉ cách include (dependency) đổi — xem dưới.

## Library `common`

Giữ MỘT `deploy/helm/common/` (source of truth cho partial ambassador). Mỗi backend chart
(api/auth/prediction/service-mgt/cli) khai trong `Chart.yaml`:

```yaml
dependencies:
  - name: common
    version: 0.1.0
    repository: "file://../common"
```

Vendored 1 lần bằng `helm dependency build deploy/helm/<svc>` (copy vào `charts/`). Chạy trong
`scripts/deploy.sh` + `make helm-deps`. Chart vẫn install standalone sau build.
(Phương án thay thế — copy vật lý `common/` vào 5 chart — LOẠI vì maintain 5 bản.)

## Cross-cutting distribution (final)

| Resource | Home mới | Toggle |
|---|---|---|
| ServiceAccount per-service (api/auth/prediction/service-mgt/cli, automount:false) | **mỗi service chart** (tự tạo + set `serviceAccountName` trên deployment) | `serviceAccount.create` |
| gateway SA + Role `gateway-bluegreen` + ConfigMap state | **gateway-svc** (đã có `rbac.yaml`) | — |
| HPA api-svc (+ blue/green target) | **api-svc** | `hpa.enabled` |
| HPA gateway-svc | **gateway-svc** | `hpa.enabled` |
| Ingress `/`→web-svc:3000, `/api`→api-svc:8118 | **gateway-svc** (chart edge/routing) | `ingress.enabled` |
| LimitRange `stock-defaults` + ResourceQuota `stock-quota` | **bootstrap** | `quota.enabled` |
| pod-reader SA + Role + RoleBinding | **bootstrap** | `rbac.enabled` |
| PDB (mọi service + db + minio + pgadmin) | **bootstrap** | `pdb.enabled` |
| netpol `default-deny-ingress` (ns-wide) | **bootstrap** | `networkPolicy.enabled` |
| netpol `allow-gateway-to-frontend` (api-svc+web-svc, multi-target) | **bootstrap** | `networkPolicy.enabled` |
| netpol `allow-api-to-backends` (auth-svc+prediction-svc, multi-target) | **bootstrap** | `networkPolicy.enabled` |
| netpol `deny-backend-egress-internet` (auth-svc+service-mgt, multi-target) | **bootstrap** | `networkPolicy.enabled` |
| netpol `allow-backends-to-db` (target db, single) | **db** | `networkPolicy.enabled` |

Netpol single-target duy nhất tách được là `allow-backends-to-db` → chart db. default-deny
là ns-wide, 3 policy còn lại multi-target → ở bootstrap. Khi bật demo netpol phải set
`--set networkPolicy.enabled=true` cho CẢ bootstrap và db (deploy.sh lo).

`db-schema` ConfigMap vẫn tạo tay (`kubectl create configmap db-schema`), ngoài Helm — giữ
nguyên, note trong deploy.sh + NOTES của bootstrap.

## Deploy orchestration

Rewrite `scripts/deploy.sh`:
1. tạo namespace + `db-schema` ConfigMap (như cũ).
2. `helm dependency build` cho 5 backend chart (vendor `common`).
3. `helm upgrade --install <name> deploy/helm/<name> -n stock --set imageTag=$TAG [-f secret]`
   theo THỨ TỰ: `bootstrap → db → minio → service-mgt → prediction-svc → auth-svc → api-svc
   → gateway-svc → web-svc → cli-svc → pgadmin → cronjobs`.
4. `DEMO_TOGGLES=1` map sang per-chart `--set`: `api-svc,gateway-svc --set hpa.enabled=true`;
   `gateway-svc --set ingress.enabled=true`; `bootstrap,db --set networkPolicy.enabled=true`.
5. Secret override `-f` áp cho các chart có secret.

`make helm-deps` = chạy `helm dependency build` toàn bộ backend chart.

**Upgrade/rollback demo (P6):** trên MỘT chart, vd
`helm upgrade api-svc deploy/helm/api-svc -n stock --set imageTag=v2` → `helm history api-svc`
→ `helm rollback api-svc 1`. Vòng đời độc lập per-service = điểm ăn tiền của refactor.

## Docs / labs blast radius (đổi mọi lệnh `helm ... stock ... --set global.X`)

- `scripts/deploy.sh`, `scripts/run-labs.sh`
- `deploy/k8s/ckad-labs/day_{2,3,4,5}/lab.md` + `run-day{2,3,4,5}.sh`
- `VERIFY.md` (§4 checklist), `docs/ckad-checklist.md`
- `README.md`, `CLAUDE.md`, `docs/claude/directory-structure.md`, `docs/claude/database.md`
- `deploy/helm/README.md`, `deploy/k8s/kustomize/README.md`
- `capstone-requirements.md` (nhẹ — P6 wording), `IMPLEMENTATION.md`/`DESIGN.md` (nếu có ref)

Mapping lệnh: `helm upgrade stock deploy/helm/stock -n stock --set global.hpa.enabled=true`
→ `helm upgrade api-svc deploy/helm/api-svc -n stock --set hpa.enabled=true` (+ gateway-svc).
`helm template stock deploy/helm/stock` → per-chart hoặc script wrapper render tất cả.

## Verification (apply SAU)

- `helm lint deploy/helm/<chart>` sạch cho từng chart.
- `helm template <chart> deploy/helm/<chart> -n stock [--set ...]` render OK.
- **Parity:** so object đã parse của per-chart render vs umbrella render (git HEAD) — cùng
  Deployment/Service/Secret/CronJob/PDB/netpol/quota/SA/HPA/Ingress (chỉ khác label
  `app.kubernetes.io/managed-by` / release name — chấp nhận). Xác nhận KHÔNG mất resource nào.
- KHÔNG `helm install`/`upgrade` vào cluster trong đợt này.

## Out of scope

- `deploy/helm/observability/` (đã độc lập).
- `docker-compose` / ofelia (không liên quan Helm).
- Đổi image/app code. Chỉ đổi packaging Helm + docs/labs.

## Risks

- **Values drift** (imageTag/secrets lặp nhiều chart) — hệ quả trực tiếp của self-contained
  (user chấp nhận); deploy.sh truyền `imageTag` đồng nhất giảm rủi ro.
- **common dependency build** — quên `helm dependency build` → backend chart thiếu partial →
  render fail. deploy.sh + `make helm-deps` bao; note rõ trong README.
- **netpol phân mảnh** — bật netpol phải set cho cả bootstrap + db; nếu chỉ 1 → graph khuyết
  (default-deny bật mà thiếu allow-db → db treo). deploy.sh set cùng lúc; ghi chú lab.
- **Lệnh cũ trong doc/lab** còn sót `--set global.*` → verify bằng grep chốt `global.` = 0
  ngoài spec/lịch sử.
