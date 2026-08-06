#!/usr/bin/env bash
# =============================================================================
# CKAD — Day 2 — Deployments & rollouts. Chạy TRỌN 4 lab, in lệnh + OUTPUT THẬT.
#   2.1 Rolling Update & Rollback   (app demo 'web' nginx, cô lập)
#   2.2 Blue/Green Switch           (1 Deployment 'bg', maxSurge:100% — KHÔNG đụng web-svc thật)
#   2.3 Scale & HPA                 (app 'cpu-burn' hpa-example)
#   2.4 Kustomize Overlay           (app 'web-kz' cô lập)
#
#   ./run-day2.sh                    # cả 4 lab, cuối tự dọn (app thật giữ nguyên)
#   ONLY=2.3 ./run-day2.sh           # chỉ 1 lab: 2.1 | 2.2 | 2.3 | 2.4
#   DUR=45 ./run-day2.sh             # thời lượng poll zero-downtime (giây, mặc định 30)
#   RUN_LOAD=0 ./run-day2.sh         # 2.3: bỏ bắn tải (chỉ attach HPA + show), nhanh
#   LOAD_SECS=120 ./run-day2.sh      # 2.3: thời lượng bắn tải (giây, mặc định 100)
#   WAIT_SCALEDOWN=1 ./run-day2.sh   # 2.3: chờ luôn cửa sổ scale-down 300s về minReplicas
#   REAL_HPA_CYCLE=1 ./run-day2.sh   # 2.3: demo up→down TRÊN HPA THẬT (api-svc+gateway), hạ target 10%→load→restore (~8')
#   KEEP=1 ./run-day2.sh             # giữ object đã tạo (không dọn)
#
# Yêu cầu: context=kind-ckad, ns=stock. Image nginx/curl pull on-demand (node pull OK);
#          web-svc:dev/:v2 + hpa-example đã có sẵn. web-svc thật (blue/green) KHÔNG bị đụng.
# =============================================================================
set -uo pipefail          # KHÔNG set -e: có lệnh cố tình lỗi (bad deploy) + rollout timeout

NS=stock
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
E2E="$DIR/e2e-test.sh"
ONLY="${ONLY:-all}"
export NS
export DUR="${DUR:-30}"                    # e2e zero-downtime đọc DUR
LOAD_SECS="${LOAD_SECS:-100}"

# ---- màu ---------------------------------------------------------------------
if [ -t 1 ]; then B=$'\e[1m'; C=$'\e[36m'; Y=$'\e[33m'; G=$'\e[32m'; R=$'\e[31m'; D=$'\e[2m'; X=$'\e[0m'
else B=; C=; Y=; G=; R=; D=; X=; fi
hr(){ printf '%s%s%s\n' "$D" "────────────────────────────────────────────────────────────────" "$X"; }
title(){ printf '\n'; hr; printf '%s%s  %s%s\n' "$B" "$C" "$1" "$X"; hr; }
note(){ printf '%s# %s%s\n' "$D" "$1" "$X"; }
run(){ printf '\n%s$' "$Y"; printf ' %s' "$@"; printf '%s\n' "$X"; "$@"; }
runsh(){ printf '\n%s$ %s%s\n' "$Y" "$1" "$X"; sh -c "$1"; }

# ---- prereq ------------------------------------------------------------------
ctx="$(kubectl config current-context 2>/dev/null || true)"
[ "$ctx" = "kind-ckad" ] || { printf '%sContext="%s" (cần kind-ckad). kubectl config use-context kind-ckad%s\n' "$R" "$ctx" "$X"; exit 1; }
kubectl get ns "$NS" >/dev/null 2>&1 || { printf '%sKhông thấy ns %s%s\n' "$R" "$NS" "$X"; exit 1; }
[ -f "$E2E" ] || { printf '%sKhông thấy %s%s\n' "$R" "$E2E" "$X"; exit 1; }

