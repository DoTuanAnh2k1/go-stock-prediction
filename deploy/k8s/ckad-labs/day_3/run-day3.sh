#!/usr/bin/env bash
# =============================================================================
# CKAD — Day 3 — Configuration & security. VERIFY hardening ĐÃ áp trên stack THẬT.
#   3.1 ConfigMap & Secret Injection   (read-only: envFrom + nginx.conf volume)
#   3.2 Security Context Lockdown       (read-only: securityContext pod+container)
#   3.3 ServiceAccount & RBAC           (read-only: automount:false + can-i)
#   3.4 Namespace Quotas                (read-only + demo reject qua --dry-run=server)
#
# TẤT CẢ read-only — KHÔNG helm upgrade, KHÔNG tạo pod thật. Chỉ 3.4 dùng
# --dry-run=server (chạy admission, KHÔNG persist) để demo reject.
#
#   ./run-day3.sh                    # cả 4 lab
#   ONLY=3.2 ./run-day3.sh           # chỉ 1 lab: 3.1 | 3.2 | 3.3 | 3.4
#   KEEP=1 ./run-day3.sh             # (không sinh object nháp — KEEP không có tác dụng ở đây)
#
# Yêu cầu: context=kind-ckad, ns=stock, hardening đã helm upgrade (rev6/7/8).
#          helm KHÔNG trên PATH → dùng /home/chronical/.local/bin/helm.
# =============================================================================
set -uo pipefail          # KHÔNG set -e: 3.4 có lệnh --dry-run CỐ TÌNH lỗi (reject)

NS=stock
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ONLY="${ONLY:-all}"
HELM=/home/chronical/.local/bin/helm

# ---- màu (tắt khi không phải terminal) --------------------------------------
if [ -t 1 ]; then B=$'\e[1m'; C=$'\e[36m'; Y=$'\e[33m'; G=$'\e[32m'; R=$'\e[31m'; D=$'\e[2m'; X=$'\e[0m'
else B=; C=; Y=; G=; R=; D=; X=; fi

hr(){ printf '%s%s%s\n' "$D" "────────────────────────────────────────────────────────────────" "$X"; }
title(){ printf '\n'; hr; printf '%s%s  %s%s\n' "$B" "$C" "$1" "$X"; hr; }
note(){ printf '%s# %s%s\n' "$D" "$1" "$X"; }
run(){ printf '\n%s$' "$Y"; printf ' %s' "$@"; printf '%s\n' "$X"; "$@"; }
runsh(){ printf '\n%s$ %s%s\n' "$Y" "$1" "$X"; eval "$1"; }     # lệnh có pipe — bash-native (KHÔNG sh -c)
runx(){ # chạy lệnh MONG ĐỢI lỗi (reject/Forbidden) — heredoc gắn thẳng, KHÔNG sh -c
  printf '\n%s$' "$Y"; printf ' %s' "$@"; printf '%s\n' "$X"
  if "$@"; then printf '%s(?!) lệnh này lẽ ra phải bị API server từ chối%s\n' "$R" "$X"
  else printf '%s↑ ĐÚNG NHƯ MONG ĐỢI — API server TỪ CHỐI (Forbidden/admission)%s\n' "$G" "$X"; fi; }

# ---- prereq -----------------------------------------------------------------
ctx="$(kubectl config current-context 2>/dev/null || true)"
[ "$ctx" = "kind-ckad" ] || { printf '%sContext hiện tại = "%s" (mong đợi kind-ckad). Đổi: kubectl config use-context kind-ckad%s\n' "$R" "$ctx" "$X"; exit 1; }
kubectl get ns "$NS" >/dev/null 2>&1 || { printf '%sKhông thấy namespace %s%s\n' "$R" "$NS" "$X"; exit 1; }
[ -x "$HELM" ] || { printf '%sKhông thấy helm tại %s (Day 3 chỉ đọc history — vẫn cần cho tham chiếu)%s\n' "$R" "$HELM" "$X"; exit 1; }

