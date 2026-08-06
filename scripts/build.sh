#!/usr/bin/env bash
# =============================================================================
# build.sh — build & tag 7 app image (<svc>:$TAG) rồi kind load vào cluster.
#
#   ./scripts/build.sh                # build all, tag :dev, load vào kind "ckad"
#   TAG=v2 ./scripts/build.sh         # tag khác (cho demo rolling update / prod overlay)
#   KIND_CLUSTER=ckad ./scripts/build.sh
#   NO_KIND_LOAD=1 ./scripts/build.sh # chỉ build, không load kind
#   ./scripts/build.sh api-svc web-svc   # chỉ build service chỉ định
#
# Build context bám deploy/docker-compose.yaml. Chạy từ repo root.
# =============================================================================
set -euo pipefail
cd "$(dirname "$0")/.."

TAG="${TAG:-1.0.0}"                        # release tag; CI: TAG=$(git rev-parse --short HEAD)
KIND_CLUSTER="${KIND_CLUSTER:-ckad}"

# svc | dockerfile | build-context   (context bám compose: api/prediction = repo root)
SERVICES=(
  "api-svc|deploy/api-svc.Dockerfile|."
  "prediction-svc|deploy/prediction-svc.Dockerfile|."
  "auth-svc|deploy/auth-svc.Dockerfile|auth-svc"
  "service-mgt|deploy/service-mgt.Dockerfile|service-mgt"
  "gateway-svc|deploy/gateway-svc.Dockerfile|gateway-svc"
  "web-svc|deploy/web-svc.Dockerfile|web-svc"
  "cli-svc|deploy/cli-svc.Dockerfile|cli-svc"
)

# Version stamping (parity với make versions / GET /api/version). Defensive nếu không có git.
GIT_SHA="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
BUILD_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
GIT_DIRTY="$(test -n "$(git status --porcelain 2>/dev/null)" && echo true || echo false)"

WANT=("$@")   # optional subset
want() { [ ${#WANT[@]} -eq 0 ] && return 0; for w in "${WANT[@]}"; do [ "$w" = "$1" ] && return 0; done; return 1; }

for entry in "${SERVICES[@]}"; do
  IFS='|' read -r svc dockerfile context <<< "$entry"
  want "$svc" || continue
  echo ">> build ${svc}:${TAG}  (-f ${dockerfile}  ctx ${context})"
  docker build \
    --build-arg "GIT_SHA=${GIT_SHA}" \
    --build-arg "BUILD_TIME=${BUILD_TIME}" \
    --build-arg "GIT_DIRTY=${GIT_DIRTY}" \
    -f "${dockerfile}" -t "${svc}:${TAG}" "${context}"
done

if [ "${NO_KIND_LOAD:-0}" != "1" ] && command -v kind >/dev/null 2>&1; then
  for entry in "${SERVICES[@]}"; do
    IFS='|' read -r svc _ _ <<< "$entry"
    want "$svc" || continue
    echo ">> kind load ${svc}:${TAG} -> ${KIND_CLUSTER}"
    kind load docker-image "${svc}:${TAG}" --name "${KIND_CLUSTER}"
  done
else
  echo ">> skip kind load (NO_KIND_LOAD=${NO_KIND_LOAD:-0}, kind present=$(command -v kind >/dev/null 2>&1 && echo yes || echo no))"
fi
echo ">> done. images tagged :${TAG}"
