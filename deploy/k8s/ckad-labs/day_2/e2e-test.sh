#!/usr/bin/env bash
# =============================================================================
# e2e-test.sh — harness kiểm thử end-to-end cho các lab Day 2.
# Mục tiêu: "giả lập end-to-end" để CHỨNG MINH từng lab đạt yêu cầu, không chỉ
# nhìn `kubectl get`. Chỉ cần kubectl + curl; HTTP check chạy từ host qua
# port-forward (không phụ thuộc Ingress/LoadBalancer).
#
# Namespace mặc định: stock (override: NS=... ./e2e-test.sh ...)
#
# Lệnh:
#   zero-downtime <svc> <port> <deploy> <newimage> [path]
#         Lab 2.1 — bắn traffic liên tục vào Service TRONG LÚC rolling update,
#         đếm request OK/FAIL => chứng minh update không rớt request.
#   rollback <deploy> [to-revision]
#         Lab 2.1 — undo rollout + in rollout history.
#   bluegreen <svc> <selectorKey> <selectorVal>
#         Lab 2.2 — lật selector của Service sang màu mới, in endpoints để xác nhận
#         traffic chuyển pod set khác.
#   loadgen <svc> <port> <concurrency> <seconds> [path]
#         Lab 2.3 — sinh tải song song để đẩy CPU => kích HPA scale up.
#   watch-hpa <hpa>
#         Lab 2.3 — theo dõi HPA (REPLICAS/TARGETS) realtime.
#
# Ví dụ:
#   ./e2e-test.sh zero-downtime web 80 web nginx:1.27 /
#   NS=stock ./e2e-test.sh loadgen web 80 20 120 /
# =============================================================================
set -euo pipefail

NS="${NS:-stock}"
LPORT="${LPORT:-18080}"

die() { echo "ERROR: $*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1 || die "cần '$1' trong PATH"; }
have kubectl; have curl

# Mở port-forward tới 1 Service, trả về PID (biến toàn cục PF_PID). Tự dọn ở EXIT.
_pf_open() {
  local svc="$1" port="$2"
  kubectl port-forward -n "$NS" "svc/$svc" "$LPORT:$port" >/tmp/e2e-pf.log 2>&1 &
  PF_PID=$!
  trap '_pf_close' EXIT
  sleep 3
  kill -0 "$PF_PID" 2>/dev/null || die "port-forward tới svc/$svc:$port thất bại (xem /tmp/e2e-pf.log)"
}
_pf_close() { [ -n "${PF_PID:-}" ] && kill "$PF_PID" 2>/dev/null || true; }

cmd_zero_downtime() {
  local svc="${1:?svc}" port="${2:?port}" deploy="${3:?deploy}" image="${4:?newimage}" path="${5:-/}"
  _pf_open "$svc" "$port"
  local okf badf; okf=$(mktemp); badf=$(mktemp)
  echo "[zero-downtime] poll http://localhost:$LPORT$path mỗi 0.2s trong lúc update..."
  ( while true; do
      if curl -fsS -m 2 "http://localhost:$LPORT$path" >/dev/null 2>&1; then echo x >>"$okf"; else echo x >>"$badf"; fi
      sleep 0.2
    done ) & local poll=$!

  echo "[zero-downtime] kubectl set image deploy/$deploy *=$image"
  kubectl set image -n "$NS" "deploy/$deploy" "*=$image"
  kubectl rollout status -n "$NS" "deploy/$deploy" --timeout=180s
  sleep 2

  kill "$poll" 2>/dev/null || true
  local ok bad; ok=$(wc -l <"$okf" | tr -d ' '); bad=$(wc -l <"$badf" | tr -d ' ')
  echo "[zero-downtime] KẾT QUẢ: OK=$ok  FAIL=$bad"
  if [ "$bad" -eq 0 ]; then echo "PASS ✅ rolling update KHÔNG rớt request"; else echo "FAIL ❌ có $bad request lỗi khi update (kiểm tra maxUnavailable/readinessProbe)"; fi
  rm -f "$okf" "$badf"
}

cmd_rollback() {
  local deploy="${1:?deploy}" rev="${2:-}"
  if [ -n "$rev" ]; then kubectl rollout undo -n "$NS" "deploy/$deploy" --to-revision="$rev";
  else kubectl rollout undo -n "$NS" "deploy/$deploy"; fi
  kubectl rollout status -n "$NS" "deploy/$deploy" --timeout=120s
  echo "[rollback] history:"; kubectl rollout history -n "$NS" "deploy/$deploy"
}

cmd_bluegreen() {
  local svc="${1:?svc}" key="${2:?selectorKey}" val="${3:?selectorVal}"
  echo "[bluegreen] endpoints TRƯỚC:"; kubectl get endpoints -n "$NS" "$svc" -o wide
  kubectl patch svc -n "$NS" "$svc" -p "{\"spec\":{\"selector\":{\"$key\":\"$val\"}}}"
  sleep 2
  echo "[bluegreen] endpoints SAU (đã trỏ '$key=$val'):"; kubectl get endpoints -n "$NS" "$svc" -o wide
  echo "[bluegreen] pod đang nhận traffic:"; kubectl get pods -n "$NS" -l "$key=$val"
}

cmd_loadgen() {
  local svc="${1:?svc}" port="${2:?port}" conc="${3:-10}" secs="${4:-60}" path="${5:-/}"
  _pf_open "$svc" "$port"
  echo "[loadgen] $conc luồng x ${secs}s vào http://localhost:$LPORT$path (đẩy CPU cho HPA)"
  local endf; endf=$(mktemp); date -d "+$secs seconds" +%s >"$endf" 2>/dev/null || echo "0" >"$endf"
  local pids=()
  for i in $(seq 1 "$conc"); do
    ( end=$(cat "$endf"); while [ "$(date +%s)" -lt "$end" ]; do curl -fsS -m 2 "http://localhost:$LPORT$path" >/dev/null 2>&1 || true; done ) &
    pids+=($!)
  done
  echo "[loadgen] đang chạy... (mở tab khác: ./e2e-test.sh watch-hpa <hpa>)"
  for p in "${pids[@]}"; do wait "$p" 2>/dev/null || true; done
  rm -f "$endf"; echo "[loadgen] xong."
}

cmd_watch_hpa() {
  local hpa="${1:?hpa}"
  kubectl get hpa -n "$NS" "$hpa" -w
}

main() {
  local sub="${1:-}"; shift || true
  case "$sub" in
    zero-downtime) cmd_zero_downtime "$@";;
    rollback)      cmd_rollback "$@";;
    bluegreen)     cmd_bluegreen "$@";;
    loadgen)       cmd_loadgen "$@";;
    watch-hpa)     cmd_watch_hpa "$@";;
    *) grep -E '^#( |=)' "$0" | sed 's/^# \{0,1\}//'; exit 1;;
  esac
}
main "$@"
