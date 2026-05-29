#!/usr/bin/env bash
# ============================================================================
# perf-viz.sh — PACS Live Performance Dashboard
# Collects latency samples from access-api (Fast Path) and all report-api
# endpoints, then renders a colour ASCII dashboard with:
#   • Horizontal bar charts  (p50 / p95 / p99 / max)
#   • Latency histogram
#   • Per-endpoint comparison chart
#   • SLO pass/fail badge
# ============================================================================
set -euo pipefail

# ---------------------------------------------------------------------------
# Environment
# ---------------------------------------------------------------------------
ENV_FILE="$(dirname "$0")/../../.env"
if [ -f "$ENV_FILE" ]; then source "$ENV_FILE"; fi

if [ -n "${REDIS_ADDR:-}" ]; then
  REDIS_HOST="${REDIS_ADDR%:*}"
  REDIS_PORT="${REDIS_ADDR#*:}"
fi

API_KEY="${API_KEY:-dev-api-key-2026}"
API_URL="${API_URL:-http://localhost:8080}"
REPORT_URL="${REPORT_URL:-http://localhost:8082}"
MANAGER="${MANAGER:-cccccccc-cccc-cccc-cccc-cccccccccccc}"
ORG="${ORG:-a0000000-0000-0000-0000-000000000003}"
FAST_N="${FAST_N:-30}"      # swipe samples
REPORT_N="${REPORT_N:-20}"  # report-api samples per endpoint
FAST_SLO="${FAST_SLO:-50}"   # ms
REPORT_SLO="${REPORT_SLO:-200}" # ms
TODAY=$(date +%Y-%m-%d)
MONTH=$(date +%Y-%m-01)

# ---------------------------------------------------------------------------
# ANSI colours
# ---------------------------------------------------------------------------
R=$'\e[0;31m'   # red
G=$'\e[0;32m'   # green
Y=$'\e[0;33m'   # yellow
B=$'\e[0;34m'   # blue
M=$'\e[0;35m'   # magenta
C=$'\e[0;36m'   # cyan
W=$'\e[1;37m'   # bold white
DIM=$'\e[2m'
NC=$'\e[0m'

BAR_CHAR="█"
SHADE_CHAR="░"
BAR_WIDTH=40    # total bar columns

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
repeat_char() {
  local char="$1" n="$2"
  (( n <= 0 )) && return
  local s
  s=$(printf "%${n}s")
  printf '%s' "${s// /$char}"
}

# Draw a horizontal bar: value / max mapped to BAR_WIDTH cols
# draw_bar <value_ms> <max_ms> <color> <slo_ms>
draw_bar() {
  local val="$1" max="$2" color="$3" slo="$4"
  local filled shade badge=""
  filled=$(awk "BEGIN { f=int(($val/$max)*$BAR_WIDTH); if(f>$BAR_WIDTH) f=$BAR_WIDTH; print f }")
  shade=$(( BAR_WIDTH - filled ))
  local slo_marker
  slo_marker=$(awk "BEGIN { print int(($slo/$max)*$BAR_WIDTH) }")

  printf "${color}"
  repeat_char "$BAR_CHAR" "$filled"
  printf "${DIM}"
  repeat_char "$SHADE_CHAR" "$shade"
  printf "${NC}"

  # SLO verdict
  if awk "BEGIN { exit !($val > $slo) }"; then
    badge="${R}✗ FAIL${NC}"
  else
    badge="${G}✓ PASS${NC}"
  fi
  printf " ${W}%5.0fms${NC} %s\n" "$val" "$badge"
}

