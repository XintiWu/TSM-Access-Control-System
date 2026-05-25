#!/usr/bin/env bash
# Generate 50+ ANTI_PASSBACK denials in ~1 minute for Grafana/Prometheus alert demo.
set -euo pipefail

# Load environment variables if present
ENV_FILE="$(dirname "$0")/../.env"
if [ -f "$ENV_FILE" ]; then
  source "$ENV_FILE"
fi

# Derive REDIS_HOST and REDIS_PORT from REDIS_ADDR if set
if [ -n "${REDIS_ADDR:-}" ]; then
  REDIS_HOST="${REDIS_ADDR%:*}"
  REDIS_PORT="${REDIS_ADDR#*:}"
fi


API="${API:-http://localhost:8080}"
USER="${USER:-22222222-2222-2222-2222-222222222222}"
DOOR="${DOOR:-11111111-1111-1111-1111-111111111111}"

redis_cmd() {
  docker compose exec -T redis redis-cli "$@" 2>/dev/null
}

echo ">>> Clearing passback state for ${USER}"
redis_cmd DEL "passback:${USER}" >/dev/null || true

echo ">>> First IN (ALLOW) to establish passback=IN"
curl -sf -X POST "${API}/access/swipe" \
  -H "Content-Type: application/json" \
  -d "{\"userId\":\"${USER}\",\"doorId\":\"${DOOR}\",\"direction\":\"IN\",\"cardUid\":\"CARD001\",\"timestamp\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}" | jq -r '.decision'

echo ">>> Sending 55 consecutive IN swipes (expect DENY / ANTI_PASSBACK)..."
for i in $(seq 1 55); do
  curl -sf -X POST "${API}/access/swipe" \
    -H "Content-Type: application/json" \
    -d "{\"userId\":\"${USER}\",\"doorId\":\"${DOOR}\",\"direction\":\"IN\",\"cardUid\":\"CARD001\",\"timestamp\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}" >/dev/null || true
done

echo ">>> Wait for report-api metrics poll (up to 20s)..."
sleep 20
echo ">>> Prometheus metric (host):"
curl -sf http://localhost:8082/metrics 2>/dev/null | grep report_passback_deny_1m || true
echo "Done. Check Grafana → Access Analytics → Anti-passback stat / alerting."
