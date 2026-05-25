#!/usr/bin/env bash
set -euo pipefail

# Load environment variables if present
ENV_FILE="$(dirname "$0")/../../.env"
if [ -f "$ENV_FILE" ]; then
  source "$ENV_FILE"
fi

# Derive REDIS_HOST and REDIS_PORT from REDIS_ADDR if set
if [ -n "${REDIS_ADDR:-}" ]; then
  REDIS_HOST="${REDIS_ADDR%:*}"
  REDIS_PORT="${REDIS_ADDR#*:}"
fi


ACCESS_API="${API_URL:-http://localhost:8080}"
ADMIN_API="${ADMIN_URL:-http://localhost:8081}"
USER="${DEMO_USER:-22222222-2222-2222-2222-222222222222}"
DOOR="${DEMO_DOOR:-11111111-1111-1111-1111-111111111111}"
WAIT="${BAN_WAIT:-3}"

redis_cmd() {
  if command -v redis-cli >/dev/null 2>&1; then
    redis-cli -h "${REDIS_HOST:-localhost}" -p "${REDIS_PORT:-6379}" "$@"
  else
    docker compose exec -T redis redis-cli "$@"
  fi
}

clear_passback() {
  redis_cmd DEL "passback:${USER}" >/dev/null || true
}

swipe_json() {
  curl -sf -X POST "${ACCESS_API}/access/swipe" \
    -H "Content-Type: application/json" \
    -d "{\"userId\":\"${USER}\",\"doorId\":\"${DOOR}\",\"direction\":\"IN\",\"cardUid\":\"CARD001\",\"timestamp\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}"
}

expect_swipe() {
  local want_decision="$1"
  local want_reason="${2:-}"
  clear_passback
  local resp decision reason
  resp=$(swipe_json)
  echo "$resp" | jq .
  decision=$(echo "$resp" | jq -r '.decision')
  reason=$(echo "$resp" | jq -r '.reason // empty')
  if [[ "$decision" != "$want_decision" ]]; then
    echo "FAIL: expected decision=${want_decision}, got=${decision}" >&2
    exit 1
  fi
  if [[ -n "$want_reason" && "$reason" != "$want_reason" ]]; then
    echo "FAIL: expected reason=${want_reason}, got=${reason}" >&2
    exit 1
  fi
  echo "PASS: decision=${decision} reason=${reason:-<none>}"
  echo
}

echo "=== Admin ban pipeline demo ==="
echo

echo ">>> Swipe before ban (expect ALLOW)"
expect_swipe "ALLOW"

echo ">>> POST ${ADMIN_API}/admin/employees/${USER}/ban"
curl -sf -X POST "${ADMIN_API}/admin/employees/${USER}/ban" | jq .
echo "Waiting ${WAIT}s for cache invalidation worker..."
sleep "$WAIT"
echo

echo ">>> Swipe after ban (expect DENY / PERMISSION_DENIED)"
expect_swipe "DENY" "PERMISSION_DENIED"

echo ">>> POST ${ADMIN_API}/admin/employees/${USER}/unban"
curl -sf -X POST "${ADMIN_API}/admin/employees/${USER}/unban" | jq .
echo "Waiting ${WAIT}s for cache invalidation worker..."
sleep "$WAIT"
echo

echo ">>> Swipe after unban (expect ALLOW)"
expect_swipe "ALLOW"

echo "=== Admin ban pipeline demo complete ==="