# Compute percentile from a sorted array passed as args
# percentile <p 0-100> <sorted values...>
percentile() {
  local p=$1; shift
  local arr=("$@")
  local n=${#arr[@]}
  local idx=$(( (n * p) / 100 ))
  (( idx >= n )) && idx=$(( n - 1 ))
  (( idx < 0  )) && idx=0
  echo "${arr[$idx]}"
}

# Collect N integer-ms latency samples from a curl call
# collect_samples <n> <label> <curl_extra_args...>
collect_samples() {
  local n="$1" label="$2"; shift 2
  local samples=() errors=0
  printf "  ${DIM}Sampling %-28s [" "$label" >&2
  for ((i=0; i<n; i++)); do
    local ms
    ms=$(curl -sf -o /dev/null -w '%{time_total}' "$@" 2>/dev/null \
         | awk '{printf "%.0f", $1*1000}') || { errors=$(( errors + 1 )); printf "${R}.${NC}" >&2; continue; }
    samples+=("$ms")
    printf "${G}·${NC}" >&2
  done
  printf "] ${DIM}%d ok, %d err${NC}\n" "${#samples[@]}" "$errors" >&2
  # Only emit the sample list on stdout (captured by caller)
  echo "${samples[*]:-}"
}

# Print a latency histogram (buckets of <bucket_ms> ms)
# histogram <bucket_ms> <max_bar_width> <sorted_values...>
histogram() {
  local bucket="$1" bar_w="$2"; shift 2
  # Delegate entirely to awk: portable, no associative array needed
  printf '%s\n' "$@" | sort -n | awk \
    -v bucket="$bucket" -v bar_w="$bar_w" \
    -v CY="${Y}" -v CC="${C}" -v CW="${W}" -v CN="${NC}" -v DIM="${DIM}" '
  {
    b = int($1 / bucket) * bucket
    counts[b]++
    if (counts[b] > max_c) max_c = counts[b]
    if (!(b in seen)) { order[++n] = b; seen[b]=1 }
  }
  END {
    if (max_c == 0) exit
    for (i = 1; i <= n; i++) {
      bk = order[i]
      cnt = counts[bk]
      bar_len = int((cnt / max_c) * bar_w)
      if (bar_len < 1) bar_len = 1
      shade = bar_w - bar_len
      hi = bk + bucket - 1
      printf "  %s%4d-%4dms%s | %s", CC, bk, hi, CN, CY
      for (j = 0; j < bar_len; j++) printf "#"
      printf "%s", DIM
      for (j = 0; j < shade; j++) printf "."
      printf "%s| %s%d%s\n", CN, CW, cnt, CN
    }
  }
  '
}


# Section header
section() {
  echo
  printf "${M}${W}━━━  %s  ━━━${NC}\n" "$1"
}

# Box header
box_header() {
  local title="$1" width=66
  echo
  printf "${C}╔%s╗${NC}\n" "$(repeat_char "═" $((width-2)))"
  printf "${C}║${W}%-$((width-2))s${C}║${NC}\n" "  $title"
  printf "${C}╚%s╝${NC}\n" "$(repeat_char "═" $((width-2)))"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
# Clear screen (only if TERM is set)
[[ -n "${TERM:-}" ]] && clear || echo
box_header "PACS Performance Dashboard  •  $(date '+%Y-%m-%d %H:%M:%S')"
printf "  ${DIM}access-api  : ${NC}${W}%s${NC}\n" "$API_URL"
printf "  ${DIM}report-api  : ${NC}${W}%s${NC}\n" "$REPORT_URL"
printf "  ${DIM}SLO targets : ${NC}${W}Fast-path p99 < ${FAST_SLO}ms  │  Report-api p95 < ${REPORT_SLO}ms${NC}\n"

# ── 1. Fast Path (access-api /access/swipe) ──────────────────────────────
section "Fast Path  (access-api)  —  ${FAST_N} samples"

# Clear passback for test user so every swipe hits the normal ALLOW path
if command -v redis-cli >/dev/null 2>&1; then
  redis-cli -h "${REDIS_HOST:-localhost}" -p "${REDIS_PORT:-6379}" \
    DEL "passback:22222222-2222-2222-2222-222222222222" >/dev/null 2>&1 || true
else
  docker compose exec -T redis redis-cli \
    DEL "passback:22222222-2222-2222-2222-222222222222" >/dev/null 2>&1 || true
fi

raw_fast=$(collect_samples "$FAST_N" "/access/swipe" \
  -H "X-API-Key: ${API_KEY}" \
  -X POST "${API_URL}/access/swipe" \
  -H "Content-Type: application/json" \
  -d "{\"userId\":\"22222222-2222-2222-2222-222222222222\",\"doorId\":\"11111111-1111-1111-1111-111111111111\",\"direction\":\"IN\",\"cardUid\":\"CARD001\",\"timestamp\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}")

if [[ -z "$raw_fast" ]]; then
  echo "  ${R}ERROR: No successful samples — is access-api reachable?${NC}"
  fast_ok=0
else
  IFS=' ' read -ra fast_arr <<< "$raw_fast"
  IFS=$'\n' fast_sorted=($(printf '%s\n' "${fast_arr[@]}" | sort -n)); unset IFS
  fast_max=${fast_sorted[${#fast_sorted[@]}-1]}
  fast_min=${fast_sorted[0]}
  fast_p50=$(percentile 50 "${fast_sorted[@]}")
  fast_p95=$(percentile 95 "${fast_sorted[@]}")
  fast_p99=$(percentile 99 "${fast_sorted[@]}")
  fast_avg=$(awk "BEGIN { s=0; for(i=1;i<=${#fast_arr[@]};i++) s+=ARGV[i]; printf \"%.0f\", s/${#fast_arr[@]} }" "${fast_arr[@]}")
  chart_max=$(awk "BEGIN { print ($fast_max > $FAST_SLO ? $fast_max : $FAST_SLO) + 10 }")

  echo
  printf "  ${DIM}%-6s${NC} │ " "p50"
  draw_bar "$fast_p50" "$chart_max" "${G}" "$FAST_SLO"
  printf "  ${DIM}%-6s${NC} │ " "p95"
  draw_bar "$fast_p95" "$chart_max" "${Y}" "$FAST_SLO"
  printf "  ${DIM}%-6s${NC} │ " "p99"
  draw_bar "$fast_p99" "$chart_max" "${R}" "$FAST_SLO"
  printf "  ${DIM}%-6s${NC} │ " "max"
  draw_bar "$fast_max" "$chart_max" "${M}" "$FAST_SLO"

  printf "\n  ${DIM}avg=${NC}${W}%sms${NC}  ${DIM}min=${NC}${W}%sms${NC}  ${DIM}n=${NC}${W}%d${NC}\n" \
    "$fast_avg" "$fast_min" "${#fast_arr[@]}"

  echo
  printf "  ${W}Latency distribution (bucket = 5ms):${NC}\n"
  histogram 5 30 "${fast_sorted[@]}"

  if awk "BEGIN { exit !($fast_p99 > $FAST_SLO) }"; then
    fast_ok=0
    printf "\n  ${R}${W}▶ SLO BREACH: p99 %sms > %sms${NC}\n" "$fast_p99" "$FAST_SLO"
  else
    fast_ok=1
    printf "\n  ${G}${W}▶ SLO OK: p99 %sms ≤ %sms${NC}\n" "$fast_p99" "$FAST_SLO"
  fi
fi

# ── 2. Report API endpoints ───────────────────────────────────────────────
section "Report API  —  ${REPORT_N} samples per endpoint"

# Use parallel indexed arrays (bash 3.x safe — no declare -A needed)
ep_names=()
ep_p50_arr=()
ep_p95_arr=()
ep_p99_arr=()
ep_max_arr=()
ep_ok_arr=()

endpoints=(
  "department|/reports/department?orgUnitId=${ORG}&startDate=${MONTH}&endDate=${TODAY}&granularity=daily"
  "workforce_util|/reports/analytics/workforce-utilization?orgUnitId=${ORG}&startDate=${MONTH}&endDate=${TODAY}"
  "attend_trends|/reports/analytics/attendance-trends?orgUnitId=${ORG}&startDate=${MONTH}&endDate=${TODAY}&granularity=monthly"
  "door_heatmap|/reports/analytics/door-heatmap?orgUnitId=${ORG}&minutes=60"
  "personal|/reports/personal?startDate=${MONTH}&endDate=${TODAY}"
)

echo
for ep in "${endpoints[@]}"; do
  IFS='|' read -r ep_name ep_path <<< "$ep"
  raw=$(collect_samples "$REPORT_N" "$ep_name" \
    -H "X-API-Key: ${API_KEY}" \
    -H "X-User-ID: ${MANAGER}" \
    "${REPORT_URL}${ep_path}") || true

  ep_names+=("$ep_name")
  if [[ -z "$raw" ]]; then
    ep_p50_arr+=(0); ep_p95_arr+=(9999); ep_p99_arr+=(9999)
    ep_max_arr+=(9999); ep_ok_arr+=(0)
  else
    IFS=' ' read -ra arr <<< "$raw"
    IFS=$'\n' sorted_ep=($(printf '%s\n' "${arr[@]}" | sort -n)); unset IFS
    ep_p50_arr+=("$(percentile 50 "${sorted_ep[@]}")")
    ep_p95_arr+=("$(percentile 95 "${sorted_ep[@]}")")
    ep_p99_arr+=("$(percentile 99 "${sorted_ep[@]}")")
    ep_max_arr+=("${sorted_ep[${#sorted_ep[@]}-1]}")
    ep_ok_arr+=(1)
  fi
done

# Find global max for chart scale
global_max=0
for i in "${!ep_names[@]}"; do
  (( ep_max_arr[i] > global_max )) && global_max=${ep_max_arr[i]}
done
chart_max_r=$(awk "BEGIN { print ($global_max > $REPORT_SLO ? $global_max : $REPORT_SLO) + 20 }")

echo
printf "  ${W}%-20s  %s\n${NC}" "Endpoint" "p95 bar (SLO=${REPORT_SLO}ms) ──────────────────── result"
printf "  ${DIM}%s${NC}\n" "$(repeat_char "─" 68)"

report_fails=0
for i in "${!ep_names[@]}"; do
  ep_n="${ep_names[i]}"
  ok_v="${ep_ok_arr[i]}"
  if (( ok_v == 0 )); then
    printf "  ${R}%-20s  ERROR (no samples)${NC}\n" "$ep_n"
    report_fails=$(( report_fails + 1 ))
    continue
  fi
  p50v="${ep_p50_arr[i]}"
  p95v="${ep_p95_arr[i]}"
  p99v="${ep_p99_arr[i]}"

  # Colour by p95 vs SLO
  if awk "BEGIN { exit !($p95v > $REPORT_SLO) }"; then
    bar_col="${R}"; report_fails=$(( report_fails + 1 )); badge="${R}✗${NC}"
  else
    bar_col="${G}"; badge="${G}✓${NC}"
  fi

  filled=$(awk "BEGIN { print int(($p95v/$chart_max_r)*$BAR_WIDTH) }")
  shade=$(( BAR_WIDTH - filled ))
  (( filled > BAR_WIDTH )) && filled=$BAR_WIDTH && shade=0

  printf "  ${C}%-20s${NC} │${bar_col}" "$ep_n"
  repeat_char "$BAR_CHAR" "$filled"
  printf "${DIM}"
  repeat_char "$SHADE_CHAR" "$shade"
  printf "${NC}│ ${W}p50=%-5s p95=%-5s p99=%-5s${NC} %b\n" \
    "${p50v}ms" "${p95v}ms" "${p99v}ms" "$badge"
done

# SLO gauge (% of budget used)
echo
printf "  ${W}SLO budget gauge  (100%% = %dms):${NC}\n" "$REPORT_SLO"
for i in "${!ep_names[@]}"; do
  ep_n="${ep_names[i]}"
  (( ep_ok_arr[i] == 0 )) && continue
  local_p95="${ep_p95_arr[i]}"
  pct=$(awk "BEGIN { printf \"%.0f\", ($local_p95/$REPORT_SLO)*100 }")
  bar_len=$(awk "BEGIN { n=int(($local_p95/$REPORT_SLO)*24); if(n>24) n=24; print n }")
  shade_len=$(( 24 - bar_len ))
  if awk "BEGIN { exit !($local_p95 > $REPORT_SLO) }"; then
    col="${R}"
  elif awk "BEGIN { exit !($local_p95 > $REPORT_SLO * 0.8) }"; then
    col="${Y}"
  else
    col="${G}"
  fi
  printf "  ${C}%-20s${NC} [${col}" "$ep_n"
  repeat_char "$BAR_CHAR" "$bar_len"
  printf "${DIM}"
  repeat_char "$SHADE_CHAR" "$shade_len"
  printf "${NC}] ${W}%3d%%%s${NC}\n" "$pct" " of budget"
done

# ── 3. Summary scoreboard ─────────────────────────────────────────────────
section "Summary"
echo
total_fails=$(( (${fast_ok:-0} == 0 ? 1 : 0) + report_fails ))

if [[ "${fast_ok:-0}" -eq 1 ]]; then
  printf "  ${G}${W}[PASS]${NC}  Fast Path  p99=${fast_p99:-?}ms  (SLO < ${FAST_SLO}ms)\n"
else
  printf "  ${R}${W}[FAIL]${NC}  Fast Path  p99=${fast_p99:-?}ms  (SLO < ${FAST_SLO}ms)\n"
fi

if (( report_fails == 0 )); then
  printf "  ${G}${W}[PASS]${NC}  Report API  all endpoints within p95 SLO (< ${REPORT_SLO}ms)\n"
else
  printf "  ${R}${W}[FAIL]${NC}  Report API  %d endpoint(s) breached p95 SLO (< ${REPORT_SLO}ms)\n" "$report_fails"
fi

echo
if (( total_fails == 0 )); then
  printf "  ${G}${W}✔  ALL SLO TARGETS MET${NC}\n\n"
  exit 0
else
  printf "  ${R}${W}✘  %d SLO TARGET(S) BREACHED${NC}\n\n" "$total_fails"
  exit 1
fi
