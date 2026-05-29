#!/usr/bin/env bash
# 完整演示：大量人流刷卡 → Kafka → ClickHouse → 報表 / 匯出 / Grafana 指標
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
REPORT_URL="${REPORT_URL:-http://localhost:8082}"
TODAY=$(date +%Y-%m-%d)
MONTH_START=$(date +%Y-%m-01)

# Demo identities (see clickhouse/seed.sql)
USERS=(
  "22222222-2222-2222-2222-222222222222:CARD001"
  "cccccccc-cccc-cccc-cccc-cccccccccccc:CARDMGR"
  "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb:CARDVP"
  "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa:CARDCEO"
)
DOORS=(
  "11111111-1111-1111-1111-111111111111"
  "22222222-2222-2222-2222-222222222221"
  "33333333-3333-3333-3333-333333333331"
)
MANAGER_ID="cccccccc-cccc-cccc-cccc-cccccccccccc"
CEO_ID="aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
TEAM_ORG="a0000000-0000-0000-0000-000000000003"
CORP_ORG="a0000000-0000-0000-0000-000000000001"

PAIRS_PER_USER="${PAIRS_PER_USER:-40}"   # 每人 IN+OUT 各 40 次 ≈ 320 ALLOW 事件
INTERVAL="${INTERVAL:-15ms}"

redis_del_passback() {
  if command -v redis-cli >/dev/null 2>&1; then
    redis-cli -h "${REDIS_HOST:-localhost}" -p "${REDIS_PORT:-6379}" DEL "passback:$1" >/dev/null 2>&1 || true
  else
    docker compose exec -T redis redis-cli DEL "passback:$1" >/dev/null 2>&1 || true
  fi
}

swipe() {
  local user="$1" door="$2" dir="$3" card="$4"
  curl -sf -H "X-API-Key: ${API_KEY}" -X POST "${API}/access/swipe" \
    -H "Content-Type: application/json" \
    -d "{\"userId\":\"${user}\",\"doorId\":\"${door}\",\"direction\":\"${dir}\",\"cardUid\":\"${card}\",\"timestamp\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}" >/dev/null
}

echo "============================================"
echo "  完整流程：大量刷卡 → 報表產出"
echo "============================================"

echo ""
echo ">>> [1/5] 健康檢查"
if curl -sf -H "X-API-Key: ${API_KEY}" "${API}/access/door/11111111-1111-1111-1111-111111111111/status" >/dev/null 2>&1 || \
   curl -sf "${API}/health" >/dev/null 2>&1; then
  echo "  access-api OK"
else
  echo "ERROR: access-api unreachable" >&2
  exit 1
fi
if curl -sf "${REPORT_URL}/ui/" >/dev/null 2>&1 || \
   curl -sf "${REPORT_URL}/health" >/dev/null 2>&1; then
  echo "  report-api OK"
else
  echo "ERROR: report-api unreachable" >&2
  exit 1
fi

echo ""
echo ">>> [2/5] 模擬大量人流（多員工 × 多門 × IN/OUT 交替）"
total=0
u_idx=0
for entry in "${USERS[@]}"; do
  user="${entry%%:*}"
  card="${entry##*:}"
  redis_del_passback "${user}"
  door="${DOORS[$((u_idx % ${#DOORS[@]}))]}"
  for ((p=0; p<PAIRS_PER_USER; p++)); do
    swipe "${user}" "${door}" "IN" "${card}"
    total=$((total + 1))
    if [[ "${INTERVAL}" != "0" ]]; then sleep "${INTERVAL}" 2>/dev/null || sleep 0.015; fi
    swipe "${user}" "${door}" "OUT" "${card}"
    total=$((total + 1))
    if [[ "${INTERVAL}" != "0" ]]; then sleep "${INTERVAL}" 2>/dev/null || sleep 0.015; fi
    # 換門模擬不同閘機流量
    door="${DOORS[$(( (u_idx + p) % ${#DOORS[@]} ))]}"
  done
  u_idx=$((u_idx + 1))
  echo "  user ${user:0:8}… ${PAIRS_PER_USER} pairs @ doors done"
