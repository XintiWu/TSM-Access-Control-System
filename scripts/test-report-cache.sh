#!/usr/bin/env bash
# Verifies report-api Redis cache: repeat hits are fast; new swipes may not appear until cache expires or is cleared.
set -euo pipefail

REPORT_URL="${REPORT_URL:-http://localhost:8082}"
ACCESS_URL="${ACCESS_URL:-http://localhost:8080}"
REDIS="${REDIS:-docker compose exec -T redis redis-cli}"
MANAGER="${MANAGER:-cccccccc-cccc-cccc-cccc-cccccccccccc}"
ORG="${ORG:-a0000000-0000-0000-0000-000000000003}"
USER_SWIPE="${USER_SWIPE:-22222222-2222-2222-2222-222222222222}"
DOOR="${DOOR:-11111111-1111-1111-1111-111111111111}"
TODAY=$(date +%Y-%m-%d)
MONTH=$(date +%Y-%m-01)
QUERY="orgUnitId=${ORG}&startDate=${MONTH}&endDate=${TODAY}&granularity=daily"
CACHE_KEY="report:dept:${ORG}:${MONTH}:${TODAY}:daily"

hdr() { echo ""; echo "=== $1 ==="; }

fetch_entries() {
  curl -sf -H "X-User-ID: ${MANAGER}" \
    "${REPORT_URL}/reports/department?${QUERY}" | jq -r '.summary.totalEntries'
}

fetch_ms() {
  curl -sf -o /dev/null -H "X-User-ID: ${MANAGER}" \
    -w '%{time_total}' "${REPORT_URL}/reports/department?${QUERY}"
}

hdr "1) Baseline department report"
E1=$(fetch_entries)
MS1=$(fetch_ms)
echo "totalEntries=${E1}  latency=${MS1}s (likely cache miss or cold CH)"

hdr "2) Immediate repeat (expect Redis cache hit — same totalEntries, often faster)"
E2=$(fetch_entries)
MS2=$(fetch_ms)
echo "totalEntries=${E2}  latency=${MS2}s"
if [[ "$E1" != "$E2" ]]; then
  echo "WARN: entries changed between run 1 and 2 without new swipe"
fi

hdr "3) New swipe then immediate report (cache may still show old totals)"
curl -sf -X POST "${ACCESS_URL}/access/swipe" -H "Content-Type: application/json" -d "{
  \"userId\": \"${USER_SWIPE}\",
  \"doorId\": \"${DOOR}\",
  \"direction\": \"IN\",
  \"timestamp\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"
}" | jq -r '.decision,.eventId' || { echo "access-api swipe failed"; exit 1; }
sleep 2
E3=$(fetch_entries)
MS3=$(fetch_ms)
echo "totalEntries=${E3}  latency=${MS3}s"
if [[ "$E3" == "$E2" ]]; then
  echo "RESULT: Cached report unchanged after swipe (stale up to 5 min TTL)."
else
  echo "RESULT: Report updated immediately (cache miss or invalidation active)."
fi

hdr "4) Clear report cache key in Redis, fetch again (should reflect ClickHouse)"
$REDIS DEL "${CACHE_KEY}" >/dev/null 2>&1 || true
sleep 1
E4=$(fetch_entries)
MS4=$(fetch_ms)
echo "totalEntries=${E4}  latency=${MS4}s (after DEL ${CACHE_KEY})"
if [[ "$E4" != "$E3" ]]; then
  echo "PASS: After cache invalidation, totals changed (${E3} -> ${E4})."
else
  echo "NOTE: Totals still equal — ingestion lag or same-day aggregate not yet in MV; wait and re-run."
fi

hdr "Summary"
echo "Cache key pattern: report:dept:* TTL=5 minutes (see report-api/internal/cache/redis.go)"
echo "Compare latencies: cold=${MS1}s cached=${MS2}s after-swipe=${MS3}s after-DEL=${MS4}s"
