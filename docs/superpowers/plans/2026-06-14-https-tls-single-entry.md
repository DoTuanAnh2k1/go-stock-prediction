# HTTPS/TLS Single Entry Point Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Consolidate frontend + API into a single HTTPS entry point by removing the redundant `nginx` container and upgrading the `frontend` nginx to handle TLS termination (port 443) with HTTP→HTTPS redirect (port 80), while providing cert generation scripts for all deployment scenarios.

**Architecture:** The `frontend` container's nginx becomes the sole public entry point — serving the React SPA and proxying `/api/*` to `api:8118`, with TLS certificates mounted from `nginx/certs/` on the host. A custom `docker-entrypoint.sh` auto-generates self-signed certs on first startup if none are present, so `docker-compose up` works out of the box. The redundant `nginx` container is removed from docker-compose.

**Tech Stack:** nginx:alpine (already in frontend Dockerfile), openssl (included in alpine), Docker bind-mount for cert dir, bash scripts for per-scenario cert generation.

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `nginx/certs/.gitkeep` | Create | Placeholder so git tracks the certs directory |
| `nginx/certs/.gitignore` | Create | Ignore actual `.pem`/`.key` files |
| `frontend/docker-entrypoint.sh` | Create | Auto-generate self-signed certs if missing, then exec nginx |
| `frontend/nginx.conf` | Modify | HTTPS server block (443) + HTTP→HTTPS redirect (80) |
| `frontend/Dockerfile` | Modify | `EXPOSE 80 443`, copy entrypoint, set `ENTRYPOINT` |
| `docker-compose.yaml` | Modify | Remove `nginx` service; frontend ports 80+443; mount `./nginx/certs` |
| `.env.example` | Modify | Update `ALLOWED_ORIGINS` default to `https://localhost` |
| `scripts/gen-self-signed.sh` | Create | Self-signed cert for dev / Tailscale internal / home router |
| `scripts/gen-tailscale-cert.sh` | Create | Tailscale MagicDNS HTTPS cert via `tailscale cert` |
| `scripts/gen-letsencrypt.sh` | Create | Let's Encrypt via Certbot standalone (domain / cloud) |
| `scripts/gen-cloudflare-origin.sh` | Create | Cloudflare Origin Certificate installer + step-by-step guide |

---

### Task 1: Create nginx/certs directory

**Files:**
- Create: `nginx/certs/.gitkeep`
- Create: `nginx/certs/.gitignore`

- [ ] **Step 1: Create directory and placeholder**

```bash
mkdir -p nginx/certs
touch nginx/certs/.gitkeep
```

- [ ] **Step 2: Write `.gitignore` to exclude actual cert files**

Create `nginx/certs/.gitignore`:
```
*.pem
*.crt
*.key
*.csr
```

- [ ] **Step 3: Commit**

```bash
git add nginx/certs/.gitkeep nginx/certs/.gitignore
git commit -m "chore: add nginx/certs directory for TLS certificate storage"
```

---

### Task 2: Create docker-entrypoint.sh

**Files:**
- Create: `frontend/docker-entrypoint.sh`

- [ ] **Step 1: Write the entrypoint script**

Create `frontend/docker-entrypoint.sh`:
```bash
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
  echo "[TLS] Self-signed cert generated (valid 10 years). Replace with real certs for production."
fi

exec nginx -g "daemon off;"
```

- [ ] **Step 2: Make it executable**

```bash
chmod +x frontend/docker-entrypoint.sh
```

- [ ] **Step 3: Verify script syntax**

```bash
bash -n frontend/docker-entrypoint.sh && echo "Syntax OK"
```
Expected: `Syntax OK`

- [ ] **Step 4: Commit**

```bash
git add frontend/docker-entrypoint.sh
git commit -m "feat(frontend): add docker-entrypoint for auto self-signed TLS cert on first start"
```

---

### Task 3: Update frontend/nginx.conf for HTTPS

**Files:**
- Modify: `frontend/nginx.conf`

- [ ] **Step 1: Replace nginx.conf with HTTPS-enabled config**

