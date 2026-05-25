.PHONY: up down build seed migrate demo demo-ban demo-report demo-full demo-passback-alert swipe swipe-demo ban unban test test-unit test-integration test-e2e-pipeline verify-pipeline hooks load-demo load-shift-change shift-change-prep seed-load-users test-report-cache benchmark-report-api

# Install repo git hooks (strips Cursor Co-authored-by from commits)
hooks:
	git config core.hooksPath .githooks
	chmod +x .githooks/commit-msg .githooks/prepare-commit-msg
	@echo "Git hooks installed: .githooks (Cursor co-author will be removed)"

COMPOSE = docker compose
DEMO_USER ?= 22222222-2222-2222-2222-222222222222
ADMIN_URL ?= http://localhost:8081

up:
	$(COMPOSE) up -d --build
	@echo "Waiting for services..."
	@sleep 15
	@$(MAKE) init-kafka-topics
	$(COMPOSE) restart cache-invalidation-worker
	@sleep 5
	@$(MAKE) schema-ch
	@$(MAKE) schema-ch-migrate
	@$(MAKE) seed-ch
	@$(MAKE) seed

init-kafka-topics:
	$(COMPOSE) exec kafka /opt/kafka/bin/kafka-topics.sh --create --if-not-exists \
		--bootstrap-server kafka:9092 --topic inout-events --partitions 1 --replication-factor 1
	$(COMPOSE) exec kafka /opt/kafka/bin/kafka-topics.sh --create --if-not-exists \
		--bootstrap-server kafka:9092 --topic permission-events --partitions 1 --replication-factor 1

down:
	$(COMPOSE) down -v

build:
	$(COMPOSE) build

# Ensure ClickHouse schema exists (idempotent CREATE IF NOT EXISTS).
schema-ch:
	@test -f clickhouse/init.sql
	$(COMPOSE) exec -T clickhouse clickhouse-client --password password123 --multiquery < clickhouse/init.sql
	@echo "ClickHouse schema applied"

schema-ch-migrate:
	@test -f clickhouse/migrate-analytics.sql
	$(COMPOSE) exec -T clickhouse clickhouse-client --password password123 --multiquery < clickhouse/migrate-analytics.sql
	@echo "ClickHouse analytics migration applied"

# Load demo org/employee into ClickHouse (idempotent TRUNCATE + INSERT).
seed-ch:
	@test -f clickhouse/seed.sql
	$(COMPOSE) exec -T clickhouse clickhouse-client --password password123 --multiquery < clickhouse/seed.sql
	@echo "ClickHouse seed applied"

seed:
	@chmod +x scripts/utils/seed-redis.sh scripts/demo/demo.sh scripts/demo/demo-ban.sh scripts/demo/verify-pipeline.sh
	@./scripts/utils/seed-redis.sh

demo: swipe-demo

demo-ban:
	@./scripts/demo/demo-ban.sh

swipe-demo:
	@./scripts/demo/demo.sh

ban:
	curl -sf -X POST "$(ADMIN_URL)/admin/employees/$(USER)/ban" | jq .

unban:
	curl -sf -X POST "$(ADMIN_URL)/admin/employees/$(USER)/unban" | jq .

# Simulate one badge swipe (default: IN). Example: make swipe DIRECTION=OUT
swipe:
	cd badge-reader-sim && go run ./cmd/sim \
		--api http://localhost:8080 \
		--direction $(or $(DIRECTION),IN)

test: test-unit

test-unit:
	cd access-api && go test ./...
	cd admin-api && go test ./...
	cd cache-invalidation-worker && go test ./...
	cd report-api && go test ./...

test-integration:
	cd access-api/tests/integration && go mod tidy && go test -tags=integration . -count=1 -timeout=5m

test-e2e-pipeline:
	E2E_PIPELINE=1 $(MAKE) test-integration

verify-pipeline:
	@./scripts/demo/verify-pipeline.sh

verify-performance:
	@chmod +x scripts/demo/verify-performance.sh
	@./scripts/demo/verify-performance.sh

demo-report:
	@chmod +x scripts/demo/demo-report.sh
	@./scripts/demo/demo-report.sh

# Bulk swipe traffic → ClickHouse → CSV/PDF reports (see scripts/demo/demo-full-flow.sh)
demo-full:
	@chmod +x scripts/demo/demo-full-flow.sh
	@./scripts/demo/demo-full-flow.sh

demo-passback-alert:
	@chmod +x scripts/demo/demo-passback-alert.sh
	@./scripts/demo/demo-passback-alert.sh

logs:
	$(COMPOSE) logs -f access-api admin-api aggregation-worker cache-invalidation-worker report-api

# Generate traffic spike for Grafana dashboard demo
LOAD_COUNT ?= 200
LOAD_INTERVAL ?= 20ms
load-demo:
	cd badge-reader-sim && go run ./cmd/sim \
		--api http://localhost:8080 \
		--count $(LOAD_COUNT) --interval $(LOAD_INTERVAL)

# Shift-change spike: default 90k distinct users, all IN (enter factory)
SHIFT_COUNT ?= 90000
SHIFT_WORKERS ?= 150
SHIFT_RAMP ?= 0s
# Clear Redis passback state before a fresh 90k IN run (avoids ANTI_PASSBACK from prior demos)
shift-change-prep:
	$(COMPOSE) exec redis redis-cli FLUSHDB
	@$(MAKE) seed
	@echo "Redis flushed and demo cards re-seeded — ready for load-shift-change"

load-shift-change:
	@mkdir -p data/outbox
	cd badge-reader-sim && go run ./cmd/load \
		--api http://localhost:8080 \
		--count $(SHIFT_COUNT) --workers $(SHIFT_WORKERS) \
		--direction IN --unique-users=true --ramp $(SHIFT_RAMP)

# Optional: seed 90k synthetic employees into ClickHouse (slow; for report/analytics demos)
LOAD_USER_COUNT ?= 90000
test-report-cache:
	@chmod +x scripts/demo/test-report-cache.sh
	@./scripts/demo/test-report-cache.sh

benchmark-report-api:
	@chmod +x scripts/demo/benchmark-report-api.sh
	@./scripts/demo/benchmark-report-api.sh

seed-load-users:
	@chmod +x scripts/utils/gen-load-users-sql.sh
	@./scripts/utils/gen-load-users-sql.sh $(LOAD_USER_COUNT) > clickhouse/seed-load-users.sql
	$(COMPOSE) exec -T clickhouse clickhouse-client --password password123 --multiquery < clickhouse/seed-load-users.sql
	@echo "Seeded $(LOAD_USER_COUNT) load-test employees into ClickHouse"

REPORT_URL ?= http://localhost:8082
ORG_UNIT ?= a0000000-0000-0000-0000-000000000003
REPORT_USER ?= 22222222-2222-2222-2222-222222222222

report-export-pdf:
	curl -sf -H "X-User-ID: $(REPORT_USER)" 	  "$(REPORT_URL)/reports/export?orgUnitId=$(ORG_UNIT)&startDate=$(shell date +%Y-%m-01)&endDate=$(shell date +%Y-%m-%d)&format=pdf&type=department" 	  -o report_export.pdf && echo "wrote report_export.pdf"
