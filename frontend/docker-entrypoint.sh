#!/bin/sh
set -e

CERT_DIR="/etc/nginx/certs"
CERT_FILE="$CERT_DIR/cert.pem"
KEY_FILE="$CERT_DIR/key.pem"

if [ ! -f "$CERT_FILE" ] || [ ! -f "$KEY_FILE" ]; then
  echo "[TLS] No certificates found in $CERT_DIR — generating self-signed cert..."
  openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
    -keyout "$KEY_FILE" \
    -out "$CERT_FILE" \
    -subj "/CN=localhost/O=go-stock-prediction/C=VN" \
    2>/dev/null
  chmod 600 "$KEY_FILE"
  echo "[TLS] Self-signed cert generated (valid 10 years). Replace with real certs for production."
fi

exec nginx -g "daemon off;"
