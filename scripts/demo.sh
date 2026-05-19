#!/usr/bin/env bash
set -euo pipefail

API="${API_URL:-http://localhost:8080}"
USER="${DEMO_USER:-22222222-2222-2222-2222-222222222222}"
DOOR="${DEMO_DOOR:-11111111-1111-1111-1111-111111111111}"
BANNED="${BANNED_USER:-00000000-0000-0000-0000-000000000099}"

swipe() {
  local user="$1" dir="$2"
  echo ">>> Swipe ${dir} (user=${user})"
  curl -s -X POST "${API}/access/swipe" \
    -H "Content-Type: application/json" \
    -d "{\"userId\":\"${user}\",\"doorId\":\"${DOOR}\",\"direction\":\"${dir}\",\"cardUid\":\"CARD001\",\"timestamp\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}" | jq .
  echo
}

echo "=== Access Fast Path Demo ==="
echo

swipe "$USER" "IN"
echo "Expected: ALLOW"
echo

swipe "$USER" "IN"
echo "Expected: DENY (ANTI_PASSBACK)"
echo

swipe "$USER" "OUT"
echo "Expected: ALLOW"
echo

swipe "$BANNED" "IN"
echo "Expected: DENY (PERMISSION_DENIED)"
echo

echo ">>> Employee state"
curl -s "${API}/access/employee/${USER}/state" | jq .
echo

echo ">>> Door status"
curl -s "${API}/access/door/${DOOR}/status" | jq .