# =============================================================================
lab_3_1(){
title "LAB 3.1 — ConfigMap & Secret Injection (đã hiện thực sẵn trong stack thật)"

note "1) Liệt kê ConfigMap + Secret của ns stock (nginx.conf ambassador, config, secret per-service)"
runsh "kubectl get cm,secret -n $NS | grep -E 'nginx|config|secret'"

note "2) Secret+ConfigMap → env: 'describe deploy | grep' phần Environment (dễ nhớ hơn -o jsonpath)"
runsh "kubectl describe deploy prediction-svc -n $NS | grep -A2 'Environment Variables from'"
note "   → prediction-config (ConfigMap) + prediction-secret (Secret) = envFrom nạp cả ConfigMap lẫn Secret vào env"

note "3) ConfigMap → mounted volume: nginx.conf mount subPath — 'describe deploy | grep nginx.conf'"
runsh "kubectl describe deploy prediction-svc -n $NS | grep 'nginx.conf'"
note "   → /etc/nginx/nginx.conf from nginx-conf (ConfigMap <svc>-nginx mount subPath, read-only)"
}

# =============================================================================
lab_3_2(){
title "LAB 3.2 — Security Context Lockdown (verify trên pod live — 5 backend đã hardening)"

P="$(kubectl get pod -n "$NS" -l app=prediction-svc -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)"
[ -n "$P" ] || { note "(không thấy pod prediction-svc đang chạy — skip)"; return; }
note "pod mẫu = $P"

note "1) 'describe pod' KHÔNG show securityContext container (chỉ SeccompProfile) → dùng 'get pod -o yaml | grep <field>'"
note "   (yaml + grep tên field = dễ nhớ, khỏi cú pháp jsonpath). Pod-level fsGroup/seccomp qua describe:"
runsh "kubectl describe pod $P -n $NS | grep -iE 'SeccompProfile'"
note "   Toàn bộ securityContext (pod + 3 container) qua yaml + grep tên field:"
runsh "kubectl get pod $P -n $NS -o yaml | grep -E 'securityContext:|fsGroup:|type: RuntimeDefault|runAsNonRoot:|runAsUser:|allowPrivilegeEscalation:|readOnlyRootFilesystem:|drop:|add:|- ALL|- CHOWN|- SETUID|- SETGID'"
note "   → POD: fsGroup 2000 + seccomp RuntimeDefault"
note "   → prediction-svc(app): drop ALL + runAsNonRoot + runAsUser:1000 (Dockerfile useradd -u 1000)"
note "   → nginx: drop ALL + add CHOWN/SETUID/SETGID (official image chown cache + hạ quyền worker)"
note "   → log-sidecar: drop ALL + readOnlyRootFilesystem (chỉ tail -f)"

note "2) service-mgt app = readOnlyRootFilesystem:true (+ /tmp emptyDir) — read-only rootfs THẬT"
S="$(kubectl get pod -n "$NS" -l app=service-mgt -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)"
if [ -n "$S" ]; then
  runsh "kubectl get pod $S -n $NS -o yaml | grep -E 'readOnlyRootFilesystem:|allowPrivilegeEscalation:|drop:|- ALL' | head -6"
else
  note "   (không thấy pod service-mgt — skip)"
fi
note "   → drop ALL + allowPrivilegeEscalation:false + readOnlyRootFilesystem:true"
}

# =============================================================================
lab_3_3(){
title "LAB 3.3 — ServiceAccount & RBAC (per-svc SA automount:false + pod-reader Role)"

note "1) Liệt kê ServiceAccount ns stock (mỗi backend 1 SA riêng + pod-reader + gateway-svc)"
run kubectl get sa -n "$NS"

note "2) Token KHÔNG mount — 'describe pod | grep' cho thấy Service Account + KHÔNG có volume kube-api-access-*"
P="$(kubectl get pod -n "$NS" -l app=prediction-svc -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)"
if [ -n "$P" ]; then
  runsh "kubectl describe pod $P -n $NS | grep -iE 'Service Account:|kube-api-access'"
  note "   → chỉ 'Service Account: prediction-svc', KHÔNG dòng kube-api-access-* ⇒ token KHÔNG mount"
  runsh "kubectl get sa prediction-svc -n $NS -o yaml | grep -i automount"
  note "   → automountServiceAccountToken: false (đặt ở SA object → không service nào gọi k8s API)"
else
  note "   (không thấy pod prediction-svc — skip)"
fi

note "3) RBAC oracle nhanh (can-i) + LIST toàn bộ quyền pod-reader:"
run kubectl auth can-i list pods --as=system:serviceaccount:stock:pod-reader -n "$NS"
note "   ↑ yes"
runsh "kubectl auth can-i --list --as=system:serviceaccount:stock:pod-reader -n $NS | grep -E 'Resources|^pods|^services|^endpoints'"
note "   → chỉ get/list/watch trên pods/services/endpoints = least-privilege"

note "4) DEMO THẬT bằng impersonation (--as) — KHÔNG chỉ can-i mà THỰC SỰ chạy lệnh AS the SA:"
note "   (a) get pods AS pod-reader → liệt kê ĐƯỢC (Role cho phép đọc):"
runsh "kubectl get pods --as=system:serviceaccount:stock:pod-reader -n $NS | head -4"
note "   (b) get pods AS auth-svc (SA KHÔNG bind Role) → Forbidden THẬT từ API server:"
runx kubectl get pods --as=system:serviceaccount:stock:auth-svc -n "$NS"
note "   (c) delete pod AS pod-reader → Forbidden (Role chỉ ĐỌC, không có verb delete):"
[ -n "$P" ] && runx kubectl delete pod "$P" --as=system:serviceaccount:stock:pod-reader -n "$NS" --dry-run=server
note "   → cùng 1 SA: đọc OK / xóa bị chặn; SA khác không quyền = Forbidden. RBAC enforce ở API-server admission."
}

