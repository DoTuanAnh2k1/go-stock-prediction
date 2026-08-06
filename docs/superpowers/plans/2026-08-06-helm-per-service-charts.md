# Helm per-service charts — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: dispatched as parallel subagents after Phase 0.
> Steps use checkbox (`- [ ]`) syntax. This is infra-as-code (Helm/YAML + docs), so each
> task's "test" is `helm lint` + `helm template` render (not pytest). Verify = render clean
> + parity vs umbrella (git HEAD).

**Goal:** Convert the single umbrella chart `deploy/helm/stock/` into per-service independent
Helm charts under `deploy/helm/<svc>/`, self-contained values, cross-cutting distributed.

**Architecture:** 12 app/infra charts + `common` (library) + `bootstrap` (ns governance) +
`cronjobs`, each `helm install`-able standalone. `common` shared via `file://../common`
dependency. No cluster apply this round — verify by render/parity.

**Tech Stack:** Helm v3 (`~/.local/bin/helm`), kubectl, bash.

## Global Constraints

- Namespace `stock`. helm binary at `~/.local/bin/helm` (not on Bash PATH — use full path).
- `common` library keeps `type: library`, name `common`, version `0.1.0`, template names
  `stock.*` unchanged (they take image via `dict "image" ...`, no `.Values.global` refs).
- Every `.Values.global.X` → `.Values.X`; each chart's `values.yaml` holds ONLY keys it uses.
- Per-service ServiceAccount created **unconditionally** in its chart (`automountServiceAccountToken: false`);
  deployments already set `serviceAccountName: <svc>`. `rbac.enabled` (bootstrap) gates ONLY pod-reader.
- `db-schema` ConfigMap stays manual (out of Helm).
- NO `helm install`/`upgrade` to cluster. Verify with `helm lint` + `helm template` only.
- Grep gate at the end: zero `--set global.` / `.Values.global.` outside spec & this plan.

---

## Phase 0 — Structural moves (done by coordinator, sequential — NOT parallel)

### Task 0: Relocate charts + scaffold bootstrap + remove umbrella

**Files:**
- `git mv deploy/helm/stock/charts/<svc>` → `deploy/helm/<svc>` for: common, db, minio,
  service-mgt, prediction-svc, auth-svc, api-svc, gateway-svc, web-svc, cli-svc, pgadmin, cronjobs.
- Create: `deploy/helm/bootstrap/{Chart.yaml,values.yaml,templates/}`.
- Move cross-cutting sources into bootstrap: `stock/templates/{quota,serviceaccounts,pdb,networkpolicy}.yaml`.
- Delete: `deploy/helm/stock/` remainder (`Chart.yaml`, `values.yaml`, `values-secret.yaml.example`,
  `templates/{hpa,ingress,NOTES.txt}` — hpa→api/gateway, ingress→gateway handled in Phase 1;
  NOTES→bootstrap NOTES).

- [ ] **Step 1:** `git mv` each subchart dir out to `deploy/helm/<svc>/`.
- [ ] **Step 2:** Create `deploy/helm/bootstrap/Chart.yaml` (`type: application`, name bootstrap, v0.1.0).
- [ ] **Step 3:** `git mv stock/templates/quota.yaml deploy/helm/bootstrap/templates/quota.yaml` (same for serviceaccounts→rename `rbac.yaml`, pdb, networkpolicy). Keep `stock/templates/hpa.yaml` + `ingress.yaml` aside for Phase 1 (gateway/api).
- [ ] **Step 4:** Delete `deploy/helm/stock/` (Chart.yaml, values.yaml, values-secret.example, remaining templates, NOTES).
- [ ] **Step 5:** Verify tree: `find deploy/helm -maxdepth 1 -type d` shows the 14 dirs, no `stock`.
- [ ] **Step 6:** Commit `refactor(helm): relocate subcharts to top-level, scaffold bootstrap`.

After Phase 0, dispatch Phase 1 tracks A/B/C + Phase 2 D/E in parallel.

---

## Phase 1 — Per-chart value migration (parallel tracks A, B, C)

Each chart: (a) rewrite `.Values.global.X`→`.Values.X` in all templates; (b) write a
self-contained `values.yaml` with only its keys (see spec table); (c) add SA where backend;
(d) add distributed cross-cutting. Verify each: `~/.local/bin/helm lint deploy/helm/<svc>` +
`~/.local/bin/helm template <svc> deploy/helm/<svc> -n stock`. Backend charts (api/auth/
prediction/service-mgt/cli) also add `common` dependency + run `helm dependency build`.

