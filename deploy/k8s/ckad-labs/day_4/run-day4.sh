#!/usr/bin/env bash
# =============================================================================
# CKAD — Day 4 — Networking & storage. Áp/verify trên stack THẬT (ns stock).
#   4.1 ClusterIP & NodePort        (drill throwaway 'ep-demo' selector mismatch)
#   4.2 Ingress Routing             (gated Helm template; port-forward controller)
#   4.3 NetworkPolicy Isolation     (gated; bật → demo isolation → TẮT LẠI)
#   4.4 Persistent Volume Claims    (PVC nháp 'ck-data' + writer/reader → persist)
#
#   ./run-day4.sh                    # cả 4 lab, cuối tự dọn object nháp
#   ONLY=4.3 ./run-day4.sh           # chỉ 1 lab: 4.1 | 4.2 | 4.3 | 4.4
#   KEEP=1 ./run-day4.sh             # GIỮ object nháp (ep-demo, ck-data...) — không dọn
#
# Yêu cầu: context=kind-ckad, ns=stock. helm KHÔNG trên PATH → /home/chronical/.local/bin/helm.
#   4.2/4.3 BẬT gated Helm feature (ingress/networkPolicy) rồi 4.3 TẮT LẠI netpol (an toàn
#   metrics scrape). 4.2 cần ingress-nginx controller (nếu chưa cài → in hướng dẫn + skip).
# =============================================================================
set -uo pipefail          # KHÔNG set -e: có curl/wait cố tình fail (endpoint rỗng, netpol chặn)

NS=stock
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$DIR/../../../.." && pwd)"           # repo root
CHART="$ROOT/deploy/helm/stock"
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
HELMLOG="${TMPDIR:-/tmp}/ckad-day4-helm.log"; : > "$HELMLOG"
runhelm(){  # helm output rất dài (NOTES/manifest) → hiện ~8 dòng đầu, full ra $HELMLOG
  printf '\n%s$' "$Y"; printf ' %s' "$@"; printf '%s\n' "$X"
  local out; out="$("$@" 2>&1)"
  printf '%s\n' "$out" | head -8
  { printf '\n===== $ %s =====\n' "$*"; printf '%s\n' "$out"; } >> "$HELMLOG"
  printf '%s   … (%s dòng — full output ở %s)%s\n' "$D" "$(printf '%s\n' "$out" | wc -l | tr -d ' ')" "$HELMLOG" "$X"
}

# ---- prereq -----------------------------------------------------------------
ctx="$(kubectl config current-context 2>/dev/null || true)"
[ "$ctx" = "kind-ckad" ] || { printf '%sContext hiện tại = "%s" (mong đợi kind-ckad). Đổi: kubectl config use-context kind-ckad%s\n' "$R" "$ctx" "$X"; exit 1; }
kubectl get ns "$NS" >/dev/null 2>&1 || { printf '%sKhông thấy namespace %s%s\n' "$R" "$NS" "$X"; exit 1; }
[ -x "$HELM" ] || { printf '%sKhông thấy helm tại %s%s\n' "$R" "$HELM" "$X"; exit 1; }
[ -d "$CHART" ] || { printf '%sKhông thấy Helm chart tại %s%s\n' "$R" "$CHART" "$X"; exit 1; }

# =============================================================================
lab_4_1(){
title "LAB 4.1 — ClusterIP & NodePort (Service thật + drill selector mismatch)"

note "1) Service THẬT trên stack — TYPE + PORT + NODEPORT (ClusterIP nội bộ / NodePort expose)"
runsh "kubectl get svc -n $NS -o custom-columns='NAME:.metadata.name,TYPE:.spec.type,PORT:.spec.ports[0].port,NODEPORT:.spec.ports[0].nodePort'"

note "2) DRILL selector mismatch (throwaway ep-demo — dọn cuối)"
kubectl delete deploy,svc ep-demo -n "$NS" --ignore-not-found >/dev/null 2>&1
run kubectl create deploy ep-demo --image=nginx:1.27-alpine -n "$NS"
note "   Service selector SAI (app=WRONG) → endpoints RỖNG:"
kubectl expose deploy ep-demo -n "$NS" --port=80 --selector=app=WRONG >/dev/null 2>&1
kubectl rollout status deploy/ep-demo -n "$NS" --timeout=90s >/dev/null 2>&1 || true
run kubectl get endpoints ep-demo -n "$NS"
note "   → ENDPOINTS <none>: selector không khớp pod nào ⇒ nguyên nhân #1 'service không tới được'"

note "3) FIX: patch selector về app=ep-demo → Endpoints controller nhồi IP:port pod Ready"
run kubectl patch svc ep-demo -n "$NS" -p '{"spec":{"selector":{"app":"ep-demo"}}}'
sleep 2
run kubectl get endpoints ep-demo -n "$NS"
note "   → ENDPOINTS có IP:80 = pod khớp selector + Ready"
}

