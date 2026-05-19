# Distributed Physical Access Control System

TSMC Cloud Native 2026 — Physical Access Control (Fast Path + Admin ban)

## Architecture

- **access-api** (8080): Stateless REST API (<50ms decision via Redis)
- **admin-api** (8081): Ban/unban employees → Kafka `permission-events`
- **cache-invalidation-worker**: Syncs `perm:denied` keys to Redis
- **Redis**: Permission deny-list + anti-passback state
- **Kafka** (`inout-events`, `permission-events`): Async event buffers
- **aggregation-worker**: Persists swipe events to MariaDB
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
make up              # docker compose up + migrate + seed Redis
make demo            # anti-passback demo (ban step uses Admin API)
make demo-ban        # Admin ban → Redis → swipe DENY → unban
make verify-pipeline # confirm swipe event reaches MariaDB via Kafka
make down            # tear down
```

Upgrading an existing DB volume (without `make down -v`):

```bash
make migrate         # apply migrations/002_employee.sql
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

## Tests

```bash
make test-unit
make test-integration      # Redis testcontainers (no compose stack required)
make test-e2e-pipeline       # full Kafka→DB path (requires make up)
make verify-pipeline       # shell script: swipe + poll MariaDB
```

## Demo UUIDs (after `make seed`)

| Role | UUID |
|------|------|
| Normal user | `22222222-2222-2222-2222-222222222222` |
| Banned user (DB seed; ban via Admin for Redis) | `00000000-0000-0000-0000-000000000099` |
| Door | `11111111-1111-1111-1111-111111111111` |

Ban flow no longer seeds `perm:denied` in Redis — use `make ban` or `make demo-ban`.
