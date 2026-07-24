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
title "LAB 2.2 — Blue/Green Switch, kiểu 1 DEPLOYMENT (mentor yêu cầu; app cô lập 'bg')"
note "Đề gốc là 2 Deployment + lật selector (blue/green thật). Mentor yêu cầu CHỈ 1 Deployment"
note "=> RollingUpdate maxSurge=100%/maxUnavailable=0: bung GREEN song song BLUE rồi gỡ BLUE. web-svc thật KHÔNG bị đụng."

note "setup) 1 Deployment 'bg' (4 replica, RollingUpdate maxSurge=100% maxUnavailable=0, web-svc:dev = BLUE) + Service bg"
kubectl apply -f - >/dev/null <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata: { name: bg, namespace: stock, labels: { app: bg } }
spec:
  replicas: 4
  strategy:
    type: RollingUpdate
    rollingUpdate: { maxSurge: 100%, maxUnavailable: 0 }
  selector: { matchLabels: { app: bg } }
  template:
    metadata: { labels: { app: bg } }
    spec:
      containers:
        - name: web
          image: web-svc:dev
          imagePullPolicy: IfNotPresent
          ports: [ { containerPort: 3000 } ]
          readinessProbe: { httpGet: { path: /, port: 3000 }, initialDelaySeconds: 2, periodSeconds: 3 }
          # preStop sleep 5: pod phục vụ thêm 5s khi đang bị gỡ endpoint -> tránh termination race
          # (SIGTERM tới nginx TRƯỚC khi kube-proxy gỡ endpoint) => 0 request rớt khi cutover.
          lifecycle: { preStop: { exec: { command: ["sh", "-c", "sleep 5"] } } }
---
apiVersion: v1
kind: Service
metadata: { name: bg, namespace: stock, labels: { app: bg } }
spec:
  selector: { app: bg }
  ports: [ { port: 3000, targetPort: 3000 } ]
EOF
kubectl rollout status deploy/bg -n "$NS" --timeout=120s || true
run kubectl get deploy bg -n "$NS"

note "1) SURGE nhìn thấy được — set image tới image LỖI để 'đóng băng' surge cho dễ quan sát:"
note "   maxSurge=100% bung nguyên bộ GREEN (4) song song BLUE (4) = 8 pod; maxUnavailable=0 giữ AVAILABLE=4"
run kubectl set image deploy/bg web=web-svc:nope -n "$NS"
kubectl rollout status deploy/bg -n "$NS" --timeout=12s || printf '%s   ↑ kẹt (green chưa Ready) = đúng, để quan sát surge%s\n' "$D" "$X"
runsh "kubectl get pods -n $NS -l app=bg -o custom-columns='POD:.metadata.name,IMAGE:.spec.containers[0].image,READY:.status.containerStatuses[0].ready' --no-headers"
run kubectl get deploy bg -n "$NS"
note "   → 4 BLUE Ready phục vụ 100% + 4 GREEN surge (chưa Ready); AVAILABLE giữ 4"

note "2) CUTOVER thật blue→green (web-svc:v2) + đo zero-downtime bằng poller TRONG cluster"
run bash "$E2E" zero-downtime bg 3000 bg web-svc:v2 /
note "   RS sau cutover: web-svc:dev(blue)→0, web-svc:nope→0, web-svc:v2(green)→4/4"
runsh "kubectl get rs -n $NS -l app=bg -o custom-columns='RS:.metadata.name,IMAGE:.spec.template.spec.containers[0].image,DESIRED:.spec.replicas,READY:.status.readyReplicas' --no-headers"

note "3) ROLLBACK về BLUE — rollout undo --to-revision=1 (tức thì; KHÔNG lật selector vì chỉ 1 Deployment)"
run bash "$E2E" rollback bg 1
runsh "kubectl get deploy bg -n $NS -o jsonpath='{.spec.template.spec.containers[0].image}'; echo '  <- da ve blue'"
note "   Khác 2.1: 2.1 maxSurge=1 (thay dần từng pod); 2.2 maxSurge=100% (bung cả loạt = blue/green-style)."

