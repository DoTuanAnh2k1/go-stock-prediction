#!/bin/bash
# gen-cloudflare-origin.sh — install a Cloudflare Origin Certificate
#
# Cloudflare Origin Certs are issued by Cloudflare CA (not public CA).
# They are only trusted by Cloudflare's proxy — not by browsers directly.
# Set SSL/TLS mode to "Full (strict)" in Cloudflare dashboard.
#
# Usage (copy downloaded files):
#   ./scripts/gen-cloudflare-origin.sh /path/to/cert.pem /path/to/key.pem
#
# Manual steps (if not passing files):
#   1. Cloudflare Dashboard → your domain → SSL/TLS → Origin Server
#   2. Create Certificate → RSA 2048 → validity up to 15 years
#   3. Copy "Origin Certificate" → save locally as cert.pem
#   4. Copy "Private Key"        → save locally as key.pem
#   5. Run: ./scripts/gen-cloudflare-origin.sh cert.pem key.pem
#   6. Cloudflare SSL/TLS mode → set to "Full (strict)"

set -euo pipefail

CERT_SRC="${1:-}"
KEY_SRC="${2:-}"
CERT_DIR="$(cd "$(dirname "$0")/.." && pwd)/gateway-svc/certs"
mkdir -p "$CERT_DIR"

if [ -z "$CERT_SRC" ] || [ -z "$KEY_SRC" ]; then
  echo "Usage: $0 <cert_file> <key_file>"
  echo ""
  echo "Manual steps:"
  echo "  1. Cloudflare Dashboard → your domain → SSL/TLS → Origin Server"
  echo "  2. Create Certificate → RSA 2048 → validity up to 15 years"
  echo "  3. Save Origin Certificate  → some local file (e.g. ~/cf-cert.pem)"
  echo "  4. Save Private Key         → some local file (e.g. ~/cf-key.pem)"
  echo "  5. Run: $0 ~/cf-cert.pem ~/cf-key.pem"
  echo "  6. Set Cloudflare SSL/TLS mode to 'Full (strict)'"
  echo "  7. docker compose restart gateway"
  exit 0
fi

cp "$CERT_SRC" "$CERT_DIR/cert.pem"
cp "$KEY_SRC"  "$CERT_DIR/key.pem"
chmod 600 "$CERT_DIR/key.pem"

echo "[TLS] Cloudflare Origin Certificate installed to gateway-svc/certs/"
echo ""
echo "Ensure Cloudflare SSL/TLS encryption mode is set to 'Full (strict)'"
echo "Then: docker compose restart gateway"
