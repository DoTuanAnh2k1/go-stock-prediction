#!/bin/bash
# gen-letsencrypt.sh — obtain Let's Encrypt cert via Certbot standalone
# Prerequisites: certbot installed (apt install certbot / brew install certbot)
# Port 80 must be reachable from the internet during cert issuance.
# Usage: ./scripts/gen-letsencrypt.sh <domain> [email]

set -euo pipefail

DOMAIN="${1:-}"
EMAIL="${2:-}"

if [ -z "$DOMAIN" ]; then
  echo "Usage: $0 <domain> [email]"
  echo "Example: $0 stock.example.com admin@example.com"
  exit 1
fi

CERT_DIR="$(cd "$(dirname "$0")/.." && pwd)/nginx/certs"
mkdir -p "$CERT_DIR"

# Stop frontend temporarily — certbot needs port 80
echo "[TLS] Stopping frontend container to free port 80..."
docker compose stop frontend 2>/dev/null || true

EMAIL_FLAG="--register-unsafely-without-email"
[ -n "$EMAIL" ] && EMAIL_FLAG="--email $EMAIL"

echo "[TLS] Running certbot for $DOMAIN ..."
certbot certonly --standalone --non-interactive --agree-tos \
  $EMAIL_FLAG -d "$DOMAIN"

# Copy to nginx/certs/
cp "/etc/letsencrypt/live/$DOMAIN/fullchain.pem" "$CERT_DIR/cert.pem"
cp "/etc/letsencrypt/live/$DOMAIN/privkey.pem"   "$CERT_DIR/key.pem"

echo "[TLS] Done. Certs written to nginx/certs/"
echo ""
echo "Set in .env:  ALLOWED_ORIGINS=https://$DOMAIN"
echo "Then:         docker compose start frontend"
echo ""
echo "Auto-renewal crontab (run as root):"
echo "  0 3 * * * certbot renew --quiet && \\"
echo "    cp /etc/letsencrypt/live/$DOMAIN/fullchain.pem $CERT_DIR/cert.pem && \\"
echo "    cp /etc/letsencrypt/live/$DOMAIN/privkey.pem $CERT_DIR/key.pem && \\"
echo "    docker compose restart frontend"