done
echo "  已送出約 ${total} 次刷卡請求"

echo ""
echo ">>> [3/5] 等待 aggregation-worker 寫入 ClickHouse（15s）"
sleep 15
# Query ClickHouse: prefer cloud HTTP interface, fall back to local docker compose
if [[ -n "${CLICKHOUSE_ADDR:-}" && "${CLICKHOUSE_ADDR}" != *"localhost"* ]]; then
  ch_http_addr=$(echo "${CLICKHOUSE_ADDR}" | sed 's/:9440/:8443/; s/:9000/:8443/')
  if [[ "${ch_http_addr}" != *":"* ]]; then
    ch_http_addr="${ch_http_addr}:8443"
  fi
  EVENTS=$(curl -sf --max-time 10 \
    "https://${ch_http_addr}/?query=SELECT+count()+FROM+access_control.inout_events+WHERE+toDate(event_time)%3Dtoday()" \
    -u "${CLICKHOUSE_USER:-default}:${CLICKHOUSE_PASSWORD:-}" 2>/dev/null || echo "?")
else
  EVENTS=$(docker compose exec -T clickhouse clickhouse-client --password "${CLICKHOUSE_PASSWORD:-password123}" \
    --query "SELECT count() FROM access_control.inout_events WHERE toDate(event_time) = today()" 2>/dev/null || echo "?")
fi
echo "  今日 inout_events 筆數: ${EVENTS}"

echo ""
echo ">>> [4/5] 報表 API（JSON）"
echo "  --- 部門摘要（課長 / monthly）---"
curl -sf -H "X-API-Key: ${API_KEY}" -H "X-User-ID: ${MANAGER_ID}" \
  "${REPORT_URL}/reports/department?orgUnitId=${TEAM_ORG}&startDate=${MONTH_START}&endDate=${TODAY}&granularity=monthly" \
  | jq '{orgUnitName, summary, periodCount: (.periods | length)}'

echo "  --- 門禁熱點（CEO / 近 1h）---"
curl -sf -H "X-API-Key: ${API_KEY}" -H "X-User-ID: ${CEO_ID}" \
  "${REPORT_URL}/reports/analytics/door-heatmap?minutes=60" \
  | jq '{windowMinutes, topDoor: .doors[0]}'

echo "  --- 考勤趨勢（CEO / 全公司 / daily）---"
curl -sf -H "X-API-Key: ${API_KEY}" -H "X-User-ID: ${CEO_ID}" \
  "${REPORT_URL}/reports/analytics/attendance-trends?orgUnitId=${CORP_ORG}&startDate=${MONTH_START}&endDate=${TODAY}&granularity=daily" \
  | jq '{points: (.series | length), last: .series[-1]}'

echo ""
echo ">>> [5/5] 匯出檔案"
curl -sf -H "X-API-Key: ${API_KEY}" -H "X-User-ID: ${MANAGER_ID}" \
  "${REPORT_URL}/reports/export?orgUnitId=${TEAM_ORG}&startDate=${MONTH_START}&endDate=${TODAY}&format=csv&type=events" \
  -o ./report_demo.csv
echo "  CSV: report_demo.csv ($(wc -l < ./report_demo.csv | tr -d ' ') lines)"

curl -sf -H "X-API-Key: ${API_KEY}" -H "X-User-ID: ${MANAGER_ID}" \
  "${REPORT_URL}/reports/export?orgUnitId=${TEAM_ORG}&startDate=${MONTH_START}&endDate=${TODAY}&format=pdf&type=department" \
  -o ./report_demo.pdf
echo "  PDF: report_demo.pdf ($(wc -c < ./report_demo.pdf | tr -d ' ') bytes)"

echo ""
echo "============================================"
echo "  完成"
echo "  - Grafana 熱點/考勤: ${GRAFANA_URL:-http://localhost:3001} → Access Analytics"
echo "  - Prometheus 告警指標: report_passback_deny_1m_max"
echo "============================================"