Replace entire contents of `frontend/nginx.conf`:
```nginx
# Redirect HTTP → HTTPS
server {
  listen 80;
  server_name _;
  return 301 https://$host$request_uri;
}

# HTTPS entry point — serves React SPA and proxies /api/*
server {
  listen 443 ssl;
  server_name _;

  ssl_certificate     /etc/nginx/certs/cert.pem;
  ssl_certificate_key /etc/nginx/certs/key.pem;
  ssl_protocols       TLSv1.2 TLSv1.3;
  ssl_ciphers         HIGH:!aNULL:!MD5;
  ssl_session_cache   shared:SSL:10m;
  ssl_session_timeout 10m;

  root  /usr/share/nginx/html;
  index index.html;

  # Security headers applied to all responses
  add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
  add_header X-Frame-Options            "DENY"                               always;
  add_header X-Content-Type-Options     "nosniff"                            always;
  add_header Referrer-Policy            "strict-origin-when-cross-origin"    always;

  # Proxy API calls to Go backend
  location /api/ {
    proxy_pass http://api:8118;
    proxy_set_header Host              $host;
    proxy_set_header X-Real-IP         $remote_addr;
    proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto https;

    proxy_connect_timeout 10s;
    proxy_read_timeout    600s;
    proxy_send_timeout    600s;
  }

  # SPA fallback — all routes serve index.html
  location / {
    try_files $uri $uri/ /index.html;
  }
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/nginx.conf
git commit -m "feat(frontend): HTTPS server block on 443 + HTTP redirect on 80"
```

---

### Task 4: Update frontend/Dockerfile

**Files:**
- Modify: `frontend/Dockerfile`

- [ ] **Step 1: Update Dockerfile to expose 443 and use custom entrypoint**

Replace entire contents of `frontend/Dockerfile`:
```dockerfile
# Stage 1: build
FROM node:22-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
RUN npm run build

# Stage 2: serve
FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh
EXPOSE 80 443
ENTRYPOINT ["/docker-entrypoint.sh"]
```

- [ ] **Step 2: Commit**

```bash
git add frontend/Dockerfile
git commit -m "feat(frontend): expose 443, switch to docker-entrypoint.sh for TLS init"
```

---

### Task 5: Update docker-compose.yaml

**Files:**
- Modify: `docker-compose.yaml`

- [ ] **Step 1: Remove `nginx` service, update `frontend` service ports and volume**