### Task A: api-svc + gateway-svc + web-svc

**Files:**
- `deploy/helm/api-svc/{Chart.yaml,values.yaml,templates/{deployment,bluegreen,service,configmap,secret,serviceaccount,hpa}.yaml}`
- `deploy/helm/gateway-svc/{Chart.yaml,values.yaml,templates/{deployment,service,configmap,rbac,hpa,ingress}.yaml}`
- `deploy/helm/web-svc/{Chart.yaml,values.yaml,templates/{deployment,bluegreen,service}.yaml}`

**Interfaces (Produces):** command form `--set imageTag=`, `--set bluegreen.enabled=`,
`--set bluegreen.activeColor=`, `--set hpa.enabled=`, `--set ingress.enabled=` — Track D/E rely on these.

- [ ] **Step 1:** api-svc: replace `.Values.global.imageTag`→`.Values.imageTag`,
  `.Values.global.secrets.*`→`.Values.secrets.*`, `.Values.global.bluegreen.*`→`.Values.bluegreen.*`,
  `.Values.global.hpa.enabled`→`.Values.hpa.enabled` in deployment.yaml/bluegreen.yaml/service.yaml/secret.yaml.
  Add `common` dep to Chart.yaml (`repository: "file://../common"`).
- [ ] **Step 2:** api-svc: create `templates/serviceaccount.yaml` (SA `api-svc`, automount:false, unconditional).
- [ ] **Step 3:** api-svc: create `templates/hpa.yaml` (from `stock/templates/hpa.yaml` — keep ONLY api-svc +
  blue/green HPA block; `.Values.hpa.apiSvc.*`, `.Values.hpa.enabled`, `.Values.bluegreen.enabled` for target list).
- [ ] **Step 4:** api-svc: write `values.yaml`: `replicas: 2`, `imageTag: dev`, `image: api-svc`,
  `secrets:{postgresPassword,jwtSecret,internalSecret,adminPassword}` (dev placeholders),
  `bluegreen:{enabled: true, activeColor: green}`, `hpa:{enabled: false, apiSvc:{minReplicas:2,maxReplicas:6,targetCPU:60}}`.
- [ ] **Step 5:** gateway-svc: same global→local rewrite; create `templates/hpa.yaml` (gateway block only) +
  `templates/ingress.yaml` (from `stock/templates/ingress.yaml`, `.Values.ingress.{enabled,className,host}`).
  `values.yaml`: `replicas:2, imageTag, image: gateway-svc, bluegreen:{enabled:true}, hpa:{enabled:false,gatewaySvc:{minReplicas:2,maxReplicas:5,targetCPU:60}}, ingress:{enabled:false,className:nginx,host:stock.local}`.
- [ ] **Step 6:** web-svc: rewrite `.Values.global.{imageTag,bluegreen.*}`→local; `values.yaml`:
  `replicas:2, imageTag, image: web-svc, bluegreen:{enabled:true,activeColor:green}`.
- [ ] **Step 7:** `helm dependency build deploy/helm/api-svc`; `helm lint` + `helm template` all three (with and without `--set hpa.enabled=true` for api/gateway, `--set ingress.enabled=true` for gateway). Expect clean render, HPA/Ingress appear only when toggled.
- [ ] **Step 8:** Commit `refactor(helm): api-svc/gateway-svc/web-svc self-contained charts + distributed hpa/ingress`.

### Task B: auth-svc + prediction-svc + service-mgt + cli-svc

**Files:** each `deploy/helm/<svc>/{Chart.yaml,values.yaml,templates/{deployment,service,configmap,secret,serviceaccount[,pvc]}.yaml}`

- [ ] **Step 1:** For each: rewrite `.Values.global.imageTag`→`.Values.imageTag`,
  `.Values.global.secrets.*`→`.Values.secrets.*` in deployment.yaml + secret.yaml. Add `common` dep to Chart.yaml.
- [ ] **Step 2:** For each: create `templates/serviceaccount.yaml` (SA `<svc>`, automount:false, unconditional).
  (These were in central `serviceaccounts.yaml`; now per-chart. prediction-svc keeps its pvc.yaml.)
- [ ] **Step 3:** Write `values.yaml` per spec table:
  auth-svc `secrets:{postgresPassword,jwtSecret,internalSecret}`;
  prediction-svc `replicas:2, secrets:{postgresPassword,s3AccessKey,s3SecretKey}`;
  service-mgt `secrets:{postgresPassword}`; cli-svc `secrets:{internalSecret}`. All + `replicas, imageTag, image`.
