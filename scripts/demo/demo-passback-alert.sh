#!/usr/bin/env bash
# Generate 50+ ANTI_PASSBACK denials in ~1 minute for Grafana/Prometheus alert demo.
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
API="${API_URL:-${API:-http://localhost:8080}}"
USER_ID="${USER_ID:-22222222-2222-2222-2222-222222222222}"
DOOR="${DOOR:-11111111-1111-1111-1111-111111111111}"

redis_cmd() {
  if command -v redis-cli >/dev/null 2>&1; then
    redis-cli -h "${REDIS_HOST:-localhost}" -p "${REDIS_PORT:-6379}" "$@" 2>/dev/null
  else
    docker compose exec -T redis redis-cli "$@" 2>/dev/null
  fi
}

echo ">>> Clearing passback state for ${USER_ID}"
redis_cmd DEL "passback:${USER_ID}" >/dev/null || true

echo ">>> First IN (ALLOW) to establish passback=IN"
curl -sf -H "X-API-Key: ${API_KEY}" -X POST "${API}/access/swipe" \
  -H "Content-Type: application/json" \
  -d "{\"userId\":\"${USER_ID}\",\"doorId\":\"${DOOR}\",\"direction\":\"IN\",\"cardUid\":\"CARD001\",\"timestamp\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}" | jq -r '.decision'

echo ">>> Sending 55 consecutive IN swipes (expect DENY / ANTI_PASSBACK)..."
for i in $(seq 1 55); do
  curl -sf -H "X-API-Key: ${API_KEY}" -X POST "${API}/access/swipe" \
    -H "Content-Type: application/json" \
    -d "{\"userId\":\"${USER_ID}\",\"doorId\":\"${DOOR}\",\"direction\":\"IN\",\"cardUid\":\"CARD001\",\"timestamp\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}" >/dev/null || true
done

echo ">>> Wait for report-api metrics poll (up to 20s)..."
sleep 20
echo ">>> Prometheus metric (host):"
curl -sf -H "X-API-Key: ${API_KEY}" "${REPORT_URL:-http://localhost:8082}/metrics" 2>/dev/null | grep report_passback_deny_1m || true
echo "Done. Check Grafana → Access Analytics → Anti-passback stat / alerting."
