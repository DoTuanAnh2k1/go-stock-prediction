#!/usr/bin/env bash
# =============================================================================
# CKAD — Day 5 — Observability & exam prep. Verify/triage trên stack THẬT (ns stock).
#   5.1 Self-Healing App        (read-only: startupProbe prediction-svc)
#   5.2 CLI Observability       (read-only: logs -c / events / top)
#   5.3 Broken YAML Triage      (throwaway 'triage' 3 bug → fix từng lỗi → dọn)
#   5.4 Helm Deploy & Rollback  (read-only: history/get values + --dry-run; KHÔNG rollback thật)
#
#   ./run-day5.sh                    # cả 4 lab, cuối tự dọn (triage)
#   ONLY=5.3 ./run-day5.sh           # chỉ 1 lab: 5.1 | 5.2 | 5.3 | 5.4
#   KEEP=1 ./run-day5.sh             # GIỮ object nháp (triage) — không dọn
#
# Yêu cầu: context=kind-ckad, ns=stock, metrics-server (cho top). helm KHÔNG trên PATH →
#   /home/chronical/.local/bin/helm. 5.4 CHỈ đọc + --dry-run (KHÔNG helm rollback thật để
#   an toàn release live — rollback là revert TOÀN BỘ state, xoá resource ở rev sau).
# =============================================================================
set -uo pipefail          # KHÔNG set -e: 5.3 apply broken.yaml CỐ TÌNH bị reject (BUG1)

NS=stock
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$DIR/../../../.." && pwd)"           # repo root
DEMO_CHART="${DEMO_CHART:-api-svc}"                # chart per-service dùng minh hoạ upgrade/rollback
CHART="$ROOT/deploy/helm/$DEMO_CHART"
BROKEN="$DIR/broken.yaml"
ONLY="${ONLY:-all}"
HELM=/home/chronical/.local/bin/helm

# ---- màu (tắt khi không phải terminal) --------------------------------------
if [ -t 1 ]; then B=$'\e[1m'; C=$'\e[36m'; Y=$'\e[33m'; G=$'\e[32m'; R=$'\e[31m'; D=$'\e[2m'; X=$'\e[0m'
else B=; C=; Y=; G=; R=; D=; X=; fi

hr(){ printf '%s%s%s\n' "$D" "────────────────────────────────────────────────────────────────" "$X"; }
title(){ printf '\n'; hr; printf '%s%s  %s%s\n' "$B" "$C" "$1" "$X"; hr; }
note(){ printf '%s# %s%s\n' "$D" "$1" "$X"; }
run(){ printf '\n%s$' "$Y"; printf ' %s' "$@"; printf '%s\n' "$X"; "$@"; }
runsh(){ printf '\n%s$ %s%s\n' "$Y" "$1" "$X"; sh -c "$1"; }   # cho lệnh có pipe
runx(){ # chạy lệnh MONG ĐỢI lỗi (demo)
  printf '\n%s$' "$Y"; printf ' %s' "$@"; printf '%s\n' "$X"
  if "$@"; then printf '%s(?!) lệnh này lẽ ra phải lỗi%s\n' "$R" "$X"
  else printf '%s↑ ĐÚNG NHƯ MONG ĐỢI — apply bị admission từ chối (selector≠template)%s\n' "$G" "$X"; fi; }
HELMLOG="${TMPDIR:-/tmp}/ckad-day5-helm.log"; : > "$HELMLOG"
runhelm(){  # helm output rất dài (--dry-run render CẢ manifest) → hiện ~10 dòng đầu, full ra $HELMLOG
  printf '\n%s$' "$Y"; printf ' %s' "$@"; printf '%s\n' "$X"
  local out; out="$("$@" 2>&1)"
  printf '%s\n' "$out" | head -10
  { printf '\n===== $ %s =====\n' "$*"; printf '%s\n' "$out"; } >> "$HELMLOG"
  printf '%s   … (%s dòng — full output ở %s)%s\n' "$D" "$(printf '%s\n' "$out" | wc -l | tr -d ' ')" "$HELMLOG" "$X"
}

