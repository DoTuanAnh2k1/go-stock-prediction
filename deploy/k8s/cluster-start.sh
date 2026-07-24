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

echo
echo "✔ Cluster đang lên. Pod tự khởi động lại (~2-3'). Theo dõi:"
echo "   kubectl get pods -n stock -w"
echo "   (nếu pod kẹt Init:wait-db lâu → CoreDNS chưa Ready, chờ thêm hoặc rollout restart coredns lần nữa)"
