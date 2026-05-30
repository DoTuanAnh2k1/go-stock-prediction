#!/bin/bash
set -euo pipefail

# Setup test database for integration tests
# Usage: ./scripts/setup-test-db.sh

MYSQL_HOST="${TEST_MYSQL_HOST:-localhost}"
MYSQL_PORT="${TEST_MYSQL_PORT:-3307}"
MYSQL_USER="${TEST_MYSQL_USER:-root}"
MYSQL_PASSWORD="${TEST_MYSQL_PASSWORD:-test123}"
MYSQL_DB="${TEST_MYSQL_DB:-go_stock_prediction_test}"

echo "==> Waiting for test MySQL to be ready..."
for i in $(seq 1 30); do
    if mysqladmin ping -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" -p"$MYSQL_PASSWORD" --silent 2>/dev/null; then
        echo "==> MySQL is ready!"
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo "==> ERROR: MySQL did not become ready in time"
        exit 1
    fi
    sleep 1
done

echo "==> Creating database if not exists..."
mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" -p"$MYSQL_PASSWORD" \
    -e "CREATE DATABASE IF NOT EXISTS \`$MYSQL_DB\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"

echo "==> Test database setup complete: $MYSQL_DB on $MYSQL_HOST:$MYSQL_PORT"