# =============================================================================
lab_2_1(){
title "LAB 2.1 — Rolling Update & Rollback (app 'web' nginx, cô lập)"

note "setup) Apply web.yaml — 4 replica nginx:1.25, RollingUpdate maxUnavailable=0, readiness, preStop sleep 5"
run kubectl apply -f "$DIR/web.yaml"
note "   chờ 4 pod Ready (pull nginx:1.25 lần đầu ~15-30s)..."
kubectl rollout status deploy/web -n "$NS" --timeout=180s || true
run kubectl get deploy web -n "$NS"

note "1) Rolling update v1→v2 (nginx:1.25 → 1.27) — gắn change-cause + rollout status (exit code = tín hiệu CI)"
run kubectl annotate deploy/web kubernetes.io/change-cause="pin nginx:1.27" --overwrite -n "$NS"
run kubectl set image deploy/web web=nginx:1.27 -n "$NS"
run kubectl rollout status deploy/web -n "$NS" --timeout=180s
note "   ReplicaSet: RS mới scale-up, RS cũ về 0 (giữ để undo); revisionHistoryLimit giữ tối đa 10"
run kubectl get rs -n "$NS" -l app=web
run kubectl rollout history deploy/web -n "$NS"

note "2) Zero-downtime — bắn traffic TRONG cluster (poll qua Service ClusterIP) TRONG LÚC rolling update 1.27→1.26"
run bash "$E2E" zero-downtime web 80 web nginx:1.26 /

note "3) BAD deploy (nginx:9.9.9-nope) — maxUnavailable=0 chặn không sập; pod cũ vẫn phục vụ"
run kubectl set image deploy/web web=nginx:9.9.9-nope -n "$NS"
note "   rollout status --timeout=20s sẽ 'timed out' (KẸT ở ImagePullBackOff) — đúng như mong đợi:"
run kubectl rollout status deploy/web -n "$NS" --timeout=20s || printf '%s   ↑ timed out = rollout KẸT (không phải lỗi script)%s\n' "$D" "$X"
run kubectl get pods -n "$NS" -l app=web
note "   → pod MỚI ImagePullBackOff/ErrImagePull; pod CŨ vẫn 1/1 Running (Service không sập)"

note "4) Rollback (undo) → hồi image tốt trước đó + in rollout history"
run bash "$E2E" rollback web
run kubectl get pods -n "$NS" -l app=web
}

