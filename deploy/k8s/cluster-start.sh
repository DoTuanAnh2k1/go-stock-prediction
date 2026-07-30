#!/usr/bin/env bash
# =============================================================================
# BẬT lại kind cluster 'ckad' — docker start 3 node → k8s + pod tự khởi động lại.
# Pod cần ~2-3' để lên đủ. Script chờ node Ready + restart CoreDNS (đôi khi flaky
# sau start → nếu chết, wait-db không resolve 'db' được, backend kẹt Init).
#   ./cluster-start.sh
# ⚠️ Chỉ bật khi có đủ RAM (host 14.5GB sát ngưỡng với full stack + IDE — cân nhắc
#    đóng bớt IDE, hoặc hạ replicas về 1).
# =============================================================================
set -uo pipefail
NODES="ckad-control-plane ckad-worker ckad-worker2"

command -v docker >/dev/null 2>&1 || { echo "docker không có trên PATH"; exit 1; }

# ---- inotify guard ----------------------------------------------------------
# kind 3-node ~30 pod vượt trần inotify mặc định (128) → kube-proxy crashloop
# "too many open files" → networking node hỏng → pod kẹt Unknown, backend 0/2.
# Persist ở /etc/sysctl.d/99-kind-inotify.conf; đây là lưới an toàn nếu file bị xoá
# hoặc chưa nạp. Nâng TRƯỚC docker start để node lên với limit đúng.
MIN_INST=512
cur_inst="$(sysctl -n fs.inotify.max_user_instances 2>/dev/null || echo 0)"
if [ "${cur_inst:-0}" -lt "$MIN_INST" ]; then
  echo "→ inotify max_user_instances=$cur_inst (thấp) → nâng 1280/524288 (cần sudo)"
  if sudo sysctl fs.inotify.max_user_instances=1280 fs.inotify.max_user_watches=524288 >/dev/null 2>&1; then
    echo "  ✔ đã nâng inotify"
  else
    echo "  ⚠️ nâng thất bại — kube-proxy có thể crashloop 'too many open files'."
    echo "     Chạy tay: sudo sysctl fs.inotify.max_user_instances=1280 fs.inotify.max_user_watches=524288"
  fi
else
  echo "→ inotify OK (max_user_instances=$cur_inst)"
fi

echo "→ docker start $NODES"
docker start $NODES

echo "→ chờ API server + node Ready..."
for i in $(seq 1 40); do
  if kubectl get nodes >/dev/null 2>&1; then
    kubectl wait --for=condition=Ready nodes --all --timeout=15s >/dev/null 2>&1 && break
  fi
  sleep 3
done
kubectl get nodes 2>/dev/null || { echo "API server chưa sẵn sàng — thử lại sau vài giây"; exit 1; }

echo "→ restart CoreDNS (đề phòng flaky sau start)"
kubectl -n kube-system rollout restart deploy/coredns >/dev/null 2>&1 || true
kubectl -n kube-system rollout status deploy/coredns --timeout=90s 2>&1 | tail -1 || true

# ---- dọn ghost Job pod "Unknown" --------------------------------------------
# CronJob đang chạy dở lúc cluster-stop → Job pod kẹt "Unknown" sau up (owner=Job,
# KHÔNG tự heal như Deployment pod). Vô hại nhưng bẩn output. Chờ Deployment tự heal
# rồi CHỈ xoá pod owner=Job (Deployment/DaemonSet pod để kubelet tự reconnect).
echo "→ chờ pod tự heal (~45s) rồi dọn ghost Job pod 'Unknown' còn sót..."
sleep 45
_ghosts="$(kubectl get pods -n stock --no-headers 2>/dev/null | awk '$3=="Unknown"{print $1}')"
if [ -n "$_ghosts" ]; then
  for _p in $_ghosts; do
    _owner="$(kubectl get pod "$_p" -n stock -o jsonpath='{.metadata.ownerReferences[0].kind}' 2>/dev/null)"
    if [ "$_owner" = "Job" ]; then
      kubectl delete pod "$_p" -n stock --grace-period=0 --force >/dev/null 2>&1 && echo "  ✔ dọn ghost Job pod $_p"
    else
      echo "  • $_p (owner=$_owner) — để tự heal, không xoá"
    fi
  done
else
  echo "  ✔ không còn ghost 'Unknown'"
fi

echo
echo "✔ Cluster đang lên. Pod tự khởi động lại (~2-3'). Theo dõi:"
echo "   kubectl get pods -n stock -w"
echo "   (nếu pod kẹt Init:wait-db lâu → CoreDNS chưa Ready, chờ thêm hoặc rollout restart coredns lần nữa)"