# ---- prereq -----------------------------------------------------------------
ctx="$(kubectl config current-context 2>/dev/null || true)"
[ "$ctx" = "kind-ckad" ] || { printf '%sContext hiện tại = "%s" (mong đợi kind-ckad). Đổi: kubectl config use-context kind-ckad%s\n' "$R" "$ctx" "$X"; exit 1; }
kubectl get ns "$NS" >/dev/null 2>&1 || { printf '%sKhông thấy namespace %s%s\n' "$R" "$NS" "$X"; exit 1; }
[ -x "$HELM" ] || { printf '%sKhông thấy helm tại %s%s\n' "$R" "$HELM" "$X"; exit 1; }

# =============================================================================
lab_5_1(){
title "LAB 5.1 — Self-Healing App (startupProbe cho container slow-start)"

P="$(kubectl get pod -n "$NS" -l app=prediction-svc -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)"
[ -n "$P" ] || { note "(không thấy pod prediction-svc — skip)"; return; }
note "pod mẫu = $P"

note "1) startupProbe của prediction-svc (Python torch import + ML init ~30–120s — thêm mới ở rev8)"
runsh "kubectl get pod $P -n $NS -o jsonpath='{.spec.containers[?(@.name==\"prediction-svc\")].startupProbe}'; echo"
note "   → tcpSocket:8119 · periodSeconds:5 · failureThreshold:30 (~150s gate liveness/readiness)"
note "   → auth-svc (Java/Flyway) ĐÃ có startupProbe từ trước (failureThreshold:30 periodSeconds:5)"
note "Điểm chốt: thứ tự probe = startup → (readiness ∥ liveness). Startup chưa pass → liveness HOÃN"
note "           → container slow-start KHÔNG bị liveness restart loop."
}

# =============================================================================
lab_5_2(){
title "LAB 5.2 — CLI Observability (logs -c / events / top trên pod thật ambassador)"

POD="$(kubectl get pod -n "$NS" -l app=api-svc -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)"
if [ -n "$POD" ]; then
  note "1) logs -c — pod api-svc nhiều container (ambassador): app + nginx + log-sidecar"
  runsh "kubectl get pod $POD -n $NS -o jsonpath='{range .spec.containers[*]}{.name} {end}'; echo"
  note "   → 3 container/pod → kubectl logs BẮT BUỘC -c <container>"
  run kubectl logs "$POD" -n "$NS" -c log-sidecar --tail=2
else
  note "1) (không thấy pod api-svc — skip logs -c)"
fi

note "2) Events (nguồn chẩn đoán #1 cho pull/schedule/probe/quota) — toàn ns, sort theo thời gian"
runsh "kubectl get events -n $NS --sort-by=.lastTimestamp | tail -4"

note "3) top (metrics-server) — usage THẬT (khác resources.requests là 'đặt trước')"
run kubectl top pods -n "$NS" -l app=prediction-svc
run kubectl top nodes
}

