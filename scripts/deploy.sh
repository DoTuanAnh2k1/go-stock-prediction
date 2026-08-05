#!/usr/bin/env bash
# =============================================================================
# deploy.sh — cài/nâng cấp stack vào namespace K8s (Helm umbrella "stock").
#
#   ./scripts/deploy.sh                       # ns stock, tag dev, toggle demo ON
#   NS=stock RELEASE=stock TAG=v2 ./scripts/deploy.sh
#   DEMO_TOGGLES=0 ./scripts/deploy.sh        # giữ default chart (hpa/ingress/netpol OFF)
#   SECRET_FILE=deploy/helm/stock/values-secret.yaml ./scripts/deploy.sh   # override secret
#
# Tự tạo namespace + db-schema ConfigMap (ngoài Helm) trước khi helm upgrade --install.
# Cần: kubectl (context trỏ cluster đích) + helm v3.
# =============================================================================
set -euo pipefail
cd "$(dirname "$0")/.."

NS="${NS:-stock}"
RELEASE="${RELEASE:-stock}"
TAG="${TAG:-dev}"
CHART="deploy/helm/stock"
HELM="${HELM:-helm}"
command -v "$HELM" >/dev/null 2>&1 || HELM="$HOME/.local/bin/helm"   # helm hay ngoài PATH

echo ">> namespace ${NS}"
kubectl create namespace "$NS" --dry-run=client -o yaml | kubectl apply -f -

echo ">> db-schema ConfigMap (schema init cho db pod — KHÔNG do Helm quản)"
kubectl create configmap db-schema -n "$NS" \
  --from-file=01-schema.sql=database.sql \
  --dry-run=client -o yaml | kubectl apply -f -

ARGS=(upgrade --install "$RELEASE" "$CHART" -n "$NS" --set "global.imageTag=${TAG}")

# Toggle demo-gated ON (P4 HPA, N3 Ingress, N4 NetworkPolicy) để "submitted cluster state"
# có đủ Required. Tắt: DEMO_TOGGLES=0 (Ingress cần ingress-nginx cài trước).
if [ "${DEMO_TOGGLES:-1}" = "1" ]; then
  ARGS+=(--set global.hpa.enabled=true
         --set global.ingress.enabled=true
         --set global.networkPolicy.enabled=true)
fi

# Secret override (production). Nếu file tồn tại → -f; nếu không → dùng placeholder dev.
SECRET_FILE="${SECRET_FILE:-${CHART}/values-secret.yaml}"
if [ -f "$SECRET_FILE" ]; then
  echo ">> override secrets: -f ${SECRET_FILE}"
  ARGS+=(-f "$SECRET_FILE")
else
  echo ">> no ${SECRET_FILE} — dùng placeholder dev trong values.yaml"
fi

echo ">> ${HELM} ${ARGS[*]}"
"$HELM" "${ARGS[@]}"

echo ">> rollout status"
kubectl rollout status deploy -n "$NS" --timeout=180s || true
kubectl get pods,svc -n "$NS"
echo ">> done. verify: ./scripts/smoke-test.sh"
