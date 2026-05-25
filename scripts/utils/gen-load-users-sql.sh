#!/usr/bin/env bash
# Generate ClickHouse seed for synthetic load-test employees (optional, for reporting realism).
# Usage: ./scripts/utils/gen-load-users-sql.sh 90000 > clickhouse/seed-load-users.sql
#        make seed-load-users   # applies via clickhouse-client

set -euo pipefail
COUNT="${1:-90000}"
ORG="${2:-a0000000-0000-0000-0000-000000000003}"
TEAM_NAME="${3:-Team-A}"

echo "-- Auto-generated load-test employees (COUNT=$COUNT)"
echo "INSERT INTO employee (id, name, org_unit_id, card_uid, is_active, report_role, updated_at) VALUES"

for ((i=0; i<COUNT; i++)); do
  id=$(printf "00000000-0000-4000-a000-%012x" "$i")
  name=$(printf "Load User %d" "$i")
  card=$(printf "LOAD%08d" "$i")
  comma=","
  if (( i == COUNT - 1 )); then comma=";"; fi
  printf "('%s','%s','%s','%s',1,'EMPLOYEE',now())%s\n" "$id" "$name" "$ORG" "$card" "$comma"
done
