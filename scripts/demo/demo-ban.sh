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


API_KEY="${API_KEY:-dev-api-key-2026}"
ACCESS_API="${API_URL:-http://localhost:8080}"
ADMIN_API="${ADMIN_URL:-http://localhost:8081}"
USER_ID="${DEMO_USER:-22222222-2222-2222-2222-222222222222}"
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
  redis_cmd DEL "passback:${USER_ID}" >/dev/null || true
}

swipe_json() {
  local dir="${1:-IN}"
  curl -sf -H "X-API-Key: ${API_KEY}" -X POST "${ACCESS_API}/access/swipe" \
    -H "Content-Type: application/json" \
    -d "{\"userId\":\"${USER_ID}\",\"doorId\":\"${DOOR}\",\"direction\":\"${dir}\",\"cardUid\":\"CARD001\",\"timestamp\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}"
}

expect_swipe() {
  local want_decision="$1"
  local want_reason="${2:-}"
  local dir="${3:-IN}"
  clear_passback
  local resp decision reason
  resp=$(swipe_json "$dir")
  decision=$(echo "$resp" | jq -r '.decision')
  reason=$(echo "$resp" | jq -r '.reason // empty')

  # Self-correct if user was left in IN state on remote GKE Redis from previous runs
  if [[ "$dir" == "IN" && "$decision" == "DENY" && "$reason" == "ANTI_PASSBACK" && "$want_decision" == "ALLOW" ]]; then
    echo "User is already IN (Redis state). Swiping OUT first to reset..."
    swipe_json "OUT" >/dev/null
    resp=$(swipe_json "IN")
    decision=$(echo "$resp" | jq -r '.decision')
    reason=$(echo "$resp" | jq -r '.reason // empty')
  fi

  echo "$resp" | jq .
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
expect_swipe "ALLOW" "" "IN"

echo ">>> POST ${ADMIN_API}/admin/employees/${USER_ID}/ban"
curl -sf -H "X-API-Key: ${API_KEY}" -X POST "${ADMIN_API}/admin/employees/${USER_ID}/ban" | jq .
echo "Waiting ${WAIT}s for cache invalidation worker..."
sleep "$WAIT"
echo

echo ">>> Swipe after ban (expect DENY / PERMISSION_DENIED)"
expect_swipe "DENY" "PERMISSION_DENIED" "IN"

echo ">>> POST ${ADMIN_API}/admin/employees/${USER_ID}/unban"
curl -sf -H "X-API-Key: ${API_KEY}" -X POST "${ADMIN_API}/admin/employees/${USER_ID}/unban" | jq .
echo "Waiting ${WAIT}s for cache invalidation worker..."
sleep "$WAIT"
echo

echo ">>> Swipe after unban (expect ALLOW)"
# Swipe OUT to bypass anti-passback if Redis cannot be cleared from the local machine
expect_swipe "ALLOW" "" "OUT"

echo "=== Admin ban pipeline demo complete ==="
