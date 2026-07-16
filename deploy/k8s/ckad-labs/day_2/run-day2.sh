#!/usr/bin/env bash
# =============================================================================
# CKAD — Day 2 — Deployments & rollouts. Chạy TRỌN 4 lab, in lệnh + OUTPUT THẬT.
#   2.1 Rolling Update & Rollback   (app demo 'web' nginx, cô lập)
#   2.2 Blue/Green Switch           (app demo 'bg' cô lập — KHÔNG đụng web-svc thật)
#   2.3 Scale & HPA                 (app 'cpu-burn' hpa-example)
#   2.4 Kustomize Overlay           (app 'web-kz' cô lập)
#
#   ./run-day2.sh                    # cả 4 lab, cuối tự dọn (app thật giữ nguyên)
#   ONLY=2.3 ./run-day2.sh           # chỉ 1 lab: 2.1 | 2.2 | 2.3 | 2.4
#   DUR=45 ./run-day2.sh             # thời lượng poll zero-downtime (giây, mặc định 30)
#   RUN_LOAD=0 ./run-day2.sh         # 2.3: bỏ bắn tải (chỉ attach HPA + show), nhanh
#   LOAD_SECS=120 ./run-day2.sh      # 2.3: thời lượng bắn tải (giây, mặc định 100)
#   WAIT_SCALEDOWN=1 ./run-day2.sh   # 2.3: chờ luôn cửa sổ scale-down 300s về minReplicas
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
title "LAB 2.2 — Blue/Green Switch (app cô lập 'bg' — web-svc THẬT không bị đụng)"

note "setup) 2 Deployment bg-blue(color=blue, web-svc:dev) + bg-green(color=green, web-svc:v2) + Service bg→blue"
kubectl apply -f - >/dev/null <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata: { name: bg-blue, namespace: stock, labels: { app: bg, color: blue } }
spec:
  replicas: 2
  selector: { matchLabels: { app: bg, color: blue } }
  template:
    metadata: { labels: { app: bg, color: blue } }
    spec:
      containers:
        - name: web
          image: web-svc:dev
          imagePullPolicy: IfNotPresent
          ports: [ { containerPort: 3000 } ]
          readinessProbe: { httpGet: { path: /, port: 3000 }, initialDelaySeconds: 2, periodSeconds: 3 }
---
apiVersion: apps/v1
kind: Deployment
metadata: { name: bg-green, namespace: stock, labels: { app: bg, color: green } }
spec:
  replicas: 2
  selector: { matchLabels: { app: bg, color: green } }
  template:
    metadata: { labels: { app: bg, color: green } }
    spec:
      containers:
        - name: web
          image: web-svc:v2
          imagePullPolicy: IfNotPresent
          ports: [ { containerPort: 3000 } ]
          readinessProbe: { httpGet: { path: /, port: 3000 }, initialDelaySeconds: 2, periodSeconds: 3 }
---
apiVersion: v1
kind: Service
metadata: { name: bg, namespace: stock, labels: { app: bg } }
spec:
  selector: { app: bg, color: blue }
  ports: [ { port: 3000, targetPort: 3000 } ]
EOF
run kubectl get deploy -n "$NS" -l app=bg
note "   chờ cả 2 màu Ready..."
kubectl rollout status deploy/bg-blue -n "$NS" --timeout=120s || true
kubectl rollout status deploy/bg-green -n "$NS" --timeout=120s || true

note "1) Service bg đang trỏ BLUE — endpoints = 2 IP pod blue"
run kubectl get endpoints bg -n "$NS" -o wide

note "2) FLIP selector color=green (switch tức thời; green đã Ready sẵn nên zero-downtime)"
run bash "$E2E" bluegreen bg color green
note "   → Endpoints đổi sang 2 IP pod green. Selector Deployment IMMUTABLE nên phải 2 Deployment; rollback = lật lại color=blue."
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
  [ "${KEEP:-0}" = "1" ] && { printf '\n%sKEEP=1 → giữ lại object (web, bg-*, web-kz, HPA...).%s\n' "$D" "$X"; return; }
  title "DỌN DẸP (roll back — web-svc thật + cpu-burn giữ nguyên)"
  run kubectl delete -f "$DIR/web.yaml" --ignore-not-found
  run kubectl delete deploy bg-blue bg-green -n "$NS" --ignore-not-found
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
