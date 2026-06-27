#!/bin/sh
# Writes /versions/web-svc.json so api-svc (or any aggregator) can read it
# from a shared volume.  The file contains the same fields as the static
# /version.json served by nginx plus a started_at timestamp.
#
# Runs before nginx starts because the stock nginx image executes every
# /docker-entrypoint.d/*.sh script in alphabetical order.  Failures here
# must not prevent nginx from starting, so we use "|| true" throughout.

set -e

VERSIONS_DIR="/versions"
OUT="$VERSIONS_DIR/web-svc.json"
STARTED_AT="$(date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || echo "unknown")"

mkdir -p "$VERSIONS_DIR" || true

printf '{"service":"web-svc","git_sha":"%s","build_time":"%s","dirty":"%s","started_at":"%s"}\n' \
    "${GIT_SHA:-unknown}" \
    "${BUILD_TIME:-unknown}" \
    "${GIT_DIRTY:-unknown}" \
    "$STARTED_AT" \
    > "$OUT" || true

echo "[web-svc] version stamp written to $OUT"
