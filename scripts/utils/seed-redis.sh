#!/usr/bin/env bash
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


# Test UUIDs for demo
DOOR_ID="11111111-1111-1111-1111-111111111111"
NORMAL_USER="22222222-2222-2222-2222-222222222222"
BANNED_USER="00000000-0000-0000-0000-000000000099"

redis_cmd() {
  if command -v redis-cli >/dev/null 2>&1; then
    redis-cli -h "${REDIS_HOST:-localhost}" -p "${REDIS_PORT:-6379}" "$@"
  else
    docker compose exec -T redis redis-cli "$@"
  fi
}

# Door heartbeat
redis_cmd SET "door:status:${DOOR_ID}" "ONLINE" EX 30

# Clear passback state for demo user
redis_cmd DEL "passback:${NORMAL_USER}"

# Card → userId mappings (mirrors employee.card_uid in ClickHouse)
redis_cmd SET "card:CARD001" "${NORMAL_USER}" EX 86400
redis_cmd SET "card:CARD099" "${BANNED_USER}" EX 86400

echo "Seeded Redis:"
echo "  door:status:${DOOR_ID} = ONLINE"
echo "  passback:${NORMAL_USER} cleared"
echo "  card:CARD001 → ${NORMAL_USER}"
echo "  card:CARD099 → ${BANNED_USER}"
echo "  (use Admin API / make ban for perm:denied — not seeded here)"