# =============================================================================
lab_2_2(){
title "LAB 2.2 — Blue/Green trên api-svc CÓ SẴN (2 Deployment blue/green + 1 Service chung, FLIP selector)"
note "KHÔNG tạo manifest mới: dùng api-svc-blue + api-svc-green ĐÃ deploy sẵn (Helm bluegreen), CHUNG 1 Service"
note "'api-svc' kèm selector 'color'. Cutover = đổi .spec.selector.color; rollback = flip lại. web-svc thật KHÔNG bị đụng."

# prereq: Service api-svc phải kèm color selector (chart api-svc bluegreen.enabled=true — mặc định bật)
if ! kubectl get svc api-svc -n "$NS" -o jsonpath='{.spec.selector.color}' 2>/dev/null | grep -q .; then
  note "  Service api-svc CHƯA kèm color selector → cần: ${HELM:-/home/chronical/.local/bin/helm} upgrade api-svc $DIR/../../../helm/api-svc -n $NS --set bluegreen.enabled=true. BỎ QUA lab 2.2."
  return
fi

note "0) HIỆN TRẠNG — 2 Deployment blue/green Ready sẵn + 1 Service chung; selector đang trỏ màu nào"
run kubectl get deploy -n "$NS" -l app=api-svc -L color
runsh "kubectl get svc api-svc -n $NS -o jsonpath='  Service api-svc selector = {.spec.selector}{\"\\n\"}'"

# ---- helper: FLIP selector rồi CHỨNG MINH route bằng APP LOG (Go zerolog của api-svc, KHÔNG phải nginx) ----
_bg_prove(){                                     # $1 = màu cutover tới
  local c="$1" mk="bgprobe-$1-$$"
  note "── CUTOVER Service api-svc → '$c' (đổi selector) → curl vào Service → soi APP LOG container 'api-svc' 2 màu ──"
  run kubectl patch svc api-svc -n "$NS" -p "{\"spec\":{\"selector\":{\"app\":\"api-svc\",\"color\":\"$c\"}}}"
  runsh "kubectl get svc api-svc -n $NS -o jsonpath='  selector -> {.spec.selector}{\"\\n\"}'"
  note "  Endpoints Service = IP các pod màu '$c' (khớp = selector trỏ đúng bộ pod):"
  runsh "kubectl get endpoints api-svc -n $NS 2>/dev/null | grep -v deprecat"
  runsh "kubectl get pods -n $NS -l app=api-svc,color=$c -o jsonpath='  pod color=$c IP: {range .items[*]}{.status.podIP} {end}{\"\\n\"}'"
  kubectl delete pod bg-curl -n "$NS" --ignore-not-found >/dev/null 2>&1
  note "  NGUỒN traffic — tạo pod 'bg-curl' IN-CLUSTER bắn 5 request wget vào Service chung (qua kube-proxy→selector), path độc nhất để soi log:"
  printf "%s$ kubectl run bg-curl --image=nginx:1.27-alpine --restart=Never -n %s --command -- sh -c 'for i in 1 2 3 4 5; do wget -qO- http://api-svc:8118/api/version-%s; done'%s\n" "$Y" "$NS" "$mk" "$X"
  kubectl run bg-curl --image=nginx:1.27-alpine --restart=Never -n "$NS" --command -- \
    sh -c "for i in 1 2 3 4 5; do wget -qO- \"http://api-svc:8118/api/version-$mk\" >/dev/null 2>&1; done; echo curl-done" >/dev/null 2>&1
  kubectl wait --for=jsonpath='{.status.phase}'=Succeeded pod/bg-curl -n "$NS" --timeout=45s >/dev/null 2>&1 || true
  local hb hg
  hb=$(kubectl logs -n "$NS" -l app=api-svc,color=blue  -c api-svc --tail=150 2>/dev/null | grep -c "$mk")
  hg=$(kubectl logs -n "$NS" -l app=api-svc,color=green -c api-svc --tail=150 2>/dev/null | grep -c "$mk")
  note "  dòng APP LOG (Go zerolog, container 'api-svc') khớp marker ở màu '$c' = request VÀO ĐÚNG APP (không chỉ nginx ambassador):"
  runsh "kubectl logs -n $NS -l app=api-svc,color=$c -c api-svc --tail=150 2>/dev/null | grep '$mk' | tail -1 | sed 's/\\x1b\\[[0-9;]*m//g; s/^/    /'"
  printf '%s  → APP LOG api-svc  BLUE hits=%s | GREEN hits=%s  →  request tới ĐÚNG app màu "%s" (0 màu kia)%s\n' "$G" "$hb" "$hg" "$c" "$X"
  kubectl delete pod bg-curl -n "$NS" --ignore-not-found >/dev/null 2>&1
}

note "1) CUTOVER sang BLUE + chứng minh route bằng app-log"
_bg_prove blue
note "2) CUTOVER sang GREEN + chứng minh route bằng app-log"
_bg_prove green

note "3) ROLLBACK = FLIP lại (tức thời, KHÔNG rollout/rebuild). Đang ở green = activeColor mặc định."
note "   ⚠️ patch tay chỉ BỀN tới lần helm upgrade kế (helm reset selector về activeColor); cutover LÂU DÀI:"
note "      ${HELM:-/home/chronical/.local/bin/helm} upgrade api-svc $DIR/../../../helm/api-svc -n $NS --set bluegreen.activeColor=<màu>"
note "   Khác 2.1: 2.1 = rolling update maxSurge=1 thay dần TRONG 1 Deployment; 2.2 = 2 Deployment song song Ready sẵn"
note "   ⇒ cutover TỨC THÌ zero-downtime bằng selector, rollback = flip lại."
}

