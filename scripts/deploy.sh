#!/usr/bin/env bash
# =============================================================================
# deploy.sh — cài/nâng cấp stack vào namespace K8s, MỘT chart Helm độc lập / service.
#
# Umbrella "stock" đã BỎ. Mỗi service = 1 chart top-level deploy/helm/<svc>/, cài
# lần lượt theo thứ tự phụ thuộc. 5 backend chart (api/auth/prediction/service-mgt/
# cli) mang dependency library `common` (file://../common) → chạy `helm dependency
# build` trước khi install (vendor common vào charts/).
#
#   ./scripts/deploy.sh                       # ns stock, tag dev, toggle demo ON
#   NS=stock TAG=v2 ./scripts/deploy.sh
#   DEMO_TOGGLES=0 ./scripts/deploy.sh        # giữ default chart (hpa/ingress/netpol OFF)
#   SECRET_FILE=deploy/helm/secrets.yaml ./scripts/deploy.sh   # override secrets (-f)
#
# Tự tạo namespace + db-schema ConfigMap (ngoài Helm) trước khi helm upgrade --install.
# Cần: kubectl (context trỏ cluster đích) + helm v3.
#
# Vòng đời độc lập per-service — upgrade/rollback MỘT chart, ví dụ:
#   helm upgrade api-svc deploy/helm/api-svc -n stock --set imageTag=v2
#   helm history api-svc -n stock
#   helm rollback api-svc 1 -n stock
# =============================================================================
set -euo pipefail
cd "$(dirname "$0")/.."

NS="${NS:-stock}"
TAG="${TAG:-1.0.0}"                        # release tag; CI: TAG=$(git rev-parse --short HEAD)
HELM="${HELM:-helm}"
command -v "$HELM" >/dev/null 2>&1 || HELM="$HOME/.local/bin/helm"   # helm hay ngoài PATH

# Thứ tự cài (phụ thuộc): governance ns → db → object store → backend → edge → batch.
CHARTS=(bootstrap db minio service-mgt prediction-svc auth-svc api-svc gateway-svc web-svc cli-svc pgadmin cronjobs)

# Backend chart mang dependency `common` — cần vendor trước khi install.
COMMON_DEPS=(api-svc auth-svc prediction-svc service-mgt cli-svc)

# Chart có key `imageTag` trong values (nhận --set imageTag). bootstrap/db/minio/
# pgadmin không build từ image app → không nhận imageTag.
has_imagetag() {
  case "$1" in
    service-mgt|prediction-svc|auth-svc|api-svc|gateway-svc|web-svc|cli-svc|cronjobs) return 0 ;;
    *) return 1 ;;
  esac
}

# Chart có block secrets (nhận -f secret override).
has_secrets() {
  case "$1" in
    db|minio|service-mgt|prediction-svc|auth-svc|api-svc|cli-svc|pgadmin) return 0 ;;
    *) return 1 ;;
  esac
}

# DEMO_TOGGLES=1 (default): bật Required demo-gated (P4 HPA, N3 Ingress, N4 NetworkPolicy)
# để "submitted cluster state" có đủ. Rải per-chart:
#   api-svc,gateway-svc  --set hpa.enabled=true
#   gateway-svc          --set ingress.enabled=true   (cần ingress-nginx cài trước)
#   bootstrap,db         --set networkPolicy.enabled=true (netpol split → phải cả hai)
# Tắt: DEMO_TOGGLES=0.
demo_args() {  # $1 = chart name; echo extra --set flags
  [ "${DEMO_TOGGLES:-1}" = "1" ] || return 0
  case "$1" in
    api-svc)     echo "--set hpa.enabled=true" ;;
    gateway-svc) echo "--set hpa.enabled=true --set ingress.enabled=true" ;;
    bootstrap)   echo "--set networkPolicy.enabled=true" ;;
    db)          echo "--set networkPolicy.enabled=true" ;;
  esac
}

echo ">> namespace ${NS}"
kubectl create namespace "$NS" --dry-run=client -o yaml | kubectl apply -f -

echo ">> db-schema ConfigMap (schema init cho db pod — KHÔNG do Helm quản)"
kubectl create configmap db-schema -n "$NS" \
  --from-file=01-schema.sql=database.sql \
  --dry-run=client -o yaml | kubectl apply -f -

echo ">> helm dependency build (vendor 'common' vào backend chart)"
for c in "${COMMON_DEPS[@]}"; do
  echo "   - deploy/helm/${c}"
  "$HELM" dependency build "deploy/helm/${c}"
done

# Secret override (production). Nếu file tồn tại → -f cho chart có secrets; nếu
# không → dùng placeholder dev trong values.yaml mỗi chart.
SECRET_FILE="${SECRET_FILE:-deploy/helm/secrets.yaml}"
if [ -f "$SECRET_FILE" ]; then
  echo ">> override secrets: -f ${SECRET_FILE} (áp cho chart có secrets)"
else
  echo ">> no ${SECRET_FILE} — dùng placeholder dev trong values.yaml"
fi

for c in "${CHARTS[@]}"; do
  ARGS=(upgrade --install "$c" "deploy/helm/${c}" -n "$NS")
  has_imagetag "$c" && ARGS+=(--set "imageTag=${TAG}")
  if [ -f "$SECRET_FILE" ] && has_secrets "$c"; then
    ARGS+=(-f "$SECRET_FILE")
  fi
  # shellcheck disable=SC2046
  ARGS+=($(demo_args "$c"))
  echo ">> ${HELM} ${ARGS[*]}"
  "$HELM" "${ARGS[@]}"
done

echo ">> rollout status"
kubectl rollout status deploy -n "$NS" --timeout=180s || true
kubectl get pods,svc -n "$NS"
echo ">> done. verify: ./scripts/smoke-test.sh"
