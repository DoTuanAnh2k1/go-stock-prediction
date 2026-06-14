#!/bin/bash
# gen-self-signed.sh — generate self-signed TLS cert
# Use for: local dev, Tailscale internal network, home router public IP
# Usage:   ./scripts/gen-self-signed.sh [hostname]
#          hostname defaults to "localhost"

set -euo pipefail

HOSTNAME="${1:-localhost}"
CERT_DIR="$(cd "$(dirname "$0")/.." && pwd)/nginx/certs"
mkdir -p "$CERT_DIR"

echo "[TLS] Generating self-signed cert for CN=$HOSTNAME ..."
openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
  -keyout "$CERT_DIR/key.pem" \
  -out    "$CERT_DIR/cert.pem" \
  -subj   "/CN=$HOSTNAME/O=go-stock-prediction/C=VN" \
  -addext "subjectAltName=DNS:$HOSTNAME,DNS:localhost,IP:127.0.0.1"

echo "[TLS] Done. Certs written to nginx/certs/"
echo "      $(openssl x509 -noout -enddate -in "$CERT_DIR/cert.pem")"
echo ""
echo "NOTE: Browsers warn on self-signed certs."
echo "      Chrome workaround: chrome://flags/#allow-insecure-localhost"
echo ""
echo "Next: docker-compose restart frontend"