# =============================================================================
lab_4_2(){
title "LAB 4.2 — Ingress Routing (gated Helm template + ingress-nginx controller)"

note "1) Kiểm tra ingress-nginx controller đã cài chưa"
run kubectl get pods -n ingress-nginx
_ctrl="$(kubectl get pods -n ingress-nginx -l app.kubernetes.io/component=controller \
  -o jsonpath='{.items[?(@.status.phase=="Running")].metadata.name}' 2>/dev/null)"
if [ -z "$_ctrl" ]; then
  note "   → CHƯA có ingress controller Running. Cài (baremetal/NodePort vì kind không map hostPort 80/443):"
  note "     kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.12.1/deploy/static/provider/baremetal/deploy.yaml"
  note "     (chờ controller 1/1 Running + 2 admission Job Completed) rồi chạy lại ONLY=4.2."
  note "   → SKIP phần verify routing (thiếu controller)."
  return
fi

note "2) Bật Ingress (gated template ingress.yaml: host stock.local, / → web-svc, /api → api-svc)"
runhelm "$HELM" upgrade stock "$CHART" -n "$NS" --reuse-values --set global.ingress.enabled=true
run kubectl get ingress -n "$NS"

note "3) Verify routing — kind nodeIP không routable từ host → port-forward controller (background)"
printf '%s$ kubectl port-forward -n ingress-nginx svc/ingress-nginx-controller 18080:80  (nền, log /tmp/day4-pf.log)%s\n' "$Y" "$X"
# Chạy TRỰC TIẾP (KHÔNG qua hàm run) + redirect ra file → process con KHÔNG giữ pipe stdout của
# script (nếu không, tail chờ EOF mãi = task "treo giả"). $! = PID kubectl THẬT (bọc `run ... &`
# thì $! là PID subshell của run → kill hụt kubectl bên trong → orphan port-forward giữ pipe).
kubectl port-forward -n ingress-nginx svc/ingress-nginx-controller 18080:80 >/tmp/day4-pf.log 2>&1 &
_pf=$!
note "   chờ port-forward sẵn sàng (retry-loop curl, KHÔNG sleep cứng)..."
_ok=0
for _i in $(seq 1 20); do
  _code="$(curl -s -o /dev/null -w '%{http_code}' -H 'Host: stock.local' http://localhost:18080/api/version 2>/dev/null || true)"
  [ "$_code" = "200" ] && { _ok=1; break; }
  sleep 1
done
if [ "$_ok" = 1 ]; then
  note "   (a) curl qua Ingress vào /api/version (route THẬT) — kỳ vọng HTTP 200:"
  runsh "curl -s -H 'Host: stock.local' http://localhost:18080/api/version -o /dev/null -w '   → HTTP %{http_code}  (Ingress route /api → api-svc:8118, ingressClassName=nginx)\\n'"
  _mk="ingressprobe-$$"
  note "   (b) VERIFY bằng APP LOG (như Day 2): curl PATH độc nhất qua Ingress rồi soi log container 'api-svc':"
  runsh "curl -s -H 'Host: stock.local' http://localhost:18080/api/version-$_mk -o /dev/null -w '   \$ curl -H \"Host: stock.local\" http://localhost:18080/api/version-$_mk  → HTTP %{http_code}\\n'"
  sleep 1
  note "   dòng APP LOG container 'api-svc' khớp marker = request ĐI QUA Ingress → Service api-svc → pod app THẬT:"
  runsh "kubectl logs -n $NS -l app=api-svc -c api-svc --tail=100 2>/dev/null | grep '$_mk' | tail -1 | sed 's/\\x1b\\[[0-9;]*m//g; s/^/     /'"
  note "   → thấy 'path=/api/version-$_mk' trong log app ⇒ chứng minh Ingress→api-svc app (không chỉ HTTP 200 ở edge)"
else
  note "   → port-forward chưa sẵn sàng sau 20s (curl != 200). Controller có thể còn khởi động."
fi
kill "$_pf" 2>/dev/null || true
wait "$_pf" 2>/dev/null || true
pkill -f 'port-forward -n ingress-nginx svc/ingress-nginx-controller 18080:80' 2>/dev/null || true
note "   → đã kill port-forward (kèm pkill fallback, không để orphan giữ pipe)"
}