Replace entire contents of `docker-compose.yaml`:
```yaml
version: "3.8"

services:
  # Database
  db:
    image: mysql:8.0
    container_name: mysql_db
    command: --default-authentication-plugin=mysql_native_password
    volumes:
      - db_data:/var/lib/mysql
      - ./database.sql:/docker-entrypoint-initdb.d/init.sql:ro
    restart: always
    environment:
      MYSQL_ROOT_PASSWORD: ${MYSQL_ROOT_PASSWORD}
      MYSQL_DATABASE: go_stock_prediction
      MYSQL_USER: dbuser
      MYSQL_PASSWORD: ${MYSQL_PASSWORD}
    healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "localhost", "-u", "root", "-p${MYSQL_ROOT_PASSWORD}"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 40s
    networks:
      - app-network

  # Prediction Service (gRPC) — Python implementation
  prediction:
    build:
      context: .
      dockerfile: prediction/Dockerfile
    container_name: prediction_service
    depends_on:
      db:
        condition: service_healthy
    restart: unless-stopped
    environment:
      GRPC_SERVER_PORT: "8119"
      DB_DRIVER: mysql
      MYSQL_HOST: db
      MYSQL_PORT: 3306
      MYSQL_USER: root
      MYSQL_PASSWORD: ${MYSQL_ROOT_PASSWORD}
      MYSQL_DB_NAME: go_stock_prediction
      MYSQL_DEBUG: "true"
      LOG_LEVEL: DEBUG
      BACKUP_DIR: "/backups"
      ALLOWED_ORIGINS: "${ALLOWED_ORIGINS:-https://localhost}"
    volumes:
      - backup_data:/backups
    networks:
      - app-network

  # API Backend (HTTP)
  api:
    build:
      context: ./api
      dockerfile: Dockerfile
    container_name: api_backend
    depends_on:
      db:
        condition: service_healthy
      prediction:
        condition: service_started
    restart: unless-stopped
    environment:
      SERVER_PORT: "8118"
      GRPC_TARGET: "prediction:8119"
      DB_DRIVER: mysql
      MYSQL_HOST: db
      MYSQL_PORT: 3306
      MYSQL_USER: root
      MYSQL_PASSWORD: ${MYSQL_ROOT_PASSWORD}
      MYSQL_DB_NAME: go_stock_prediction
      MYSQL_DEBUG: "${MYSQL_DEBUG:-false}"
      LOG_LEVEL: "${LOG_LEVEL:-INFO}"
      DB_LOG_LEVEL: "${DB_LOG_LEVEL:-WARN}"
      API_KEY: ""
      ADMIN_USERNAME: "admin"
      ADMIN_PASSWORD: "${ADMIN_PASSWORD}"
      JWT_SECRET: "${JWT_SECRET}"
      BACKUP_DIR: "/backups"
      ALLOWED_ORIGINS: "${ALLOWED_ORIGINS:-https://localhost}"
    volumes:
      - backup_data:/backups
    networks:
      - app-network

  # Frontend (React + Vite + Nginx) — single HTTPS gateway on :443
  frontend:
    build:
      context: ./frontend
      dockerfile: Dockerfile
    container_name: vnstock_frontend
    depends_on:
      - api
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx/certs:/etc/nginx/certs
    networks:
      - app-network

  # phpMyAdmin
  phpmyadmin:
    depends_on:
      - db
    image: phpmyadmin/phpmyadmin:latest
    container_name: phpmyadmin_app
    restart: always
    ports:
      - "127.0.0.1:8081:80"
    environment:
      PMA_HOST: db
      PMA_PORT: 3306
      PMA_USER: root
      PMA_PASSWORD: ${MYSQL_ROOT_PASSWORD}
      MYSQL_ROOT_PASSWORD: ${MYSQL_ROOT_PASSWORD}
      UPLOAD_LIMIT: 100M
    networks:
      - app-network

networks:
  app-network:
    driver: bridge

volumes:
  db_data:
  backup_data:
```

Key changes vs original:
- `nginx` service removed entirely
- `frontend` ports changed from `36018:80` → `80:80` + `443:443`
- `frontend` gains volume mount `./nginx/certs:/etc/nginx/certs`
- `ALLOWED_ORIGINS` default changed to `https://localhost` in both `api` and `prediction`

- [ ] **Step 2: Commit**

```bash
git add docker-compose.yaml
git commit -m "feat: remove nginx container, frontend nginx becomes single HTTPS gateway"
```

---

### Task 6: Update .env.example

**Files:**
- Modify: `.env.example`

- [ ] **Step 1: Update CORS section**

Replace the `# ============ CORS ============` section in `.env.example` with:
```
# ============ CORS ============
# Allowed frontend origin for CORS. Must match the public URL of the frontend.
# Self-signed / local:  https://localhost
# Tailscale:            https://<machine>.ts.net
# Domain / cloud:       https://yourdomain.com
# Cloudflare:           https://yourdomain.com
ALLOWED_ORIGINS=https://localhost
```

- [ ] **Step 2: Commit**

```bash
git add .env.example
git commit -m "docs: update ALLOWED_ORIGINS default to https:// in .env.example"
```

---

### Task 7: Create cert generation scripts

**Files:**
- Create: `scripts/gen-self-signed.sh`
- Create: `scripts/gen-tailscale-cert.sh`
- Create: `scripts/gen-letsencrypt.sh`
- Create: `scripts/gen-cloudflare-origin.sh`

- [ ] **Step 1: Create scripts/ directory**

```bash
mkdir -p scripts
```

- [ ] **Step 2: Write gen-self-signed.sh**

Create `scripts/gen-self-signed.sh`:
```bash
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
```

- [ ] **Step 3: Write gen-tailscale-cert.sh**

Create `scripts/gen-tailscale-cert.sh`:
```bash
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
echo "Then:         docker-compose restart frontend"
```

- [ ] **Step 4: Write gen-letsencrypt.sh**

