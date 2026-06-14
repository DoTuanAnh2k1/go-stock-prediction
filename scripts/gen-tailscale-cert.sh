#!/bin/bash
# gen-tailscale-cert.sh — obtain HTTPS cert from Tailscale MagicDNS
# Prerequisites: tailscale installed and authenticated (tailscale up)
# Usage: ./scripts/gen-tailscale-cert.sh

set -euo pipefail

CERT_DIR="$(cd "$(dirname "$0")/.." && pwd)/nginx/certs"
mkdir -p "$CERT_DIR"

# Detect Tailscale hostname
TAILSCALE_HOST=$(tailscale status --json | python3 -c "
import json, sys
d = json.load(sys.stdin)
print(d.get('Self', {}).get('DNSName', '').rstrip('.'))
" 2>/dev/null || true)

if [ -z "$TAILSCALE_HOST" ]; then
  echo "ERROR: Could not detect Tailscale hostname. Is tailscale running?"
  exit 1
fi

echo "[TLS] Obtaining cert for $TAILSCALE_HOST via Tailscale..."
tailscale cert \
  --cert-file "$CERT_DIR/cert.pem" \
  --key-file  "$CERT_DIR/key.pem" \
  "$TAILSCALE_HOST"

echo "[TLS] Done. Certs written to nginx/certs/"
echo ""
echo "Set in .env:  ALLOWED_ORIGINS=https://$TAILSCALE_HOST"
echo "Then:         docker compose restart frontend"