# =============================================================================
lab_2_3(){
title "LAB 2.3 — Scale & HPA (app 'cpu-burn' = registry.k8s.io/hpa-example)"

note "setup) Đảm bảo cpu-burn tồn tại; GỠ HPA cũ + scale=1 (HPA sở hữu replicas → phải demo manual-scale khi CHƯA có HPA)"
run kubectl apply -f "$DIR/cpu-burn.yaml"
kubectl delete hpa cpu-burn -n "$NS" --ignore-not-found >/dev/null 2>&1
kubectl scale deploy/cpu-burn --replicas=1 -n "$NS" >/dev/null 2>&1
kubectl rollout status deploy/cpu-burn -n "$NS" --timeout=120s || true

note "1) Manual scale → 10 (CÙNG 1 ReplicaSet — khác 2.1 nơi đổi image sinh RS mới)"
run kubectl scale deploy/cpu-burn --replicas=10 -n "$NS"
kubectl rollout status deploy/cpu-burn -n "$NS" --timeout=120s || true
run kubectl get rs -n "$NS" -l app=cpu-burn
run kubectl get pods -n "$NS" -l app=cpu-burn -o wide --no-headers
note "   reset về 1 trước khi attach HPA"
run kubectl scale deploy/cpu-burn --replicas=1 -n "$NS"

note "2) Attach HPA 50% CPU (declarative autoscaling/v2). requests.cpu=100m BẮT BUỘC để tính %"
run kubectl apply -f "$DIR/cpu-burn-hpa.yaml"
note "   ~15s đầu metrics chưa về → TARGETS <unknown>/50%; chờ metric-server..."
sleep 18
run kubectl get hpa cpu-burn -n "$NS"
runsh "kubectl get hpa cpu-burn -n $NS -o jsonpath='{range .status.conditions[*]}{.type}={.status} {.reason}{\"\\n\"}{end}'"

if [ "${RUN_LOAD:-1}" = "1" ]; then
  note "3) Bắn tải nền qua Service (${LOAD_SECS}s) → CPU vọt → HPA scale-UP theo BẬC (1→4→8→10, cap +4/×2 mỗi ~15s)"
  ( bash "$E2E" loadgen cpu-burn 80 30 "$LOAD_SECS" / >/tmp/day2-load.log 2>&1 ) &
  _lpid=$!
  printf '%s$ (poll) kubectl get hpa cpu-burn — REPLICAS/TARGETS mỗi 15s trong lúc bắn tải%s\n' "$Y" "$X"
  for _i in $(seq 1 7); do
    sleep 15
    runsh "kubectl get hpa cpu-burn -n $NS --no-headers 2>/dev/null"
  done
  wait "$_lpid" 2>/dev/null || true
  note "   CPU thực từng pod (%% của REQUEST 100m, KHÔNG phải limit 500m):"
  run kubectl top pods -n "$NS" -l app=cpu-burn
  note "   Events HPA — SuccessfulRescale theo bậc:"
  runsh "kubectl describe hpa cpu-burn -n $NS | sed -n '/Events:/,\$p' | grep -E 'SuccessfulRescale|New size' | tail -6"
  run kubectl get deploy cpu-burn -n "$NS"

  if [ "${WAIT_SCALEDOWN:-0}" = "1" ]; then
    note "4) Scale-DOWN — tải dừng, CPU về ~1% ngay nhưng HPA GIỮ đỉnh ~300s (stabilizationWindow) rồi hạ về min=1"
    printf '%s$ (poll) chờ HPA hạ REPLICAS về 1 (tối đa 360s)%s\n' "$Y" "$X"
    _t=0
    while [ "$_t" -lt 360 ]; do
      _r="$(kubectl get deploy cpu-burn -n "$NS" -o jsonpath='{.spec.replicas}' 2>/dev/null)"
      runsh "kubectl get hpa cpu-burn -n $NS --no-headers 2>/dev/null"
      [ "$_r" = "1" ] && { printf '%s  → đã về minReplicas=1 sau ~%ds%s\n' "$G" "$_t" "$X"; break; }
      sleep 30; _t=$((_t+30))
    done
  else
    note "4) Scale-DOWN mặc định KHÔNG chờ (cửa sổ ~300s). Bật: WAIT_SCALEDOWN=1. HPA sẽ tự về 1 sau ~5' khi hết tải."
  fi
else
  note "3) RUN_LOAD=0 → bỏ bắn tải. HPA đã attach (TARGETS/50%), scale-up/down bỏ qua cho nhanh."
fi

# ---- 5) HPA áp THẲNG lên service THẬT (api-svc blue/green + gateway-svc) ----
note "5) HPA áp THẬT lên service project (gated hpa.enabled per-chart api-svc + gateway-svc) — khác cpu-burn (app demo)"
HELM="${HELM:-/home/chronical/.local/bin/helm}"
CHART_API="$DIR/../../../helm/api-svc"
CHART_GW="$DIR/../../../helm/gateway-svc"
if kubectl get hpa gateway-svc -n "$NS" >/dev/null 2>&1; then
  run kubectl get hpa api-svc-blue api-svc-green gateway-svc -n "$NS"
  note "   Service project I/O-bound (idle 1-2% CPU) → CPU-HPA CHỈ scale khi traffic spike THẬT; thường nằm ở minReplicas (giữ HA)."
  note "   HPA sở hữu .spec.replicas ⇒ deployment OMIT replicas khi hpa.enabled (tránh flapping với helm upgrade). api-svc blue/green: HPA mỗi màu; màu idle nằm ở min."
else
  note "   HPA thật CHƯA bật. Bật: $HELM upgrade api-svc $CHART_API -n $NS --set hpa.enabled=true  (và gateway-svc tương tự)"
fi
if [ "${REAL_HPA_CYCLE:-0}" = "1" ] && [ -x "$HELM" ]; then
  note "   REAL_HPA_CYCLE=1 → demo up→down TRÊN HPA THẬT. Hạ target 60→10% (request /api/version quá rẻ, không đẩy nổi CPU 60%), bắn HTTP load, chờ scale-down, khôi phục 60%."
  runsh "$HELM upgrade api-svc     $CHART_API -n $NS --set hpa.enabled=true --set hpa.apiSvc.targetCPU=10 2>&1 | grep -E 'REVISION|STATUS'"
  runsh "$HELM upgrade gateway-svc $CHART_GW  -n $NS --set hpa.enabled=true --set hpa.gatewaySvc.targetCPU=10 2>&1 | grep -E 'REVISION|STATUS'"
  ( bash "$E2E" loadgen gateway-svc 80 150 150 /api/version >/tmp/day2-realhpa.log 2>&1 ) &
  _rp=$!; _up=""; _t=0
  printf '%s$ (poll) HPA gateway-svc + api-svc-green mỗi 20s tới scale-down về min=2 (cap 540s)%s\n' "$Y" "$X"
  while [ "$_t" -lt 540 ]; do
    _gr="$(kubectl get hpa gateway-svc  -n "$NS" --no-headers 2>/dev/null | awk '{print $7}')"
    _ar="$(kubectl get hpa api-svc-green -n "$NS" --no-headers 2>/dev/null | awk '{print $7}')"
    runsh "kubectl get hpa gateway-svc api-svc-green api-svc-blue -n $NS --no-headers 2>/dev/null"
    [ -z "$_up" ] && { [ "${_gr:-0}" -gt 2 ] 2>/dev/null || [ "${_ar:-0}" -gt 2 ] 2>/dev/null; } && _up=$_t
    [ -n "$_up" ] && [ "${_gr:-9}" = "2" ] && [ "${_ar:-9}" = "2" ] && { printf '%s  → scale-DOWN về min=2 tại ~%ds (up bắt đầu ~%ds; giữ đỉnh ~300s window)%s\n' "$G" "$_t" "$_up" "$X"; break; }
    sleep 20; _t=$((_t+20))
  done
  wait "$_rp" 2>/dev/null || true
  note "   Events (cả 2 chiều):"
  runsh "kubectl describe hpa gateway-svc -n $NS | sed -n '/Events:/,\$p' | grep 'New size' | tail -5"
  note "   khôi phục targetCPU về 60% (production) + dọn pod load:"
  runsh "$HELM upgrade api-svc     $CHART_API -n $NS --set hpa.enabled=true 2>&1 | grep REVISION"
  runsh "$HELM upgrade gateway-svc $CHART_GW  -n $NS --set hpa.enabled=true 2>&1 | grep REVISION"
  kubectl delete pod e2e-load -n "$NS" --ignore-not-found >/dev/null 2>&1
else
  note "   (full cycle up→down trên HPA thật: REAL_HPA_CYCLE=1 — mất ~8' vì window 300s + phải hạ target demo)"
fi
}

