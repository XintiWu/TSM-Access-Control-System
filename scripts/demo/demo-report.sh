#!/usr/bin/env bash
# demo-report.sh — Report API + analytics demo (roles, late rate, source_ip)
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


REPORT_URL=${REPORT_URL:-http://localhost:8082}
ACCESS_URL=${ACCESS_URL:-http://localhost:8080}
EMPLOYEE_ID="22222222-2222-2222-2222-222222222222"
MANAGER_ID="cccccccc-cccc-cccc-cccc-cccccccccccc"
CEO_ID="aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
DOOR_ID="11111111-1111-1111-1111-111111111111"
TEAM_ORG="a0000000-0000-0000-0000-000000000003"
CORP_ORG="a0000000-0000-0000-0000-000000000001"
TODAY=$(date +%Y-%m-%d)
MONTH_START=$(date +%Y-%m-01)

echo "============================================"
echo "  Report API Demo"
echo "============================================"

echo "=== Step 0: Sample swipes ==="
docker compose exec -T redis redis-cli DEL "passback:${EMPLOYEE_ID}" 2>/dev/null || true
curl -sf -X POST "${ACCESS_URL}/access/swipe" \
  -H "Content-Type: application/json" \
  -d "{\"userId\":\"${EMPLOYEE_ID}\",\"doorId\":\"${DOOR_ID}\",\"direction\":\"IN\"}" | jq -r '.decision' || true
sleep 1
curl -sf -X POST "${ACCESS_URL}/access/swipe" \
  -H "Content-Type: application/json" \
  -d "{\"userId\":\"${EMPLOYEE_ID}\",\"doorId\":\"${DOOR_ID}\",\"direction\":\"OUT\"}" | jq -r '.decision' || true
sleep 5

echo "=== Step 1: Health ==="
curl -sf "${REPORT_URL}/health" | jq .

echo "=== Step 2: Personal (employee) ==="
curl -sf -H "X-User-ID: ${EMPLOYEE_ID}" \
  "${REPORT_URL}/reports/personal?startDate=${MONTH_START}&endDate=${TODAY}" | jq .

echo "=== Step 3: Department (team manager, daily) ==="
curl -sf -H "X-User-ID: ${MANAGER_ID}" \
  "${REPORT_URL}/reports/department?orgUnitId=${TEAM_ORG}&startDate=${MONTH_START}&endDate=${TODAY}&granularity=monthly" | jq '.summary, .periods[0]'

echo "=== Step 4: Employee denied department report (403) ==="
CODE=$(curl -s -o /dev/null -w "%{http_code}" -H "X-User-ID: ${EMPLOYEE_ID}" \
  "${REPORT_URL}/reports/department?orgUnitId=${TEAM_ORG}&startDate=${MONTH_START}&endDate=${TODAY}" || echo "000")
echo "  HTTP ${CODE} (expect 403)"

echo "=== Step 5: Audit log (includes sourceIp) ==="
curl -sf -H "X-User-ID: ${MANAGER_ID}" \
  "${REPORT_URL}/reports/audit?startDate=${MONTH_START}&endDate=${TODAY}&page=1&pageSize=3" | jq '.events[0]'

echo "=== Step 6: Door heatmap (CEO) ==="
curl -sf -H "X-User-ID: ${CEO_ID}" \
  "${REPORT_URL}/reports/analytics/door-heatmap?minutes=60" | jq .

echo "=== Step 7: Attendance trends (CEO, corp org) ==="
curl -sf -H "X-User-ID: ${CEO_ID}" \
  "${REPORT_URL}/reports/analytics/attendance-trends?orgUnitId=${CORP_ORG}&startDate=${MONTH_START}&endDate=${TODAY}&granularity=daily" | jq '.series | length'

echo "=== Step 8: CSV export (events, source IP column) ==="
curl -sf -H "X-User-ID: ${MANAGER_ID}" \
  "${REPORT_URL}/reports/export?orgUnitId=${TEAM_ORG}&startDate=${MONTH_START}&endDate=${TODAY}&format=csv&type=events" \
  -o ./report_demo.csv
head -2 ./report_demo.csv

echo "=== Step 9: PDF department export ==="
curl -sf -H "X-User-ID: ${MANAGER_ID}" \
  "${REPORT_URL}/reports/export?orgUnitId=${TEAM_ORG}&startDate=${MONTH_START}&endDate=${TODAY}&format=pdf&type=department" \
  -o ./report_demo.pdf
file ./report_demo.pdf 2>/dev/null || true

echo "============================================"
echo "  Report API Demo Complete"
echo "============================================"
