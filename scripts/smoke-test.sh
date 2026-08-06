#!/usr/bin/env bash
# =============================================================================
# smoke-test.sh — E2E cơ bản qua gateway/ingress (login → version → monitoring).
#
#   ./scripts/smoke-test.sh                        # BASE_URL mặc định http://localhost
#   BASE_URL=http://stock.local ./scripts/smoke-test.sh
#   USER=chon PASS='Ch1nch2n@' ./scripts/smoke-test.sh
#
# Cho k8s: port-forward gateway trước, vd:
#   kubectl -n stock port-forward svc/gateway-svc 8080:80 &  BASE_URL=http://localhost:8080
# Exit != 0 nếu bất kỳ check FAIL.
# =============================================================================
set -uo pipefail

BASE_URL="${BASE_URL:-http://localhost}"
USER="${USER:-chon}"
PASS="${PASS:?set PASS=<super_admin password> for smoke-test}"
FAIL=0
pass() { echo "  PASS  $1"; }
fail() { echo "  FAIL  $1"; FAIL=1; }

echo ">> target: ${BASE_URL}  (user ${USER})"

# 1) /api/version (public, no auth)
ver="$(curl -fsS "${BASE_URL}/api/version" 2>/dev/null)" \
  && pass "GET /api/version" || fail "GET /api/version"

# 2) login → JWT
TOKEN="$(curl -fsS -X POST "${BASE_URL}/api/x/grant" \
  -H 'Content-Type: application/json' \
  -H "X-Token: $(printf '%s' "${USER}:${PASS}" | base64)" \
  -d '{"request":""}' 2>/dev/null | grep -o '"token":"[^"]*"' | cut -d'"' -f4)"
if [ -n "${TOKEN}" ]; then pass "POST /api/x/grant (JWT acquired)"; else fail "POST /api/x/grant (no token)"; fi

# 3) authenticated read
if [ -n "${TOKEN}" ]; then
  code="$(curl -fsS -o /dev/null -w '%{http_code}' \
    -H "Authorization: Bearer ${TOKEN}" "${BASE_URL}/api/monitoring/overview" 2>/dev/null)"
  [ "$code" = "200" ] && pass "GET /api/monitoring/overview (200)" || fail "GET /api/monitoring/overview (got ${code:-none})"

  code="$(curl -fsS -o /dev/null -w '%{http_code}' \
    -H "Authorization: Bearer ${TOKEN}" "${BASE_URL}/api/gold/latest" 2>/dev/null)"
  [ "$code" = "200" ] && pass "GET /api/gold/latest (200)" || fail "GET /api/gold/latest (got ${code:-none})"
fi

# 4) frontend reachable
code="$(curl -fsS -o /dev/null -w '%{http_code}' "${BASE_URL}/" 2>/dev/null)"
[ "$code" = "200" ] && pass "GET / (frontend 200)" || fail "GET / (got ${code:-none})"

echo ">> $( [ $FAIL -eq 0 ] && echo 'ALL PASS' || echo 'SOME FAILED' )"
exit $FAIL