# =============================================================================
lab_4_3(){
title "LAB 4.3 — NetworkPolicy Isolation (kindnet ENFORCE thật — bật → demo → TẮT LẠI)"

note "1) Bật NetworkPolicy (+ ingress để giữ /api tới được) — 5 policy default-deny + allow-*"
runhelm "$HELM" upgrade stock "$CHART" -n "$NS" --reuse-values --set global.networkPolicy.enabled=true --set global.ingress.enabled=true
run kubectl get networkpolicy -n "$NS"

note "2) ISOLATION — pod lạ (app=np-test, KHÔNG trong allow-list) curl api-svc → BỊ CHẶN (timeout)"
kubectl delete pod np-test -n "$NS" --ignore-not-found >/dev/null 2>&1
run kubectl run np-test --image=nginx:1.27-alpine --restart=Never -n "$NS" --labels=app=np-test \
  --command -- sh -c 'wget -T 5 -qO- http://api-svc:8118/health/ready; echo wget_exit=$?'
note "   chờ pod np-test Succeeded/Failed (wget timeout ~5s)..."
_t=0
while [ "$_t" -lt 40 ]; do
  _ph="$(kubectl get pod np-test -n "$NS" -o jsonpath='{.status.phase}' 2>/dev/null)"
  [ "$_ph" = "Succeeded" ] || [ "$_ph" = "Failed" ] && break
  sleep 3; _t=$((_t+3))
done
run kubectl logs np-test -n "$NS"
note "   → 'download timed out' + wget_exit=1 = default-deny chặn app=np-test (không có allow) — chặn THẬT"
kubectl delete pod np-test -n "$NS" --ignore-not-found >/dev/null 2>&1

note "3) TẮT LẠI NetworkPolicy (restore state an toàn — default-deny cắt nhầm Prometheus scrape :9464)"
runhelm "$HELM" upgrade stock "$CHART" -n "$NS" --reuse-values --set global.networkPolicy.enabled=false --set global.ingress.enabled=true
run kubectl get networkpolicy -n "$NS"
note "   → networkpolicy đã gỡ (No resources / rỗng) = metrics scrape không bị chặn"
}