# =============================================================================
lab_2_4(){
KZNS=ckad-kustomize
BASE="$DIR/kustomize/base"; DEV="$DIR/kustomize/overlays/dev"; PROD="$DIR/kustomize/overlays/prod"
title "LAB 2.4 — Kustomize: DESCRIBE base → xem overlay PATCH gì (app 'web-kz' ns riêng '$KZNS')"

note "0) Tooling: 'kubectl kustomize' / 'kubectl apply -k' (Kustomize nhúng trong kubectl, không cần binary rời)"
runsh "kubectl version --client 2>/dev/null | grep -i kustomize || echo '(kubectl có Kustomize nhúng)'"

# render sẵn 3 bản ra /tmp: base = nguồn chung; overlay = base + DELTA. Diff base↔overlay để thấy patch.
kubectl kustomize "$BASE" > /tmp/kz-base.yaml 2>/dev/null
kubectl kustomize "$DEV"  > /tmp/kz-dev.yaml  2>/dev/null
kubectl kustomize "$PROD" > /tmp/kz-prod.yaml 2>/dev/null

note "1) BASE — nguồn CHUNG, KHÔNG môi trường: name=web-kz · replicas=2 · image=web-svc:dev · KHÔNG namespace"
runsh "grep -E '^kind:|^  name:|replicas:|image:|namespace:' /tmp/kz-base.yaml"
note "   → base namespace-agnostic (không 'namespace:'), replicas mặc định 2, tag dev — overlay sẽ patch các field này"

note "2) OVERLAY dev = BASE + DELTA. diff base↔dev cho thấy CHÍNH XÁC dev đổi gì ('<' = base, '>' = dev):"
runsh "diff /tmp/kz-base.yaml /tmp/kz-dev.yaml | grep -E '^[<>]' | grep -E 'name:|namespace:|replicas:|image:' || true"
note "   → dev PATCH: + namespace=$KZNS · namePrefix 'dev-' (web-kz→dev-web-kz) · replicas 2→1 (GIỮ tag dev)"

note "3) OVERLAY prod = BASE + DELTA khác. diff base↔prod:"
runsh "diff /tmp/kz-base.yaml /tmp/kz-prod.yaml | grep -E '^[<>]' | grep -E 'name:|namespace:|replicas:|image:' || true"
note "   → prod PATCH: + namespace=$KZNS · namePrefix 'prod-' · replicas 2→5 · image web-svc:dev→v2"

note "4) BẢNG ĐỐI CHIẾU 3 render (field chính) — 1 base, 2 overlay, KHÔNG nhân đôi manifest:"
runsh "for o in base dev prod; do printf '  -- %s --\\n' \"\$o\"; grep -E '^  name:|replicas:|image:|namespace:' /tmp/kz-\$o.yaml; done"

note "5) apply -k CẢ dev (replicas 1) lẫn prod (replicas 5) vào cùng ns $KZNS — khác namePrefix nên không đụng nhau"
kubectl create namespace "$KZNS" >/dev/null 2>&1 && note "   → đã tạo ns $KZNS (Kustomize set .metadata.namespace nhưng KHÔNG tự tạo ns)" || note "   → ns $KZNS đã có"
run kubectl apply -k "$DEV"
run kubectl apply -k "$PROD"
kubectl rollout status deploy/dev-web-kz  -n "$KZNS" --timeout=90s  || true
kubectl rollout status deploy/prod-web-kz -n "$KZNS" --timeout=120s || true

note "6) OUTPUT SẠCH — 'get' trên ns $KZNS chỉ thấy đồ Kustomize (dev-web-kz 1/1 + prod-web-kz 5/5), không lẫn stack thật:"
run kubectl get deploy,svc,pod -n "$KZNS"

note "7) GIỮ dev-web-kz + prod-web-kz để quan sát — namespace $KZNS sẽ được xóa ở bước DỌN DẸP cuối (sau khi bấm Enter)"
rm -f /tmp/kz-base.yaml /tmp/kz-dev.yaml /tmp/kz-prod.yaml
}