# =============================================================================
lab_3_4(){
title "LAB 3.4 — Namespace Quotas (ResourceQuota + LimitRange trên ns stock THẬT)"

note "1) ResourceQuota stock-quota — Used/Hard mỗi dimension (đều dưới trần)"
run kubectl describe quota stock-quota -n "$NS"

note "2) LimitRange stock-defaults — defaultRequest/default/max tiêm vào pod thiếu resources"
run kubectl describe limitrange stock-defaults -n "$NS"

note "3) DEMO REJECT — --dry-run=server chạy admission thật NHƯNG không persist (không cần dọn)"
note "   (a) Vượt ResourceQuota: 2 container × request cpu=2 → tổng 4 > remaining requests.cpu"
runx kubectl create --dry-run=server -n "$NS" -f - <<EOF
apiVersion: v1
kind: Pod
metadata: { name: quota-test, namespace: $NS }
spec:
  containers:
    - { name: a, image: nginx:1.27-alpine, resources: { requests: { cpu: "2" }, limits: { cpu: "2" } } }
    - { name: b, image: nginx:1.27-alpine, resources: { requests: { cpu: "2" }, limits: { cpu: "2" } } }
EOF

note "   (b) Vượt LimitRange max: 1 container request cpu=3 (> max 2 mỗi container)"
runx kubectl create --dry-run=server -n "$NS" -f - <<EOF
apiVersion: v1
kind: Pod
metadata: { name: max-test, namespace: $NS }
spec:
  containers:
    - { name: a, image: nginx:1.27-alpine, resources: { requests: { cpu: "3" }, limits: { cpu: "3" } } }
EOF

note "   (c) Default injection (KHÔNG reject): pod KHÔNG khai resources → LimitRange TỰ VÁ default request/limit"
run kubectl create --dry-run=server -n "$NS" -o jsonpath='{.spec.containers[0].resources}' -f - <<EOF
apiVersion: v1
kind: Pod
metadata: { name: noreq-test, namespace: $NS }
spec:
  containers:
    - { name: a, image: nginx:1.27-alpine }
EOF
printf '\n'
note "   → {requests:50m/64Mi, limits:250m/256Mi} = LimitRange vá default ⇒ pod thiếu resources vẫn ADMIT (thoả quota)"

note "Điểm chốt: Quota requests.*/limits.* yêu cầu MỌI pod khai resources → phải pair LimitRange"
note "           (default injection). Reject xảy ra ở API-server admission (không phải scheduler)."
}

# =============================================================================
cleanup(){
  # Day 3 read-only — không sinh object nháp (chỉ --dry-run=server, không persist).
  [ "${KEEP:-0}" = "1" ] && { printf '\n%sKEEP=1 (Day 3 read-only — không có gì để giữ/dọn).%s\n' "$D" "$X"; return; }
  note "Day 3 read-only: không tạo object thật (chỉ --dry-run=server) → không cần dọn."
}

# ---- điều phối --------------------------------------------------------------
case "$ONLY" in
  3.1) lab_3_1 ;;
  3.2) lab_3_2 ;;
  3.3) lab_3_3 ;;
  3.4) lab_3_4 ;;
  *)   lab_3_1; lab_3_2; lab_3_3; lab_3_4 ;;
esac
cleanup
printf '\n%s%s✔ XONG Day 3 (Lab %s).%s\n' "$B" "$G" "$ONLY" "$X"