# ---- blue/green KINH ĐIỂN trên SERVICE CHUNG api-svc THẬT + CHỨNG MINH route đúng ----
note "═══ 4) Blue/green KINH ĐIỂN: Service chung api-svc kèm color → FLIP → chứng minh route bằng log blue/green ═══"
if kubectl get svc api-svc -n "$NS" -o jsonpath='{.spec.selector.color}' 2>/dev/null | grep -q .; then
  _bg_prove(){                                   # $1 = màu switch tới
    local c="$1" mk="bgprobe-$1-$$"
    note "→ FLIP selector Service chung sang '$c' (cutover), rồi curl vào svc + soi log nginx từng màu (marker=$mk)"
    run kubectl patch svc api-svc -n "$NS" -p "{\"spec\":{\"selector\":{\"app\":\"api-svc\",\"color\":\"$c\"}}}"
    runsh "kubectl get svc api-svc -n $NS -o jsonpath='  selector={.spec.selector}{\"\\n\"}'"
    runsh "kubectl get endpoints api-svc -n $NS 2>/dev/null | grep -v deprecat"
    note "  color=$c pod IPs (endpoints phải khớp bộ này):"
    runsh "kubectl get pods -n $NS -l app=api-svc,color=$c -o jsonpath='  {range .items[*]}{.status.podIP} {end}{\"\\n\"}'"
    kubectl delete pod bg-curl -n "$NS" --ignore-not-found >/dev/null 2>&1
    note "  curl 5 lần vào Service chung http://api-svc:8118/api/version?$mk (pod in-cluster qua kube-proxy → selector):"
    kubectl run bg-curl --image=nginx:1.27-alpine --restart=Never -n "$NS" --command -- \
      sh -c "for i in 1 2 3 4 5; do wget -qO- \"http://api-svc:8118/api/version?$mk\" >/dev/null 2>&1; done; echo curl-done" >/dev/null 2>&1
    kubectl wait --for=jsonpath='{.status.phase}'=Succeeded pod/bg-curl -n "$NS" --timeout=45s >/dev/null 2>&1 || true
    runsh "kubectl logs bg-curl -n $NS 2>/dev/null | tail -1 | sed 's/^/    /'"
    local hb hg
    hb=$(kubectl logs -n "$NS" -l app=api-svc,color=blue  -c nginx --tail=60 2>/dev/null | grep -c "$mk")
    hg=$(kubectl logs -n "$NS" -l app=api-svc,color=green -c nginx --tail=60 2>/dev/null | grep -c "$mk")
    printf '%s  → log nginx BLUE hits=%s | GREEN hits=%s  →  route ĐÚNG khi request chỉ vào màu "%s"%s\n' "$G" "$hb" "$hg" "$c" "$X"
    kubectl delete pod bg-curl -n "$NS" --ignore-not-found >/dev/null 2>&1
  }
  _bg_prove blue
  _bg_prove green
  note "→ rollback Service chung về activeColor mặc định (green)"
  run kubectl patch svc api-svc -n "$NS" -p '{"spec":{"selector":{"app":"api-svc","color":"green"}}}'
  note "  (⚠️ patch tay chỉ bền tới lần helm upgrade kế — cutover LÂU DÀI: --set global.bluegreen.activeColor=<màu>)"
else
  note "  Service chung api-svc CHƯA kèm color → cần: ${HELM:-/home/chronical/.local/bin/helm} upgrade stock $DIR/../../../helm/stock -n $NS (values global.bluegreen.activeColor). Bỏ qua phần chứng minh."
fi
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
note "5) HPA áp THẬT lên service project (gated global.hpa.enabled) — khác cpu-burn (app demo)"
HELM="${HELM:-/home/chronical/.local/bin/helm}"
CHART="$DIR/../../../helm/stock"
if kubectl get hpa gateway-svc -n "$NS" >/dev/null 2>&1; then
  run kubectl get hpa api-svc-blue api-svc-green gateway-svc -n "$NS"
  note "   Service project I/O-bound (idle 1-2% CPU) → CPU-HPA CHỈ scale khi traffic spike THẬT; thường nằm ở minReplicas (giữ HA)."
  note "   HPA sở hữu .spec.replicas ⇒ deployment OMIT replicas khi hpa.enabled (tránh flapping với helm upgrade). api-svc blue/green: HPA mỗi màu; màu idle nằm ở min."
else
  note "   HPA thật CHƯA bật. Bật: $HELM upgrade stock $CHART -n $NS --set global.hpa.enabled=true"