# =============================================================================
cleanup(){
  [ "${KEEP:-0}" = "1" ] && { printf '\n%sKEEP=1 → giữ lại object (web, web-kz, HPA...).%s\n' "$D" "$X"; return; }
  title "DỌN DẸP (roll back — api-svc/web-svc/cpu-burn thật giữ nguyên)"
  run kubectl delete -f "$DIR/web.yaml" --ignore-not-found
  run kubectl delete pod bg-curl -n "$NS" --ignore-not-found          # probe pod của 2.2 (nếu còn sót)
  run kubectl delete namespace ckad-kustomize --ignore-not-found      # lab 2.4 dùng ns riêng
  run kubectl delete pod e2e-poller e2e-load -n "$NS" --ignore-not-found
  # 2.2 flip selector Service api-svc THẬT → khôi phục về activeColor mặc định (green) nếu có color selector
  if kubectl get svc api-svc -n "$NS" -o jsonpath='{.spec.selector.color}' 2>/dev/null | grep -q .; then
    note "khôi phục Service api-svc selector → green (activeColor mặc định)"
    run kubectl patch svc api-svc -n "$NS" -p '{"spec":{"selector":{"app":"api-svc","color":"green"}}}'
  fi
  # ⚠️ PHẢI xóa HPA TRƯỚC: sau khi 2.3 bắn tải, HPA giữ cpu-burn ở 10 suốt cửa sổ scaleDown
  # ~300s (dù CPU đã 1%). Nếu chỉ `scale=1` mà giữ HPA → HPA kéo lại 10 → cpu-burn kẹt 10 (ăn
  # ~1000m requests.cpu + 10 pod). Chạy script 2 LẦN LIÊN TIẾP → resource dồn → đụng ResourceQuota
  # (lab 3.4) → 2.4 prod-web-kz (5 replica) không lên đủ → `rollout status` chờ 120s = TREO.
  note "cpu-burn: XÓA HPA rồi scale về 1 (không giữ HPA — tránh kẹt 10 do cửa sổ scaleDown 300s)"
  run kubectl delete hpa cpu-burn -n "$NS" --ignore-not-found
  run kubectl scale deploy/cpu-burn --replicas=1 -n "$NS"
  note "Kiểm chứng: app demo đã sạch, api-svc/web-svc thật còn nguyên:"
  run kubectl get deploy -n "$NS" -l 'app in (web,web-kz)'
  run kubectl get deploy -n "$NS" -l app=api-svc
}

