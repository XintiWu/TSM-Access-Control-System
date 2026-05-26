#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -lt 2 ]; then
  echo "Usage: $0 <sql-file> <clickhouse-url-with-creds>"
  echo "Example: $0 clickhouse/init.sql https://default:password@host:8443"
  exit 1
fi


SQL_FILE="$1"
CH_URL="$2"

if [ ! -f "$SQL_FILE" ]; then
  echo "Error: File $SQL_FILE not found." >&2
  exit 1
fi

# Extract username:password and clean URL
USER_PASS=$(echo "$CH_URL" | grep -oE '//[^@]+@' | tr -d '/@' || echo "")
CLEAN_URL=$(echo "$CH_URL" | sed -E 's/\/\/[^@]+@/\/\//')

if [ -z "$USER_PASS" ]; then
  echo "Error: URL must contain username and password (e.g. https://user:pass@host:port)" >&2
  exit 1
fi

echo "Applying $SQL_FILE to ClickHouse Cloud..."

# Parse SQL into statements split by null byte, piping directly into while loop
awk '
  BEGIN { RS=";"; ORS="\0" }
  # Remove line comments starting with --
  { gsub(/--[^\n]*/, "") }
  # Remove line comments starting with #
  { gsub(/#[^\n]*/, "") }
  # Print non-empty queries
  { 
    gsub(/^[ \t\n\r]+/, "")
    gsub(/[ \t\n\r]+$/, "")
    if ($0 != "") {
      printf "%s\0", $0
    }
  }
' "$SQL_FILE" | while IFS= read -r -d '' query; do
  # Strip leading/trailing whitespaces and newlines
  query_trimmed=$(echo "$query" | sed -e 's/^[[:space:]\n]*//' -e 's/[[:space:]\n]*$//')
  
  if [ -z "$query_trimmed" ]; then
    continue
  fi
  
  echo "--------------------------------------------------"
  echo "Executing query:"
  echo "$query_trimmed" | head -n 3
  if [ "$(echo "$query_trimmed" | wc -l)" -gt 3 ]; then
    echo "... (truncated)"
  fi
  
  # POST the query to ClickHouse HTTP interface using curl
  RESPONSE=$(curl -s -w "\n%{http_code}" -u "$USER_PASS" --data-binary "$query_trimmed" "$CLEAN_URL")
  
  HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
  BODY=$(echo "$RESPONSE" | sed '$d')
  
  if [ "$HTTP_CODE" -ne 200 ]; then
    echo "ERROR: ClickHouse returned HTTP $HTTP_CODE" >&2
    echo "Response body:" >&2
    echo "$BODY" >&2
    exit 1
  fi
  if [ -n "$BODY" ]; then
    echo "Response:"
    echo "$BODY"
  fi
done

echo "Successfully applied $SQL_FILE!"
