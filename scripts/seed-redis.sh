#!/usr/bin/env bash
set -euo pipefail

# Test UUIDs for demo
BANNED_USER="00000000-0000-0000-0000-000000000099"
DOOR_ID="11111111-1111-1111-1111-111111111111"
NORMAL_USER="22222222-2222-2222-2222-222222222222"

redis_cmd() {
  if command -v redis-cli >/dev/null 2>&1; then
    redis-cli -h "${REDIS_HOST:-localhost}" -p "${REDIS_PORT:-6379}" "$@"
  else
    docker compose exec -T redis redis-cli "$@"
  fi
}

redis_cmd SET "perm:denied:${BANNED_USER}" "DENY" EX 86400
redis_cmd SET "door:status:${DOOR_ID}" "ONLINE" EX 30
redis_cmd DEL "passback:${NORMAL_USER}"

echo "Seeded Redis:"
echo "  perm:denied:${BANNED_USER} = DENY"
echo "  door:status:${DOOR_ID} = ONLINE"
echo "  passback:${NORMAL_USER} cleared"
