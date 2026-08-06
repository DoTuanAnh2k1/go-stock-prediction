#!/usr/bin/env bash
# =============================================================================
# run-labs.sh — chạy các bài lab CKAD (day 1–5) từ REPO ROOT cho dễ tìm.
# Run the CKAD day labs (1–5) from the repo root.
#
#   ./scripts/run-labs.sh            # tất cả 5 ngày / all 5 days
#   ./scripts/run-labs.sh 3          # chỉ 1 ngày / a single day (1..5)
#   KEEP_NETPOL=1 ./scripts/run-labs.sh   # đừng đụng NetworkPolicy toggle
#
# NetworkPolicy: lab thiết kế với NetPol TẮT (day2 zero-downtime cần); day4 tự
# bật→demo→tắt. Wrapper này TẮT NetPol trước khi chạy rồi BẬT LẠI ở cuối (trạng
# thái capstone). The labs run with NetPol OFF; it is re-enabled at the end.
#
# Yêu cầu / Requires: cluster kind `ckad` đang chạy, ns `stock` đã deploy (scripts/deploy.sh).
# =============================================================================
set -uo pipefail
cd "$(dirname "$0")/.."
LABS="deploy/k8s/ckad-labs"

HELM="${HELM:-helm}"
command -v "$HELM" >/dev/null 2>&1 || HELM="$HOME/.local/bin/helm"

# Umbrella "stock" đã bỏ — NetworkPolicy graph tách giữa 2 chart: bootstrap giữ
# default-deny + 3 policy multi-target, db giữ allow-backends-to-db (single-target).
# Toggle phải áp CHO CẢ HAI, nếu không graph khuyết (bật default-deny mà thiếu
# allow-db → db treo).
netpol() {  # $1 = true|false
  [ "${KEEP_NETPOL:-0}" = "1" ] && return 0
  "$HELM" upgrade bootstrap deploy/helm/bootstrap -n stock \
    --set "networkPolicy.enabled=$1" >/dev/null 2>&1
  "$HELM" upgrade db deploy/helm/db -n stock \
    --set "networkPolicy.enabled=$1" >/dev/null 2>&1 \
    && echo ">> NetworkPolicy set enabled=$1 (bootstrap + db)"
}

run_day() {  # $1 = day number
  local d="$1" extra=""
  [ "$d" = "2" ] && extra="RUN_LOAD=0"          # bỏ load-gen cho nhanh
  echo ""
  echo "=================================================================="
  echo "  DAY $d — $LABS/day_$d/run-day$d.sh"
  echo "=================================================================="
  env $extra bash "$LABS/day_$d/run-day$d.sh" </dev/null
  local rc=$?
  echo ">> Day $d exit=$rc"
  return $rc
}

# ---- select days ----
if [ $# -ge 1 ]; then DAYS="$1"; else DAYS="1 2 3 4 5"; fi

kubectl get ns stock >/dev/null 2>&1 || { echo "!! ns 'stock' chưa deploy — chạy ./scripts/deploy.sh trước"; exit 1; }

netpol false                                    # labs designed with NetPol OFF
FAIL=0
for d in $DAYS; do run_day "$d" || FAIL=1; done
netpol true                                     # restore capstone state

echo ""
echo ">> $( [ $FAIL -eq 0 ] && echo 'ALL LABS DONE (see each ✔ XONG Day N)' || echo 'SOME LAB EXITED NON-ZERO — check output above' )"
exit $FAIL