# =============================================================================
lab_5_3(){
title "LAB 5.3 — Broken YAML Triage (throwaway 'triage' — 3 bug kinh điển, dọn cuối)"
[ -f "$BROKEN" ] || { note "(không thấy $BROKEN — skip)"; return; }

note "setup) dọn cũ cho idempotent"
kubectl delete deploy triage svc triage -n "$NS" --ignore-not-found >/dev/null 2>&1

note "1) BUG1 — apply broken.yaml → Deployment BỊ TỪ CHỐI (selector app=web ≠ template app=webapp)"
runx kubectl apply -f "$BROKEN"
note "   → 'selector does not match template labels' (admission fail). Service THÌ được tạo."

note "2) FIX BUG1 — sửa selector.matchLabels app:web → app:webapp rồi apply lại (sed stream)"
runsh "sed 's/matchLabels: { app: web }/matchLabels: { app: webapp }/' '$BROKEN' | kubectl apply -f -"
kubectl rollout status deploy/triage -n "$NS" --timeout=15s >/dev/null 2>&1 || true

note "3) BUG3 — image sai chính tả (nginx:latst) → Pod ImagePullBackOff/ErrImagePull (lỗi runtime)"
run kubectl get pod -n "$NS" -l app=webapp
note "   FIX: set image về nginx:1.27-alpine"
run kubectl set image deploy/triage web=nginx:1.27-alpine -n "$NS"
kubectl rollout status deploy/triage -n "$NS" --timeout=60s >/dev/null 2>&1 || true

note "4) BUG2 — Service targetPort 8080 nhưng container cổng 80 → endpoint trỏ SAI cổng (lỗi kết nối)"
sleep 2
run kubectl get endpoints triage -n "$NS"
note "   → endpoint :8080 (sai). FIX: patch targetPort về 80"
run kubectl patch svc triage -n "$NS" -p '{"spec":{"ports":[{"port":80,"targetPort":80}]}}'
sleep 2
run kubectl get endpoints triage -n "$NS"
note "   → endpoint :80 = khớp cổng container → service reachable"
note "Điểm chốt: selector≠template = admission · image sai = runtime · targetPort sai = kết nối (3 tầng)"
}

# =============================================================================
lab_5_4(){
title "LAB 5.4 — Helm Deploy & Rollback per-service (READ-ONLY + --dry-run — KHÔNG rollback thật; release '$DEMO_CHART')"

note "1) helm history — lịch sử revision thật của CHART '$DEMO_CHART' (Day 2/3 sinh ra qua các lần upgrade)"
run "$HELM" history "$DEMO_CHART" -n "$NS"

note "2) helm get values — value đang áp cho release '$DEMO_CHART'"
run "$HELM" get values "$DEMO_CHART" -n "$NS"

note "3) value override demo — --dry-run (KHÔNG apply thật, chỉ render để đối chiếu)"
runhelm "$HELM" upgrade "$DEMO_CHART" "$CHART" -n "$NS" --set imageTag=dev --dry-run
note "   → --dry-run render manifest với imageTag override NHƯNG không đổi release live (full manifest ở \$HELMLOG)"

note "Điểm chốt (KHÔNG chạy helm rollback thật trong script — an toàn release live):"
note " - helm upgrade --set k=v = override value; mỗi upgrade/rollback = 1 revision (helm history <chart>)"
note " - helm rollback <chart> <rev> = revert TOÀN BỘ state của CHART đó về snapshot rev (KHÔNG phải undo lệnh cuối)"
note " - BẪY: rollback về rev cũ XOÁ resource thêm ở rev sau — nhưng chỉ trong phạm vi chart đó (per-service = blast radius gọn)"
note " - GitOps: prefer re-apply/upgrade từ chart thay vì rollback khi muốn giữ 1 phần state"
note "   (chạy tay khi cần: $HELM rollback $DEMO_CHART <rev> -n $NS)"
}

# =============================================================================
cleanup(){
  [ "${KEEP:-0}" = "1" ] && { printf '\n%sKEEP=1 → giữ object nháp (triage).%s\n' "$D" "$X"; return; }
  title "DỌN DẸP (object nháp — stack thật giữ nguyên)"
  run kubectl delete deploy,svc triage -n "$NS" --ignore-not-found
  note "Kiểm chứng sạch (rỗng = OK):"
  run kubectl get deploy,svc -n "$NS" -l app=triage
}

# ---- điều phối --------------------------------------------------------------
case "$ONLY" in
  5.1) lab_5_1 ;;
  5.2) lab_5_2 ;;
  5.3) lab_5_3 ;;
  5.4) lab_5_4 ;;
  *)   lab_5_1; lab_5_2; lab_5_3; lab_5_4 ;;
esac
cleanup
printf '\n%s%s✔ XONG Day 5 (Lab %s).%s\n' "$B" "$G" "$ONLY" "$X"
