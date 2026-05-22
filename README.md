# Distributed Physical Access Control System

TSMC Cloud Native 2026 — Physical Access Control (Fast Path + Admin ban)

## Architecture

- **access-api** (8080): Stateless REST API (<50ms decision via Redis)
- **admin-api** (8081): Ban/unban employees → Kafka `permission-events`
- **cache-invalidation-worker**: Syncs `perm:denied` keys to Redis
- **Redis**: Permission deny-list + anti-passback state
- **Kafka** (`inout-events`, `permission-events`): Async event buffers
- **aggregation-worker**: Persists swipe events to ClickHouse
- **ClickHouse**: Events, org tree, employees (single persistent store)
- **report-api** (8082): Reports and CSV/PDF export
- **badge-reader-sim**: CLI simulator for badge swipes

## Git commits (avoid Cursor on Contributors)

After clone, run once:

```bash
make hooks
```

This installs `.githooks/` so any `Co-authored-by: Cursor <cursoragent@cursor.com>` line is removed before the commit is created. When committing manually, use plain `git commit -m "..."` (do not use `--trailer` for Cursor).

## Quick Start

**Prerequisite:** Docker Desktop must be running (`docker ps` should succeed).

```bash
make up              # docker compose up + ClickHouse seed + Redis seed
make demo            # anti-passback demo (ban step uses Admin API)
make demo-ban        # Admin ban → Redis → swipe DENY → unban
make verify-pipeline # confirm swipe event reaches ClickHouse via Kafka
make down            # tear down
```

Re-seed ClickHouse demo data (without wiping volumes):

```bash
make seed-ch
```

If you see `Cannot connect to the Docker daemon`, open **Docker Desktop** and wait until it is ready, then run `make up` again.

## API

### Access API (port 8080)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/access/swipe` | Process badge swipe |
| GET | `/access/employee/{userId}/state` | Current IN/OUT state |
| GET | `/access/door/{doorId}/status` | Door ONLINE/OFFLINE |
| GET | `/health` | Health check |
| GET | `/metrics` | Prometheus metrics |

### Admin API (port 8081)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/admin/employees/{userId}/ban` | Deactivate employee + publish BAN |
| POST | `/admin/employees/{userId}/unban` | Reactivate + publish UNBAN |
| GET | `/health` | Health check |

```bash
make ban USER=22222222-2222-2222-2222-222222222222
make unban USER=22222222-2222-2222-2222-222222222222
```

### Swipe Example

```bash
curl -X POST http://localhost:8080/access/swipe \
  -H "Content-Type: application/json" \
  -d '{
    "userId": "22222222-2222-2222-2222-222222222222",
    "doorId": "11111111-1111-1111-1111-111111111111",
    "direction": "IN",
    "cardUid": "CARD001",
    "timestamp": "2026-05-19T08:00:00Z"
  }'
```

## Badge Reader Simulator

`go.mod` 在 `badge-reader-sim/` 目錄內，請先 `cd` 進去，或用 Makefile：

```bash
# 從專案根目錄（推薦）
make swipe
make swipe DIRECTION=OUT

# 或手動進入模組目錄
cd badge-reader-sim
go run ./cmd/sim --direction IN

# 壓測
cd badge-reader-sim && go run ./cmd/sim --count 100 --interval 50ms
```

## Report API (port 8082)

Slow-path reporting backed entirely by **ClickHouse** (events, org tree, employees).

| Method | Path | Description |
|--------|------|-------------|
| GET | `/reports/personal` | Personal attendance (`startDate`, `endDate`) |
| GET | `/reports/department` | Department summary (`orgUnitId`, date range, `granularity`) |
| GET | `/reports/audit` | Paginated audit log |
| GET | `/reports/export` | Sync download **CSV or PDF** (`format=csv|pdf`, `type=events|personal|department`) |
| POST | `/reports/export/jobs` | Async export (JSON body, returns `jobId`) |
| GET | `/reports/export/jobs/:jobId` | Poll/download completed export |

All endpoints require header **`X-User-ID`** (requester employee UUID).

```bash
make demo-report          # full report API walkthrough
make report-export-pdf    # quick department PDF download
```

Demo org unit: `a0000000-0000-0000-0000-000000000003` (Team-A).


## Observability (Prometheus + Grafana)

After `make up`, monitoring stacks start with the rest of the services:

| Service | URL | Notes |
|---------|-----|-------|
| Grafana | http://localhost:3001 | Login `admin` / `admin` |
| Prometheus | http://localhost:9090 | Scrapes `access-api:8080/metrics` |
| Access API metrics | http://localhost:8080/metrics | Raw Prometheus exposition |

**Dashboard:** *Access Control — Shift Change Monitor* (folder: Access Control) — QPS, p99 latency, ALLOW/DENY, deny reasons.

Generate a traffic spike for the charts:

```bash
make load-demo
# or: make load-demo LOAD_COUNT=500 LOAD_INTERVAL=10ms
```

Config lives under `monitoring/` (Prometheus scrape + Grafana provisioning).

## Tests

```bash
make test-unit
make test-integration      # Redis testcontainers (no compose stack required)
make test-e2e-pipeline       # full Kafka→DB path (requires make up)
make verify-pipeline       # shell script: swipe + poll ClickHouse
```

## Demo UUIDs (after `make seed`)

| Role | UUID |
|------|------|
| Normal user | `22222222-2222-2222-2222-222222222222` |
| Banned user (DB seed; ban via Admin for Redis) | `00000000-0000-0000-0000-000000000099` |
| Door | `11111111-1111-1111-1111-111111111111` |

Ban flow no longer seeds `perm:denied` in Redis — use `make ban` or `make demo-ban`.
