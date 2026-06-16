#!/bin/sh
set -e

CERT_DIR="/etc/gateway/certs"
CERT_FILE="$CERT_DIR/cert.pem"
KEY_FILE="$CERT_DIR/key.pem"
GATEWAY_UID=1001
GATEWAY_GID=1001

mkdir -p "$CERT_DIR"

if [ ! -f "$CERT_FILE" ] || [ ! -f "$KEY_FILE" ]; then
  echo "[TLS] No certs found in $CERT_DIR — generating self-signed cert..."
  openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
    -keyout "$KEY_FILE" \
    -out "$CERT_FILE" \
    -subj "/CN=localhost/O=go-stock-prediction/C=VN" \
    2>/dev/null
  echo "[TLS] Self-signed cert generated (valid 10 years). Replace with real certs for production."
fi

# Ensure the gateway user can read the certs regardless of who placed them
# (host-mounted real certs may be owned by a different uid). Runs as root.
chown "$GATEWAY_UID:$GATEWAY_GID" "$CERT_DIR" "$CERT_FILE" "$KEY_FILE" 2>/dev/null || true
chmod 600 "$KEY_FILE" 2>/dev/null || true

# Drop privileges to the unprivileged gateway user and exec the server (becomes PID 1).
exec setpriv --reuid="$GATEWAY_UID" --regid="$GATEWAY_GID" --init-groups /app/gateway
