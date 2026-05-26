#!/usr/bin/env bash
# ==============================================================================
# TSMC Physical Access Control System (PACS) - Unified Script Manager
# ==============================================================================
# This script serves as a unified entry point to execute all demo, validation,
# performance, and helper scripts in the repository subdirectories.
# ==============================================================================
set -euo pipefail

# ANSI color codes using bash escape literal notation (safe for echo & printf)
COLOR_NC=$'\e[0m'
COLOR_BLACK=$'\e[0;30m'
COLOR_RED=$'\e[0;31m'
COLOR_GREEN=$'\e[0;32m'
COLOR_YELLOW=$'\e[0;33m'
COLOR_BLUE=$'\e[0;34m'
COLOR_PURPLE=$'\e[0;35m'
COLOR_CYAN=$'\e[0;36m'
COLOR_WHITE=$'\e[0;37m'
COLOR_BOLD=$'\e[1m'

# Paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEMO_DIR="${SCRIPT_DIR}/demo"
UTILS_DIR="${SCRIPT_DIR}/utils"

# Load environment variables if present
ENV_FILE="${SCRIPT_DIR}/../.env"
if [ -f "$ENV_FILE" ]; then
  source "$ENV_FILE"
fi

# Derive REDIS_HOST and REDIS_PORT from REDIS_ADDR if set
if [ -n "${REDIS_ADDR:-}" ]; then
  REDIS_HOST="${REDIS_ADDR%:*}"
  REDIS_PORT="${REDIS_ADDR#*:}"
fi

# Print banner
print_banner() {
  echo -e "${COLOR_CYAN}${COLOR_BOLD}======================================================================${COLOR_NC}"
  echo -e "${COLOR_CYAN}${COLOR_BOLD}      TSMC Cloud Native 2026 - PACS Distributed Access Control        ${COLOR_NC}"
  echo -e "${COLOR_CYAN}${COLOR_BOLD}                      Unified Script Manager                          ${COLOR_NC}"
  echo -e "${COLOR_CYAN}${COLOR_BOLD}======================================================================${COLOR_NC}"
}

# Define available scripts with descriptions and paths
# Format: "key|display_name|category|path|description"
scripts=(
  "demo|PACS Basic Flow Demo|Demo|${DEMO_DIR}/demo.sh|Simulate basic IN/OUT swipes and anti-passback checks"
  "demo-ban|Admin Ban Flow Demo|Demo|${DEMO_DIR}/demo-ban.sh|Simulate admin employee ban, cache invalidation, and deny check"
  "demo-passback-alert|Anti-Passback Alert Spike|Demo|${DEMO_DIR}/demo-passback-alert.sh|Send 55 swipes to trigger Grafana/Prometheus Slack alert"
  "demo-report|Report API Features Demo|Demo|${DEMO_DIR}/demo-report.sh|Demo report role subtree filtering, audit log, CSV & PDF export"
  "demo-full-flow|Bulk Traffic & Full Flow Demo|Demo|${DEMO_DIR}/demo-full-flow.sh|Generate traffic load, aggregate, query report, and export PDF/CSV"
  "verify-pipeline|End-to-End Persistence Check|Verification|${DEMO_DIR}/verify-pipeline.sh|Verify swipe event reaches ClickHouse DB via Kafka"
  "verify-performance|SLA Targets Validation|Verification|${DEMO_DIR}/verify-performance.sh|Validate p99 swipe < 50ms and p95 reports < 200ms SLA"
  "perf-viz|Performance Dashboard|Verification|${DEMO_DIR}/perf-viz.sh|Live ASCII dashboard: bar charts, histograms, and SLO gauges for all endpoints"
  "benchmark-report-api|Report API Endpoint Bench|Verification|${DEMO_DIR}/benchmark-report-api.sh|Benchmark p95 latency for report-api endpoints"
  "test-report-cache|Redis Report Caching Check|Verification|${DEMO_DIR}/test-report-cache.sh|Check Report API Redis caching, TTL, and invalidation"
  "seed-redis|Redis Seeding Helper|Helper|${UTILS_DIR}/seed-redis.sh|Initialize Redis with test door heartbeats and card mappings"
  "gen-load-users-sql|90k User Seeding SQL Gen|Helper|${UTILS_DIR}/gen-load-users-sql.sh|Generate INSERT SQL statements for 90,000 synthetic employees"
)

