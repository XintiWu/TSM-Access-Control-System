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


API="${API_URL:-http://localhost:8080}"
USER="${DEMO_USER:-22222222-2222-2222-2222-222222222222}"
DOOR="${DEMO_DOOR:-11111111-1111-1111-1111-111111111111}"
POLL_INTERVAL="${POLL_INTERVAL:-2}"
POLL_TIMEOUT="${POLL_TIMEOUT:-30}"

require_service() {
  local name="$1"
  if ! docker compose ps --status running --services 2>/dev/null | grep -qx "$name"; then
    echo "ERROR: service '$name' is not running. Run 'make up' first." >&2
    exit 1
  fi
}

echo "=== Kafka → DB pipeline verification ==="
echo

for svc in access-api kafka aggregation-worker redis clickhouse; do
  require_service "$svc"
done
echo "Docker services OK"
echo

redis_cmd() {
  if command -v redis-cli >/dev/null 2>&1; then
    redis-cli -h "${REDIS_HOST:-localhost}" -p "${REDIS_PORT:-6379}" "$@"
  else
    docker compose exec -T redis redis-cli "$@"
  fi
}

redis_cmd DEL "passback:${USER}" >/dev/null
echo "Cleared passback:${USER}"
echo

echo ">>> POST /access/swipe (IN)"
RESP=$(curl -sf -X POST "${API}/access/swipe" \
  -H "Content-Type: application/json" \
  -d "{\"userId\":\"${USER}\",\"doorId\":\"${DOOR}\",\"direction\":\"IN\",\"cardUid\":\"CARD001\",\"timestamp\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}")
echo "$RESP" | jq .
echo

EVENT_ID=$(echo "$RESP" | jq -r '.eventId')
DECISION=$(echo "$RESP" | jq -r '.decision')
if [[ -z "$EVENT_ID" || "$EVENT_ID" == "null" ]]; then
  echo "ERROR: swipe response missing eventId" >&2
  exit 1
fi

ch_query() {
  docker compose exec -T clickhouse clickhouse-client --password ${CLICKHOUSE_PASSWORD:-password123} --database=access_control --query="$1" 2>/dev/null
}

echo "Polling ClickHouse for eventId=${EVENT_ID} (timeout ${POLL_TIMEOUT}s)..."
elapsed=0
ROW=""
while [[ "$elapsed" -lt "$POLL_TIMEOUT" ]]; do
  ROW=$(ch_query "SELECT id, employee_id, direction, status, ifNull(reason,'') FROM inout_events WHERE id='${EVENT_ID}'" || true)
  ROW=$(echo "$ROW" | tr -d '\r')
  if [[ -n "$ROW" ]]; then
    break
  fi
  sleep "$POLL_INTERVAL"
  elapsed=$((elapsed + POLL_INTERVAL))
done

if [[ -z "$ROW" ]]; then
  echo "ERROR: event not found in ClickHouse inout_events after ${POLL_TIMEOUT}s" >&2
  echo "Hint: check worker logs with 'make logs'" >&2
  exit 1
fi

IFS=$'\t' read -r db_id db_employee db_direction db_status db_reason <<< "$ROW"

fail=0
if [[ "$db_id" != "$EVENT_ID" ]]; then
  echo "FAIL: id mismatch (got ${db_id})" >&2
  fail=1
fi
if [[ "$db_employee" != "$USER" ]]; then
  echo "FAIL: employee_id mismatch (got ${db_employee}, want ${USER})" >&2
  fail=1
fi
if [[ "$db_direction" != "IN" ]]; then
  echo "FAIL: direction mismatch (got ${db_direction}, want IN)" >&2
  fail=1
fi
if [[ "$db_status" != "$DECISION" ]]; then
  echo "FAIL: status mismatch (got ${db_status}, want ${DECISION})" >&2
  fail=1
fi

if [[ "$fail" -ne 0 ]]; then
  exit 1
fi

echo "PASS: event persisted to ClickHouse"
echo "  id=${db_id} employee_id=${db_employee} direction=${db_direction} status=${db_status} reason=${db_reason:-<none>}"
echo
echo "=== Pipeline verification complete ==="