# ---- điều phối ---------------------------------------------------------------
case "$ONLY" in
  2.1) lab_2_1 ;;
  2.2) lab_2_2 ;;
  2.3) lab_2_3 ;;
  2.4) lab_2_4 ;;
  *)   lab_2_1; lab_2_2; lab_2_3; lab_2_4 ;;
esac

# ---- chờ Enter rồi mới DỌN DẸP (cho quan sát object đã tạo: web, cpu-burn+HPA, ns ckad-kustomize, selector api-svc) ----
if [ "${KEEP:-0}" = "1" ]; then
  printf '\n%sKEEP=1 → giữ nguyên mọi object, KHÔNG dọn.%s\n' "$D" "$X"
elif [ -t 0 ]; then
  printf '\n%s⏸  Object lab đang GIỮ để quan sát (web, cpu-burn+HPA, ns ckad-kustomize, selector api-svc).%s\n' "$Y" "$X"
  printf '%s   → Bấm ENTER để DỌN DẸP (Ctrl-C để giữ nguyên)... %s' "$Y" "$X"
  read -r _ || true
  cleanup
else
  printf '\n%s(stdin không phải TTY → dọn dẹp ngay, không chờ Enter)%s\n' "$D" "$X"
  cleanup
fi
printf '\n%s%s✔ XONG Day 2 (Lab %s).%s\n' "$B" "$G" "$ONLY" "$X"