# Run a specific script by its path
run_script() {
  local path="$1"
  local name="$2"
  shift 2

  if [[ ! -f "$path" ]]; then
    echo -e "${COLOR_RED}${COLOR_BOLD}ERROR: Script not found at: ${path}${COLOR_NC}" >&2
    return 1
  fi

  chmod +x "$path"
  echo -e "${COLOR_GREEN}${COLOR_BOLD}Executing: ${name}...${COLOR_NC}"
  echo -e "${COLOR_YELLOW}Command: ${path} ${*:-}${COLOR_NC}"
  echo "----------------------------------------------------------------------"
  
  # Run the script, forwarding arguments safely without set -u
  local exit_code=0
  if [[ $# -gt 0 ]]; then
    "$path" "$@" || exit_code=$?
  else
    "$path" || exit_code=$?
  fi

  if [[ $exit_code -ne 0 ]]; then
    echo "----------------------------------------------------------------------"
    echo -e "${COLOR_RED}${COLOR_BOLD}FAIL: Script exited with error (code: $exit_code).${COLOR_NC}" >&2
    return $exit_code
  fi

  echo "----------------------------------------------------------------------"
  echo -e "${COLOR_GREEN}${COLOR_BOLD}SUCCESS: Execution finished.${COLOR_NC}"
  return 0
}

# Print usage instructions
print_usage() {
  echo -e "Usage: $0 [script-name] [arguments...]"
  echo -e "\nAvailable scripts:"
  echo -e "  ${COLOR_BOLD}Demo & Functional Runs:${COLOR_NC}"
  for item in "${scripts[@]}"; do
    IFS='|' read -r key display cat path desc <<< "$item"
    if [[ "$cat" == "Demo" ]]; then
      printf "    %-20s - %s\n" "${COLOR_CYAN}${key}${COLOR_NC}" "$desc"
    fi
  done
  
  echo -e "\n  ${COLOR_BOLD}Performance & SLA Verification:${COLOR_NC}"
  for item in "${scripts[@]}"; do
    IFS='|' read -r key display cat path desc <<< "$item"
    if [[ "$cat" == "Verification" ]]; then
      printf "    %-20s - %s\n" "${COLOR_CYAN}${key}${COLOR_NC}" "$desc"
    fi
  done

  echo -e "\n  ${COLOR_BOLD}Initialization & Helper Tools:${COLOR_NC}"
  for item in "${scripts[@]}"; do
    IFS='|' read -r key display cat path desc <<< "$item"
    if [[ "$cat" == "Helper" ]]; then
      printf "    %-20s - %s\n" "${COLOR_CYAN}${key}${COLOR_NC}" "$desc"
    fi
  done
  echo ""
}

# Main Execution Flow
if [[ $# -gt 0 ]]; then
  # Direct argument mode (保持原樣：跑完即結束)
  target_key="$1"
  shift

  target_key="${target_key%.sh}"
  
  found=0
  for item in "${scripts[@]}"; do
    IFS='|' read -r key display cat path desc <<< "$item"
    if [[ "$key" == "$target_key" ]]; then
      # 這裡若子腳本失敗，因為 set -e 會直接中斷退出
      run_script "$path" "$display" "$@"
      found=1
      break
    fi
  done

  if [[ "$found" -eq 0 ]]; then
    echo -e "${COLOR_RED}${COLOR_BOLD}Unknown script: ${target_key}${COLOR_NC}" >&2
    print_usage
    exit 1
  fi
else
  # Interactive mode - 封裝在 while true 循環中
  while true; do
    clear # 每次回到選單時清除畫面，維持終端機整潔
    print_banner
    echo -e "Please select a script to run:\n"
    
    categories=("Demo" "Verification" "Helper")
    menu_idx=1
    
    # 改用標準陣列宣告（相容舊版 Bash）
    menu_paths=()
    menu_names=()

    for cat_name in "${categories[@]}"; do
      if [[ "$cat_name" == "Demo" ]]; then
        echo -e "${COLOR_PURPLE}${COLOR_BOLD}--- Demo & Functional Runs ---${COLOR_NC}"
      elif [[ "$cat_name" == "Verification" ]]; then
        echo -e "${COLOR_PURPLE}${COLOR_BOLD}--- Performance & SLA Verification ---${COLOR_NC}"
      else
        echo -e "${COLOR_PURPLE}${COLOR_BOLD}--- Initialization & Helper Tools ---${COLOR_NC}"
      fi

      for item in "${scripts[@]}"; do
        IFS='|' read -r key display cat path desc <<< "$item"
        if [[ "$cat" == "$cat_name" ]]; then
          printf "  %2d) %-30s - %s\n" "$menu_idx" "${COLOR_CYAN}${display}${COLOR_NC}" "$desc"
          menu_paths["$menu_idx"]="$path"
          menu_names["$menu_idx"]="$display"
          menu_idx=$((menu_idx + 1))
        fi
      done
      echo ""
    done

    printf "  %2d) %s\n\n" "0" "${COLOR_RED}Exit${COLOR_NC}"

    # Prompt user for choice
    read -rp "Enter choice (0-$((menu_idx - 1))): " choice

    if [[ -z "$choice" || "$choice" == "0" ]]; then
      echo "Exiting..."
      exit 0
    fi

    if [[ ! "$choice" =~ ^[0-9]+$ ]] || [[ "$choice" -lt 1 ]] || [[ "$choice" -ge "$menu_idx" ]]; then
      echo -e "${COLOR_RED}Invalid choice. Press Enter to continue...${COLOR_NC}" >&2
      read -r
      continue
    fi

    selected_path="${menu_paths[$choice]}"
    selected_name="${menu_names[$choice]}"

    # Prompt for extra arguments
    read -rp "Enter extra arguments for script (optional): " extra_args
    
    echo ""
    # 使用迴圈時，我們不希望子腳本錯誤導致整個 Manager 崩潰退出，
    # 因此關閉 set -e 的觸發，改用布林判斷
    set +e
    eval "run_script \"\$selected_path\" \"\$selected_name\" $extra_args"
    set -e
    
    echo ""
    echo -e "${COLOR_YELLOW}Press Enter to return to the main menu...${COLOR_NC}"
    read -r
  done
fi