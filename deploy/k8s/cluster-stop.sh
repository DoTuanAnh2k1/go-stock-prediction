#!/usr/bin/env bash
# =============================================================================
# TẮT kind cluster 'ckad' — docker stop 3 node container → giải phóng RAM/CPU.
# State (PVC, deployment, image cache...) GIỮ NGUYÊN. Bật lại: ./cluster-start.sh
#   ./cluster-stop.sh
# =============================================================================
set -uo pipefail
NODES="ckad-control-plane ckad-worker ckad-worker2"

command -v docker >/dev/null 2>&1 || { echo "docker không có trên PATH"; exit 1; }
ram(){ free -h | awk '/Mem:/{m=$3" used, "$7" avail"} /Swap:/{s=$3" used"} END{print "RAM: "m" | Swap: "s}'; }

echo "TRƯỚC: $(ram)"
echo "→ docker stop $NODES"
docker stop $NODES
echo
echo "✔ Cluster 'ckad' ĐÃ TẮT (container nằm im, state giữ nguyên)."
echo "SAU:   $(ram)"
echo "→ Bật lại khi cần: $(dirname "$0")/cluster-start.sh"
