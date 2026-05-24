#!/usr/bin/env bash
# Benchmark report-api JSON endpoints; checks p95 against 200ms SLO (default).
set -euo pipefail

REPORT_URL="${REPORT_URL:-http://localhost:8082}"
MANAGER="${MANAGER:-cccccccc-cccc-cccc-cccc-cccccccccccc}"
ORG="${ORG:-a0000000-0000-0000-0000-000000000003}"
ITERATIONS="${ITERATIONS:-30}"
SLO_MS="${SLO_MS:-200}"

TODAY=$(date +%Y-%m-%d)
MONTH=$(date +%Y-%m-01)

endpoints=(
  "department|/reports/department?orgUnitId=${ORG}&startDate=${MONTH}&endDate=${TODAY}&granularity=daily"
  "workforce_utilization|/reports/analytics/workforce-utilization?orgUnitId=${ORG}&startDate=${MONTH}&endDate=${TODAY}"
  "attendance_trends|/reports/analytics/attendance-trends?orgUnitId=${ORG}&startDate=${MONTH}&endDate=${TODAY}&granularity=monthly"
  "door_heatmap|/reports/analytics/door-heatmap?orgUnitId=${ORG}&minutes=60"
  "personal|/reports/personal?startDate=${MONTH}&endDate=${TODAY}"
)

percentile() {
  local p=$1
  shift
  local -a arr=("$@")
  local n=${#arr[@]}
  local idx=$(( (n * p) / 100 ))
  if (( idx >= n )); then idx=$(( n - 1 )); fi
  if (( idx < 0 )); then idx=0; fi
  echo "${arr[$idx]}"
}

bench_one() {
  local name=$1
  local path=$2
  local -a ms_samples=()
  local errors=0

  echo ""
  echo "--- ${name} (${ITERATIONS} requests) ---"
  for ((i=0; i<ITERATIONS; i++)); do
    local t_ms
    t_ms=$(curl -sf -o /dev/null -H "X-User-ID: ${MANAGER}" \
      -w '%{time_total}' "${REPORT_URL}${path}" 2>/dev/null | awk '{printf "%.0f", $1*1000}') || { errors=$((errors+1)); continue; }
    ms_samples+=("$t_ms")
  done

  if ((${#ms_samples[@]} == 0)); then
    echo "FAIL: all requests failed"
    return 1
  fi

  IFS=$'\n' sorted=($(sort -n <<<"${ms_samples[*]}"))
  unset IFS
  local p50 p95 p99 max
  p50=$(percentile 50 "${sorted[@]}")
  p95=$(percentile 95 "${sorted[@]}")
  p99=$(percentile 99 "${sorted[@]}")
  max=${sorted[${#sorted[@]}-1]}

  echo "  ok=${#ms_samples[@]} errors=${errors}"
  echo "  latency_ms: p50=${p50} p95=${p95} p99=${p99} max=${max}"

  if (( p95 <= SLO_MS )); then
    echo "  SLO: PASS (p95 <= ${SLO_MS}ms)"
    return 0
  else
    echo "  SLO: FAIL (p95 ${p95}ms > ${SLO_MS}ms)"
    return 1
  fi
}

echo "Report API benchmark — SLO p95 <= ${SLO_MS}ms per endpoint"
echo "URL: ${REPORT_URL}"

failed=0
for ep in "${endpoints[@]}"; do
  IFS='|' read -r name path <<< "$ep"
  bench_one "$name" "$path" || failed=$((failed+1))
done

echo ""
if (( failed == 0 )); then
  echo "All endpoints within p95 SLO."
  exit 0
else
  echo "${failed} endpoint(s) exceeded p95 SLO."
  exit 1
fi
