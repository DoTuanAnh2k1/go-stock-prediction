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
#   kustomize-diff <overlayA> <overlayB>
#         Lab 2.4 — render 2 overlay (kubectl kustomize, KHÔNG apply) và khẳng định
#         image tag + replica count KHÁC nhau => chứng minh overlay patch mà không
#         nhân đôi manifest.
#
# Ví dụ:
#   ./e2e-test.sh zero-downtime web 80 web nginx:1.27 /
#   NS=stock ./e2e-test.sh loadgen web 80 20 120 /
#   ./e2e-test.sh kustomize-diff kustomize/overlays/dev kustomize/overlays/prod
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
  local dur="${DUR:-60}"
  # QUAN TRỌNG: poll TỪ TRONG cluster hitting Service ClusterIP (kube-proxy
  # load-balance tới pod Ready). KHÔNG dùng `port-forward svc` để đo downtime —
  # port-forward GHIM vào 1 pod, pod đó bị rolling update giết ⇒ báo lỗi giả.
  kubectl delete pod e2e-poller -n "$NS" --ignore-not-found >/dev/null 2>&1
  echo "[zero-downtime] khởi động poller trong cluster (${dur}s) hitting http://$svc:$port$path ..."
  kubectl run e2e-poller --image=curlimages/curl:latest --restart=Never -n "$NS" -- \
    sh -c "o=0;f=0;end=\$(( \$(date +%s) + $dur ));
           while [ \$(date +%s) -lt \$end ]; do
             if curl -fsS -m 2 \"http://$svc:$port$path\" >/dev/null 2>&1; then o=\$((o+1)); else f=\$((f+1)); fi
           done; echo \"RESULT ok=\$o fail=\$f\"" >/dev/null
  kubectl wait --for=condition=Ready pod/e2e-poller -n "$NS" --timeout=60s >/dev/null

  echo "[zero-downtime] kubectl set image deploy/$deploy *=$image"
  kubectl set image -n "$NS" "deploy/$deploy" "*=$image"
  kubectl rollout status -n "$NS" "deploy/$deploy" --timeout=180s

  echo "[zero-downtime] chờ poller kết thúc..."
  kubectl wait --for=jsonpath='{.status.phase}'=Succeeded pod/e2e-poller -n "$NS" --timeout=120s >/dev/null || true
  local res; res=$(kubectl logs e2e-poller -n "$NS" 2>/dev/null | grep RESULT || echo "RESULT ok=? fail=?")
  local bad; bad=$(echo "$res" | sed -n 's/.*fail=\([0-9]*\).*/\1/p')
  echo "[zero-downtime] KẾT QUẢ (đo từ trong cluster): $res"
  if [ "${bad:-1}" = "0" ]; then echo "PASS ✅ rolling update KHÔNG rớt request"; else echo "WARN ❌ có $bad request lỗi (kiểm tra maxUnavailable/readinessProbe)"; fi
  kubectl delete pod e2e-poller -n "$NS" --ignore-not-found >/dev/null 2>&1
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
  # Load TỪ TRONG cluster hitting Service ClusterIP => kube-proxy trải đều qua TẤT CẢ
  # pod Ready (khác port-forward chỉ ghim 1 pod). Đúng cho HPA (CPU trung bình mọi pod).
  kubectl delete pod e2e-load -n "$NS" --ignore-not-found >/dev/null 2>&1
  echo "[loadgen] $conc luồng x ${secs}s → http://$svc:$port$path (pod e2e-load, qua Service)"
  kubectl run e2e-load --image=curlimages/curl:latest --restart=Never -n "$NS" -- \
    sh -c "end=\$(( \$(date +%s) + $secs ));
      i=0; while [ \$i -lt $conc ]; do
        ( while [ \$(date +%s) -lt \$end ]; do curl -s -o /dev/null \"http://$svc:$port$path\"; done ) &
        i=\$((i+1)); done; wait; echo LOAD_DONE" >/dev/null
  echo "[loadgen] đang chạy trong cluster. Theo dõi ở TAB KHÁC:"
  echo "    kubectl top pods -n $NS -l app=$svc     # CPU từng pod backend tăng"
  echo "    ./e2e-test.sh watch-hpa <hpa>           # HPA REPLICAS/TARGETS"
  kubectl wait --for=jsonpath='{.status.phase}'=Succeeded pod/e2e-load -n "$NS" --timeout=$((secs+60))s >/dev/null 2>&1 || true
  kubectl delete pod e2e-load -n "$NS" --ignore-not-found >/dev/null 2>&1
  echo "[loadgen] xong."
}

cmd_watch_hpa() {
  local hpa="${1:?hpa}"
  kubectl get hpa -n "$NS" "$hpa" -w
}

cmd_kustomize_diff() {
  local a="${1:?overlayA dir}" b="${2:?overlayB dir}"
  echo "[kustomize-diff] render '$a' vs '$b' (chỉ render, KHÔNG apply)"
  local ra rb; ra=$(kubectl kustomize "$a") || die "kubectl kustomize '$a' lỗi"
  rb=$(kubectl kustomize "$b") || die "kubectl kustomize '$b' lỗi"
  # Deployment đầu tiên: lấy image container + replicas
  local ia ib pa pb
  ia=$(printf '%s\n' "$ra" | awk '/^ *- image:/{print $3; exit} /^ *image:/{print $2; exit}')
  ib=$(printf '%s\n' "$rb" | awk '/^ *- image:/{print $3; exit} /^ *image:/{print $2; exit}')
  pa=$(printf '%s\n' "$ra" | awk '/^ *replicas:/{print $2; exit}')
  pb=$(printf '%s\n' "$rb" | awk '/^ *replicas:/{print $2; exit}')
  echo "  $a  -> image=$ia replicas=$pa"
  echo "  $b  -> image=$ib replicas=$pb"
  if [ -n "$ia" ] && [ -n "$pa" ] && [ "$ia" != "$ib" ] && [ "$pa" != "$pb" ]; then
    echo "PASS ✅ image tag VÀ replica count khác nhau — overlay patch không nhân đôi manifest"
  else
    echo "WARN ❌ overlay không khác như mong đợi (image $ia/$ib, replicas $pa/$pb)"; exit 1
  fi
}

main() {
  local sub="${1:-}"; shift || true
  case "$sub" in
    zero-downtime)  cmd_zero_downtime "$@";;
    rollback)       cmd_rollback "$@";;
    bluegreen)      cmd_bluegreen "$@";;
    loadgen)        cmd_loadgen "$@";;
    watch-hpa)      cmd_watch_hpa "$@";;
    kustomize-diff) cmd_kustomize_diff "$@";;
    *) grep -E '^#( |=)' "$0" | sed 's/^# \{0,1\}//'; exit 1;;
  esac
}
main "$@"
