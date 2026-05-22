#!/usr/bin/env bash
# demo-report.sh — 演示 Report API 四個端點
set -euo pipefail

REPORT_URL=${REPORT_URL:-http://localhost:8082}
ACCESS_URL=${ACCESS_URL:-http://localhost:8080}
USER_ID="22222222-2222-2222-2222-222222222222"
DOOR_ID="11111111-1111-1111-1111-111111111111"
ORG_UNIT_ID="a0000000-0000-0000-0000-000000000003"
TODAY=$(date +%Y-%m-%d)
MONTH_START=$(date +%Y-%m-01)

echo "============================================"
echo "  Report API Demo"
echo "============================================"
echo ""

# Step 0: Generate some swipe data so reports have content
echo "=== Step 0: Generating swipe events ==="
# Clear passback state first
redis-cli -h localhost DEL "passback:${USER_ID}" 2>/dev/null || true

echo "  Swipe IN..."
curl -sf -X POST "${ACCESS_URL}/access/swipe" \
  -H "Content-Type: application/json" \
  -d "{\"userId\":\"${USER_ID}\",\"doorId\":\"${DOOR_ID}\",\"direction\":\"IN\"}" | jq -r '.decision' || echo "(access-api not reachable, continuing with existing data)"

sleep 1
echo "  Swipe OUT..."
curl -sf -X POST "${ACCESS_URL}/access/swipe" \
  -H "Content-Type: application/json" \
  -d "{\"userId\":\"${USER_ID}\",\"doorId\":\"${DOOR_ID}\",\"direction\":\"OUT\"}" | jq -r '.decision' || echo "(access-api not reachable)"

echo ""
echo "  Waiting 5s for aggregation worker to process events..."
sleep 5

# Step 1: Health check
echo ""
echo "=== Step 1: Health Check ==="
curl -sf "${REPORT_URL}/health" | jq .
echo ""

# Step 2: Personal Report
echo "=== Step 2: Personal Report ==="
echo "  GET /reports/personal?startDate=${MONTH_START}&endDate=${TODAY}"
curl -sf -H "X-User-ID: ${USER_ID}" \
  "${REPORT_URL}/reports/personal?startDate=${MONTH_START}&endDate=${TODAY}" | jq .
echo ""

# Step 3: Department Report
echo "=== Step 3: Department Report ==="
echo "  GET /reports/department?orgUnitId=${ORG_UNIT_ID}&startDate=${MONTH_START}&endDate=${TODAY}&granularity=daily"
curl -sf -H "X-User-ID: ${USER_ID}" \
  "${REPORT_URL}/reports/department?orgUnitId=${ORG_UNIT_ID}&startDate=${MONTH_START}&endDate=${TODAY}&granularity=daily" | jq .
echo ""

# Step 4: Audit Log
echo "=== Step 4: Audit Log ==="
echo "  GET /reports/audit?startDate=${MONTH_START}&endDate=${TODAY}&page=1&pageSize=10"
curl -sf -H "X-User-ID: ${USER_ID}" \
  "${REPORT_URL}/reports/audit?startDate=${MONTH_START}&endDate=${TODAY}&page=1&pageSize=10" | jq .
echo ""

# Step 5: CSV Export
echo "=== Step 5: CSV Export ==="
echo "  GET /reports/export?orgUnitId=${ORG_UNIT_ID}&startDate=${MONTH_START}&endDate=${TODAY}&format=csv"
curl -sf -H "X-User-ID: ${USER_ID}" \
  "${REPORT_URL}/reports/export?orgUnitId=${ORG_UNIT_ID}&startDate=${MONTH_START}&endDate=${TODAY}&format=csv" -o ./report_demo.csv
echo "  Saved to ./report_demo.csv"
head -5 ./report_demo.csv 2>/dev/null || echo "  (file empty or not created)"
echo ""

echo "============================================"
echo "  Report API Demo Complete ✅"
echo "============================================"

# Step 6: PDF Export (sync)
echo "=== Step 6: PDF Export (sync) ==="
curl -sf -H "X-User-ID: ${USER_ID}"   "${REPORT_URL}/reports/export?orgUnitId=${ORG_UNIT_ID}&startDate=${MONTH_START}&endDate=${TODAY}&format=pdf&type=department"   -o ./report_demo.pdf
echo "  Saved to ./report_demo.pdf"
file ./report_demo.pdf 2>/dev/null || true
echo ""

# Step 7: Async PDF job
echo "=== Step 7: Async PDF Export ==="
JOB=$(curl -sf -X POST -H "X-User-ID: ${USER_ID}" -H "Content-Type: application/json"   "${REPORT_URL}/reports/export/jobs"   -d "{"type":"department","format":"pdf","orgUnitId":"${ORG_UNIT_ID}","startDate":"${MONTH_START}","endDate":"${TODAY}","granularity":"daily"}" | jq -r .jobId)
echo "  jobId=${JOB}"
for i in 1 2 3 4 5 6 7 8 9 10; do
  STATUS=$(curl -sf -o /tmp/job_out -w "%{http_code}" -H "X-User-ID: ${USER_ID}"     "${REPORT_URL}/reports/export/jobs/${JOB}")
  if [ "$STATUS" = "200" ]; then
    mv /tmp/job_out ./report_async.pdf
    echo "  Saved to ./report_async.pdf"
    break
  fi
  sleep 1
done
echo ""