Create `scripts/gen-letsencrypt.sh`:
```bash
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
docker-compose stop frontend 2>/dev/null || true

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
echo "Then:         docker-compose start frontend"
echo ""
echo "Auto-renewal crontab (run as root):"
echo "  0 3 * * * certbot renew --quiet && \\"
echo "    cp /etc/letsencrypt/live/$DOMAIN/fullchain.pem $(pwd)/nginx/certs/cert.pem && \\"
echo "    cp /etc/letsencrypt/live/$DOMAIN/privkey.pem $(pwd)/nginx/certs/key.pem && \\"
echo "    docker-compose restart frontend"
```

- [ ] **Step 5: Write gen-cloudflare-origin.sh**

Create `scripts/gen-cloudflare-origin.sh`:
```bash
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
CERT_DIR="$(cd "$(dirname "$0")/.." && pwd)/nginx/certs"
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
  echo "  7. docker-compose restart frontend"
  exit 0
fi

cp "$CERT_SRC" "$CERT_DIR/cert.pem"
cp "$KEY_SRC"  "$CERT_DIR/key.pem"
chmod 600 "$CERT_DIR/key.pem"

echo "[TLS] Cloudflare Origin Certificate installed to nginx/certs/"
echo ""
echo "Ensure Cloudflare SSL/TLS encryption mode is set to 'Full (strict)'"
echo "Then: docker-compose restart frontend"
```

- [ ] **Step 6: Make all scripts executable**

```bash
chmod +x scripts/gen-self-signed.sh \
         scripts/gen-tailscale-cert.sh \
         scripts/gen-letsencrypt.sh \
         scripts/gen-cloudflare-origin.sh
```

- [ ] **Step 7: Commit**

```bash
git add scripts/
git commit -m "feat: add TLS cert generation scripts for all deployment scenarios"
```

---

### Task 8: Build and verify

**End-to-end smoke test of the entire HTTPS setup.**

- [ ] **Step 1: Rebuild the frontend image**

```bash
docker-compose build frontend
```
Expected: Build completes without errors. Look for `COPY docker-entrypoint.sh` and `EXPOSE 80 443` in output.

- [ ] **Step 2: Bring up the stack (stop first if already running)**

```bash
docker-compose down && docker-compose up -d
```
Expected: Services start: `db`, `prediction`, `api`, `frontend`, `phpmyadmin`. No `nginx_proxy` container.

- [ ] **Step 3: Check frontend logs for TLS init message**

```bash
docker logs vnstock_frontend 2>&1 | head -5
```
Expected output (first run):
```
[TLS] No certificates found in /etc/nginx/certs — generating self-signed cert...
[TLS] Self-signed cert generated (valid 10 years). ...
```
On subsequent runs (certs already exist):
```
(no TLS lines — goes straight to nginx)
```

- [ ] **Step 4: Verify cert files exist on host**

```bash
ls -la nginx/certs/
```
Expected: `cert.pem` and `key.pem` present.

- [ ] **Step 5: Test HTTP → HTTPS redirect**

```bash
curl -v http://localhost/ 2>&1 | grep -E "< HTTP|Location:"
```
Expected:
```
< HTTP/1.1 301 Moved Permanently
< Location: https://localhost/
```

- [ ] **Step 6: Test HTTPS with self-signed cert (skip cert validation)**

```bash
curl -vk https://localhost/ 2>&1 | grep -E "< HTTP|SSL connection"
```
Expected:
```
* SSL connection using TLSv1.3 / ...
< HTTP/1.1 200 OK
```

- [ ] **Step 7: Test API proxy over HTTPS**

```bash
curl -sk https://localhost/api/dashboard/stats
```
Expected: JSON response (not nginx 502/404).

- [ ] **Step 8: Verify old port 36018 is no longer listening**

```bash
curl -s --connect-timeout 2 http://localhost:36018/ 2>&1 | head -3
```
Expected: `curl: (7) Failed to connect` (connection refused).

- [ ] **Step 9: Verify phpMyAdmin still works**

```bash
curl -s http://127.0.0.1:8081/ | grep -i phpmyadmin | head -1
```
Expected: HTML line containing "phpMyAdmin".

- [ ] **Step 10: Commit any fixes needed after verification**

```bash
git add -A
git commit -m "fix: post-verification TLS fixes"
```
Skip if no fixes were needed.