# =============================================================================
lab_4_4(){
title "LAB 4.4 — Persistent Volume Claims (PVC nháp ck-data + writer/reader → persist)"

note "setup) dọn cũ cho idempotent"
kubectl delete pod pvc-writer pvc-reader -n "$NS" --ignore-not-found >/dev/null 2>&1
kubectl delete pvc ck-data -n "$NS" --ignore-not-found >/dev/null 2>&1

note "1) PVC 1Gi (RWO, StorageClass standard = local-path WaitForFirstConsumer) + writer pod ghi /data/proof.txt"
kubectl apply -f - >/dev/null <<'EOF'
apiVersion: v1
kind: PersistentVolumeClaim
metadata: { name: ck-data, namespace: stock }
spec:
  accessModes: [ReadWriteOnce]
  resources: { requests: { storage: 1Gi } }
  storageClassName: standard
---
apiVersion: v1
kind: Pod
metadata: { name: pvc-writer, namespace: stock, labels: { app: ck-pvc } }
spec:
  restartPolicy: Never
  containers:
    - name: writer
      image: nginx:1.27-alpine
      command: ["sh","-c","echo ckad-4.4-persisted-$(date +%H:%M:%S) > /data/proof.txt; cat /data/proof.txt"]
      volumeMounts: [{ name: data, mountPath: /data }]
  volumes: [{ name: data, persistentVolumeClaim: { claimName: ck-data } }]
EOF
note "   chờ writer Succeeded (PVC Bound KHI pod schedule — WaitForFirstConsumer)..."
kubectl wait --for=jsonpath='{.status.phase}'=Succeeded pod/pvc-writer -n "$NS" --timeout=120s >/dev/null 2>&1 || true
run kubectl get pvc ck-data -n "$NS"
run kubectl logs pvc-writer -n "$NS"

note "2) XÓA writer pod (PVC/PV giữ nguyên — vòng đời TÁCH khỏi pod)"
run kubectl delete pod pvc-writer -n "$NS"

note "3) reader pod mount CÙNG PVC (ck-data) → data còn nguyên = PERSIST qua vòng đời pod"
kubectl apply -f - >/dev/null <<'EOF'
apiVersion: v1
kind: Pod
metadata: { name: pvc-reader, namespace: stock, labels: { app: ck-pvc } }
spec:
  restartPolicy: Never
  containers:
    - name: reader
      image: nginx:1.27-alpine
      command: ["sh","-c","echo READ-BACK: $(cat /data/proof.txt)"]
      volumeMounts: [{ name: data, mountPath: /data }]
  volumes: [{ name: data, persistentVolumeClaim: { claimName: ck-data } }]
EOF
kubectl wait --for=jsonpath='{.status.phase}'=Succeeded pod/pvc-reader -n "$NS" --timeout=120s >/dev/null 2>&1 || true
run kubectl logs pvc-reader -n "$NS"
note "   → 'READ-BACK: ckad-4.4-persisted-...' = DATA PERSIST (khác emptyDir chết theo pod)"
}

# =============================================================================
cleanup(){
  [ "${KEEP:-0}" = "1" ] && { printf '\n%sKEEP=1 → giữ object nháp (ep-demo, ck-data, pvc-reader...).%s\n' "$D" "$X"; return; }
  title "DỌN DẸP (object nháp — stack thật + gated feature giữ nguyên state)"
  run kubectl delete deploy,svc ep-demo -n "$NS" --ignore-not-found
  run kubectl delete pod np-test pvc-writer pvc-reader -n "$NS" --ignore-not-found
  run kubectl delete pvc ck-data -n "$NS" --ignore-not-found
  note "Kiểm chứng sạch (rỗng = OK):"
  run kubectl get pods,pvc -n "$NS" -l 'app in (ep-demo,np-test,ck-pvc)'
}

# ---- điều phối --------------------------------------------------------------
case "$ONLY" in
  4.1) lab_4_1 ;;
  4.2) lab_4_2 ;;
  4.3) lab_4_3 ;;
  4.4) lab_4_4 ;;
  *)   lab_4_1; lab_4_2; lab_4_3; lab_4_4 ;;
esac

# ---- chờ Enter rồi mới DỌN DẸP (cho quan sát object nháp: ep-demo, ck-data PVC + pvc-reader...) ----
if [ "${KEEP:-0}" = "1" ]; then
  printf '\n%sKEEP=1 → giữ nguyên object nháp, KHÔNG dọn.%s\n' "$D" "$X"
elif [ -t 0 ]; then
  printf '\n%s⏸  Object nháp đang GIỮ để quan sát (ep-demo, ck-data PVC + pvc-reader...).%s\n' "$Y" "$X"
  printf '%s   → Bấm ENTER để DỌN DẸP (Ctrl-C để giữ nguyên)... %s' "$Y" "$X"
  read -r _ || true
  cleanup
else
  printf '\n%s(stdin không phải TTY → dọn dẹp ngay, không chờ Enter)%s\n' "$D" "$X"
  cleanup
fi
printf '\n%s%s✔ XONG Day 4 (Lab %s).%s\n' "$B" "$G" "$ONLY" "$X"