- [ ] **Step 4:** `helm dependency build` each; `helm lint` + `helm template` each. Expect SA rendered, no `global.` errors.
- [ ] **Step 5:** Commit `refactor(helm): auth/prediction/service-mgt/cli self-contained charts + own SA`.

### Task C: db + minio + pgadmin + cronjobs + bootstrap

**Files:**
- `deploy/helm/db/{Chart.yaml,values.yaml,templates/{statefulset,service,secret,networkpolicy}.yaml}`
- `deploy/helm/minio/`, `deploy/helm/pgadmin/`, `deploy/helm/cronjobs/`
- `deploy/helm/bootstrap/{Chart.yaml,values.yaml,templates/{quota,rbac,pdb,networkpolicy,NOTES.txt}.yaml}`

- [ ] **Step 1:** db: rewrite `.Values.global.secrets.{postgresUser,postgresPassword,postgresDb}`→local in secret.yaml.
  Create `templates/networkpolicy.yaml` = the `allow-backends-to-db` policy only, gated `if .Values.networkPolicy.enabled`.
  `values.yaml`: `secrets:{postgresUser:postgres,postgresPassword:"123",postgresDb:go_stock_prediction}, networkPolicy:{enabled:false}`.
- [ ] **Step 2:** minio: rewrite `.Values.global.secrets.{minioRootUser,minioRootPassword}`→local; `values.yaml`: `secrets:{minioRootUser,minioRootPassword}`.
- [ ] **Step 3:** pgadmin: DROP `if .Values.global.pgadmin.enabled` gates (install-or-not replaces the toggle);
  rewrite `.Values.global.secrets.pgadminPassword`→`.Values.secrets.pgadminPassword`; `values.yaml`: `imageTag, image, secrets:{pgadminPassword:admin}`.
- [ ] **Step 4:** cronjobs: rewrite `.Values.global.imageTag`→`.Values.imageTag`,
  `.Values.global.manualJob.enabled`→`.Values.manualJob.enabled` in all cronjob-*.yaml + job-manual.yaml.
  `values.yaml`: `imageTag: dev, image: prediction-svc, manualJob:{enabled:false}`.
- [ ] **Step 5:** bootstrap: in moved `quota.yaml` `rbac.yaml`(was serviceaccounts) `pdb.yaml` `networkpolicy.yaml`:
  rewrite `.Values.global.{quota,rbac,pdb,pgadmin,networkPolicy}.enabled`→`.Values.{...}.enabled`.
  In `rbac.yaml`: REMOVE the per-service SA `range` block (moved to service charts); keep ONLY pod-reader SA/Role/RoleBinding gated `rbac.enabled`.
  In `networkpolicy.yaml`: REMOVE `allow-backends-to-db` (moved to db chart); keep default-deny + the 3 multi-target policies.
  In `pdb.yaml`: keep as-is but `.Values.pgadmin.enabled`→`.Values.pgadmin.enabled` (add `pgadmin:{enabled:true}` to bootstrap values so pgadmin PDB still renders).
  Write bootstrap `values.yaml`: `quota:{enabled:true}, rbac:{enabled:true}, pdb:{enabled:true}, networkPolicy:{enabled:false}, pgadmin:{enabled:true}`.
  Write bootstrap `templates/NOTES.txt` (ns pre-req: db-schema ConfigMap; install order).
- [ ] **Step 6:** `helm lint` + `helm template` each of db/minio/pgadmin/cronjobs/bootstrap
  (bootstrap with and without `--set networkPolicy.enabled=true`, `--set quota.enabled=false`). Expect clean.
- [ ] **Step 7:** Commit `refactor(helm): infra charts self-contained + bootstrap ns-governance + db netpol`.

---

## Phase 2 — Orchestration + docs (parallel tracks D, E) — depend on Phase 1 conventions (fixed by spec)

### Task D: scripts + Makefile

**Files:** `scripts/deploy.sh`, `scripts/run-labs.sh`, `Makefile`

