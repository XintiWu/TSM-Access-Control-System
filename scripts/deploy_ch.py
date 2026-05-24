import sys
import base64
import urllib.request
import urllib.error

host = "jm9s3w1q3z.asia-southeast1.gcp.clickhouse.cloud:8443"
user = "default"
password = "~rX_dPV64U5xq"

auth_header = "Basic " + base64.b64encode(f"{user}:{password}".encode("utf-8")).decode("utf-8")

def execute_query(sql, db=None):
    sql = sql.strip()
    if not sql:
        return
    url = f"https://{host}/"
    if db:
        url += f"?database={db}"
    
    req = urllib.request.Request(
        url,
        data=sql.encode("utf-8"),
        headers={
            "Authorization": auth_header,
            "Content-Type": "text/plain"
        },
        method="POST"
    )
    try:
        with urllib.request.urlopen(req) as resp:
            resp.read()
    except urllib.error.HTTPError as e:
        print(f"Error running query: {sql[:100]}...")
        print(f"HTTP Error: {e.code} - {e.read().decode('utf-8')}")
        sys.exit(1)
    except Exception as e:
        print(f"Error running query: {sql[:100]}...")
        print(f"Exception: {e}")
        sys.exit(1)

def run_file(filepath, db=None):
    print(f"Running file {filepath}...")
    with open(filepath, "r", encoding="utf-8") as f:
        content = f.read()
    
    # Simple parsing of statements by semicolon, avoiding splitting inside comments or empty blocks
    statements = []
    current = []
    lines = content.split("\n")
    for line in lines:
        stripped = line.strip()
        if stripped.startswith("--") or not stripped:
            continue
        current.append(line)
        if stripped.endswith(";"):
            statements.append("\n".join(current))
            current = []
    
    if current:
        statements.append("\n".join(current))
        
    for stmt in statements:
        stmt = stmt.strip()
        if stmt:
            execute_query(stmt, db)

# Run deployments
print("Initializing database...")
execute_query("CREATE DATABASE IF NOT EXISTS access_control")
run_file("clickhouse/init.sql", "access_control")
run_file("clickhouse/migrate-analytics.sql", "access_control")
run_file("clickhouse/seed.sql", "access_control")
print("ClickHouse schema and seed data deployed successfully!")