fi
if [ "${REAL_HPA_CYCLE:-0}" = "1" ] && [ -x "$HELM" ]; then
  note "   REAL_HPA_CYCLE=1 → demo up→down TRÊN HPA THẬT. Hạ target 60→10% (request /api/version quá rẻ, không đẩy nổi CPU 60%), bắn HTTP load, chờ scale-down, khôi phục 60%."
  runsh "$HELM upgrade stock $CHART -n $NS --set global.hpa.enabled=true --set global.hpa.apiSvc.targetCPU=10 --set global.hpa.gatewaySvc.targetCPU=10 2>&1 | grep -E 'REVISION|STATUS'"
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
  runsh "$HELM upgrade stock $CHART -n $NS --set global.hpa.enabled=true 2>&1 | grep REVISION"
  kubectl delete pod e2e-load -n "$NS" --ignore-not-found >/dev/null 2>&1
else
  note "   (full cycle up→down trên HPA thật: REAL_HPA_CYCLE=1 — mất ~8' vì window 300s + phải hạ target demo)"
fi
}

# =============================================================================
lab_2_4(){
title "LAB 2.4 — Kustomize Overlay (app cô lập 'web-kz', chỉ có kubectl-embedded Kustomize)"

note "0) Tooling: dùng 'kubectl kustomize' / 'kubectl apply -k' (không cần binary kustomize rời)"
runsh "kubectl version --client 2>/dev/null | grep -i kustomize || echo '(kubectl có Kustomize nhúng)'"

note "1) Render 2 overlay (KHÔNG apply) — chứng minh patch image tag + replica KHÁC nhau, không nhân đôi manifest"
run bash "$E2E" kustomize-diff "$DIR/kustomize/overlays/dev" "$DIR/kustomize/overlays/prod"

note "2) Xem YAML render prod (namePrefix prod- · ns stock · replicas 5 · image web-svc:v2)"
runsh "kubectl kustomize $DIR/kustomize/overlays/prod | grep -E '^kind:|  name:|replicas:|image:|namespace:'"

note "3) apply -k prod (bonus) — 5/5, cô lập khỏi frontend web-svc thật"
run kubectl apply -k "$DIR/kustomize/overlays/prod"
kubectl rollout status deploy/prod-web-kz -n "$NS" --timeout=120s || true
run kubectl get deploy,svc -n "$NS" -l app=web-kz
note "   web-svc THẬT giữ nguyên (app=web-svc ≠ app=web-kz):"
run kubectl get deploy -n "$NS" -l app=web-svc

note "4) Dọn overlay prod (delete -k)"
run kubectl delete -k "$DIR/kustomize/overlays/prod"
}

# =============================================================================
cleanup(){
  [ "${KEEP:-0}" = "1" ] && { printf '\n%sKEEP=1 → giữ lại object (web, bg, web-kz, HPA...).%s\n' "$D" "$X"; return; }
  title "DỌN DẸP (roll back — web-svc thật + cpu-burn giữ nguyên)"
  run kubectl delete -f "$DIR/web.yaml" --ignore-not-found
  run kubectl delete deploy bg -n "$NS" --ignore-not-found
  run kubectl delete svc bg -n "$NS" --ignore-not-found
  run kubectl delete -k "$DIR/kustomize/overlays/prod" --ignore-not-found
  run kubectl delete pod e2e-poller e2e-load -n "$NS" --ignore-not-found
  note "cpu-burn: giữ Deployment, scale về 1 (HPA idle sẽ giữ ở 1)"
  run kubectl scale deploy/cpu-burn --replicas=1 -n "$NS"
  note "Kiểm chứng: app demo đã sạch, web-svc thật còn nguyên:"
  run kubectl get deploy -n "$NS" -l 'app in (web,bg,web-kz)'
  run kubectl get deploy -n "$NS" -l app=web-svc
}

# ---- điều phối ---------------------------------------------------------------
case "$ONLY" in
  2.1) lab_2_1 ;;
  2.2) lab_2_2 ;;
  2.3) lab_2_3 ;;
  2.4) lab_2_4 ;;
  *)   lab_2_1; lab_2_2; lab_2_3; lab_2_4 ;;
esac
cleanup
printf '\n%s%s✔ XONG Day 2 (Lab %s).%s\n' "$B" "$G" "$ONLY" "$X"