- [ ] **Step 1:** Rewrite `scripts/deploy.sh`: keep ns + db-schema bootstrap; add
  `helm dependency build` loop for api/auth/prediction/service-mgt/cli; install order
  `bootstrap db minio service-mgt prediction-svc auth-svc api-svc gateway-svc web-svc cli-svc pgadmin cronjobs`,
  each `helm upgrade --install <n> deploy/helm/<n> -n $NS --set imageTag=$TAG [-f secret if exists]`.
  `DEMO_TOGGLES=1`: `api-svc,gateway-svc --set hpa.enabled=true`; `gateway-svc --set ingress.enabled=true`;
  `bootstrap,db --set networkPolicy.enabled=true`. Update header comments.
- [ ] **Step 2:** Rewrite `scripts/run-labs.sh` helm section: `CHART` removed; netpol toggle now
  `helm upgrade bootstrap deploy/helm/bootstrap -n stock --set networkPolicy.enabled=$1` + same for db.
- [ ] **Step 3:** Add `Makefile` target `helm-deps:` running `helm dependency build` for the 5 backend charts.
- [ ] **Step 4:** `bash -n scripts/deploy.sh scripts/run-labs.sh` (syntax check); dry-run deploy.sh with
  `HELM=echo` if feasible to confirm command forms. Commit `refactor(helm): per-chart deploy.sh/run-labs.sh + make helm-deps`.

### Task E: docs + labs

**Files:** `README.md`, `CLAUDE.md`, `docs/claude/directory-structure.md`, `docs/claude/database.md`,
`deploy/helm/README.md`, `deploy/k8s/kustomize/README.md`, `VERIFY.md`, `docs/ckad-checklist.md`,
`deploy/k8s/ckad-labs/capstone-requirements.md`, `deploy/k8s/ckad-labs/day_{2,3,4,5}/lab.md`,
`deploy/k8s/ckad-labs/day_{2,4,5}/run-day{2,4,5}.sh`.

- [ ] **Step 1:** Replace every `helm ... stock deploy/helm/stock ... --set global.X` with per-chart form
  (`helm upgrade <svc> deploy/helm/<svc> -n stock --set X`). Map: `global.hpa.enabled`→api-svc+gateway-svc `hpa.enabled`;
  `global.ingress.enabled`→gateway-svc `ingress.enabled`; `global.networkPolicy.enabled`→bootstrap+db `networkPolicy.enabled`;
  `global.quota.enabled`→bootstrap; `global.imageTag`→per chart `imageTag`; `global.bluegreen.*`→api-svc/web-svc.
- [ ] **Step 2:** Update the umbrella/subchart narrative in CLAUDE.md + directory-structure.md +
  deploy/helm/README.md to describe per-service independent charts + `common` dep + `bootstrap` + install order.
- [ ] **Step 3:** VERIFY.md §4 checklist + ckad-checklist.md: update helm commands. capstone-requirements.md P6:
  reflect per-service chart lifecycle (upgrade/rollback on one chart).
- [ ] **Step 4:** Commit `docs(helm): per-service chart commands across labs/README/CLAUDE/VERIFY`.

---

## Phase 3 — Final verification (coordinator, sequential)

- [ ] **Step 1:** `for c in bootstrap common db minio service-mgt prediction-svc auth-svc api-svc gateway-svc web-svc cli-svc pgadmin cronjobs; do ~/.local/bin/helm lint deploy/helm/$c; done` (common lint may warn library — OK).
- [ ] **Step 2:** `helm template` each chart (default + toggled) — capture resource kinds; compare set of
  Deployments/Services/Secrets/CronJobs/PDBs/netpol/quota/SA/HPA/Ingress vs `git show HEAD:...` umbrella render.
  Confirm no resource dropped (label/managed-by/release-name diffs acceptable).
- [ ] **Step 3:** `grep -rn "global\." --include=*.yaml deploy/helm | grep -v observability` → expect empty
  (no leftover `.Values.global`). `grep -rn "\-\-set global\.\|deploy/helm/stock" README.md CLAUDE.md VERIFY.md docs scripts deploy/k8s` → expect empty.
- [ ] **Step 4:** Commit any fixups. Report parity result. (No cluster apply — user applies later.)

## Self-review notes

- Spec coverage: layout✓(0), self-contained values✓(A/B/C), common dep✓(A/B), cross-cutting
  table✓(A: hpa/ingress/SA; C: quota/rbac/pdb/netpol; db netpol✓C), deploy orchestration✓(D),
  docs/labs✓(E), verification✓(Phase 3).
- pgadmin/manualJob toggles: pgadmin `enabled` becomes install-or-not; bootstrap still needs
  `pgadmin.enabled` for the pgadmin PDB → kept in bootstrap values.
- No cluster apply anywhere. helm full path used in verify steps.
