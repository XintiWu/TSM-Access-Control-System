# Distributed Physical Access Control System

TSMC Cloud Native 2026 — Access Fast Path (刷卡機 / Badge Reader)

## Architecture

- **access-api**: Stateless REST API (<50ms decision via Redis)
- **Redis**: Permission deny-list + anti-passback state
- **Kafka** (`inout-events`): Async event buffer
- **aggregation-worker**: Persists events to MariaDB
- **badge-reader-sim**: CLI simulator for badge swipes

## Quick Start

**Prerequisite:** Docker Desktop must be running (`docker ps` should succeed).

```bash
make up      # docker compose up + seed Redis
make demo    # run anti-passback demo script
make down    # tear down
```

If you see `Cannot connect to the Docker daemon`, open **Docker Desktop** and wait until it is ready, then run `make up` again.

## API

| Method | Path | Description |
|--------|------|-------------|
| POST | `/access/swipe` | Process badge swipe |
| GET | `/access/employee/{userId}/state` | Current IN/OUT state |
| GET | `/access/door/{doorId}/status` | Door ONLINE/OFFLINE |
| GET | `/health` | Health check |
| GET | `/metrics` | Prometheus metrics |

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
make test-integration   # requires Docker
```

## Demo UUIDs (after `make seed`)

| Role | UUID |
|------|------|
| Normal user | `22222222-2222-2222-2222-222222222222` |
| Banned user | `00000000-0000-0000-0000-000000000099` |
| Door | `11111111-1111-1111-1111-111111111111` |
