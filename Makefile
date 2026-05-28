.PHONY: up down build seed migrate demo demo-ban demo-report demo-full demo-passback-alert \
        swipe swipe-demo ban unban test test-unit test-integration test-e2e-pipeline \
        verify-pipeline verify-performance hooks load-demo load-shift-change shift-change-prep \
        seed-load-users test-report-cache benchmark-report-api perf-viz schema-ch-cloud

# =============================================================================
# Auto-load .env (strip surrounding double-quotes from values)
# Supports both local docker-compose and GKE cloud environments.
# =============================================================================
ifneq (,$(wildcard ./.env))
  include .env
  # patsubst strips the surrounding "..." that shell-style .env files add
  API_URL        := $(patsubst "%",%,$(API_URL))
  ADMIN_URL      := $(patsubst "%",%,$(ADMIN_URL))
  REPORT_URL     := $(patsubst "%",%,$(REPORT_URL))
  REDIS_ADDR     := $(patsubst "%",%,$(REDIS_ADDR))
  CLICKHOUSE_ADDR     := $(patsubst "%",%,$(CLICKHOUSE_ADDR))
  CLICKHOUSE_USER     := $(patsubst "%",%,$(CLICKHOUSE_USER))
  CLICKHOUSE_PASSWORD := $(patsubst "%",%,$(CLICKHOUSE_PASSWORD))
  export API_URL ADMIN_URL REPORT_URL REDIS_ADDR
  export CLICKHOUSE_ADDR CLICKHOUSE_USER CLICKHOUSE_PASSWORD
endif

# Fall back to local docker-compose defaults when no .env overrides are present
API_URL    ?= http://localhost:8080
ADMIN_URL  ?= http://localhost:8081
REPORT_URL ?= http://localhost:8082

# Parse REDIS_ADDR → REDIS_HOST + REDIS_PORT (format: host:port)
ifneq ($(REDIS_ADDR),)
  REDIS_HOST := $(word 1,$(subst :, ,$(REDIS_ADDR)))
  REDIS_PORT := $(word 2,$(subst :, ,$(REDIS_ADDR)))
endif
REDIS_HOST ?= localhost
REDIS_PORT ?= 6379

# =============================================================================
# Internal helpers
# =============================================================================
COMPOSE    = docker compose
DEMO_USER ?= 22222222-2222-2222-2222-222222222222

# redis-cmd: prefer local redis-cli (works for both cloud and local Redis),
# fall back to docker compose exec when redis-cli isn't installed.
define redis-cmd
$(if $(shell command -v redis-cli 2>/dev/null),\
  redis-cli -h $(REDIS_HOST) -p $(REDIS_PORT),\
  $(COMPOSE) exec -T redis redis-cli)
endef

# =============================================================================
# Git hooks
# =============================================================================
hooks:
	git config core.hooksPath .githooks
	chmod +x .githooks/commit-msg .githooks/prepare-commit-msg
	@echo "Git hooks installed."

# =============================================================================
# Local docker-compose lifecycle  (local only)
# =============================================================================
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
		--bootstrap-server kafka:9092 --topic inout-events --partitions 3 --replication-factor 1
	$(COMPOSE) exec kafka /opt/kafka/bin/kafka-topics.sh --create --if-not-exists \
		--bootstrap-server kafka:9092 --topic permission-events --partitions 3 --replication-factor 1

down:
	$(COMPOSE) down -v

build:
	$(COMPOSE) build

logs:
	$(COMPOSE) logs -f access-api admin-api aggregation-worker cache-invalidation-worker report-api

# =============================================================================
# ClickHouse schema — local docker-compose
# =============================================================================
schema-ch:
	@test -f clickhouse/init.sql
	$(COMPOSE) exec -T clickhouse clickhouse-client --password password123 --multiquery < clickhouse/init.sql
	@echo "ClickHouse schema applied"

schema-ch-migrate:
	@test -f clickhouse/migrate-analytics.sql
	$(COMPOSE) exec -T clickhouse clickhouse-client --password password123 --multiquery < clickhouse/migrate-analytics.sql
	@echo "ClickHouse analytics migration applied"

seed-ch:
	@test -f clickhouse/seed.sql
	$(COMPOSE) exec -T clickhouse clickhouse-client --password password123 --multiquery < clickhouse/seed.sql
	@echo "ClickHouse seed applied"

# =============================================================================
# ClickHouse schema — Cloud (reads CLICKHOUSE_ADDR/USER/PASSWORD from .env)
# =============================================================================
CH_CLOUD_URL ?= https://$(CLICKHOUSE_USER):$(CLICKHOUSE_PASSWORD)@$(CLICKHOUSE_ADDR)

schema-ch-cloud:
	@chmod +x scripts/utils/apply-cloud-ch.sh
	./scripts/utils/apply-cloud-ch.sh clickhouse/init.sql "$(CH_CLOUD_URL)"
	./scripts/utils/apply-cloud-ch.sh clickhouse/migrate-analytics.sql "$(CH_CLOUD_URL)"
	./scripts/utils/apply-cloud-ch.sh clickhouse/seed.sql "$(CH_CLOUD_URL)"
	@echo "Cloud ClickHouse schema and seed applied"

# Optional: seed 90k synthetic employees into ClickHouse Cloud
LOAD_USER_COUNT ?= 90000
seed-load-users:
	@chmod +x scripts/utils/gen-load-users-sql.sh
	@./scripts/utils/gen-load-users-sql.sh $(LOAD_USER_COUNT) > clickhouse/seed-load-users.sql
	./scripts/utils/apply-cloud-ch.sh clickhouse/seed-load-users.sql "$(CH_CLOUD_URL)"
	@echo "Seeded $(LOAD_USER_COUNT) load-test employees into ClickHouse Cloud"

# =============================================================================
# Redis seeding (cloud-safe: uses REDIS_ADDR from .env)
# =============================================================================
seed:
	@chmod +x scripts/utils/seed-redis.sh scripts/demo/demo.sh scripts/demo/demo-ban.sh scripts/demo/verify-pipeline.sh
	@./scripts/utils/seed-redis.sh

# =============================================================================
# Badge swipes (API_URL auto-resolved from .env → GKE or localhost fallback)
# =============================================================================
swipe:
	cd badge-reader-sim && go run ./cmd/sim \
		--api $(API_URL) \
		--direction $(or $(DIRECTION),IN)

demo: swipe-demo
swipe-demo:
	@./scripts/demo/demo.sh

ban:
	curl -sf -X POST "$(ADMIN_URL)/admin/employees/$(DEMO_USER)/ban" | jq .

unban:
	curl -sf -X POST "$(ADMIN_URL)/admin/employees/$(DEMO_USER)/unban" | jq .

# =============================================================================
# Load tests (API_URL from .env)
# =============================================================================
LOAD_COUNT    ?= 200
LOAD_INTERVAL ?= 20ms

load-demo:
	cd badge-reader-sim && go run ./cmd/sim \
		--api $(API_URL) \
		--count $(LOAD_COUNT) --interval $(LOAD_INTERVAL)

SHIFT_COUNT   ?= 90000
SHIFT_WORKERS ?= 150
SHIFT_RAMP    ?= 0s

# Cloud-safe: clears passback:* keys via targeted SCAN+DEL (never FLUSHDB)
shift-change-prep:
	@echo "Clearing passback:* keys from Redis ($(REDIS_HOST):$(REDIS_PORT))..."
	@if command -v redis-cli >/dev/null 2>&1; then \
		redis-cli -h $(REDIS_HOST) -p $(REDIS_PORT) \
			--scan --pattern 'passback:*' 2>/dev/null \
			| xargs -r redis-cli -h $(REDIS_HOST) -p $(REDIS_PORT) DEL 2>/dev/null || true; \
	else \
		$(COMPOSE) exec -T redis redis-cli \
			--scan --pattern 'passback:*' 2>/dev/null \
			| xargs -r $(COMPOSE) exec -T redis redis-cli DEL 2>/dev/null || true; \
	fi
	@$(MAKE) seed
	@echo "Redis passback keys cleared and demo cards re-seeded — ready for load-shift-change"

load-shift-change:
	@mkdir -p data/outbox
	cd badge-reader-sim && go run ./cmd/load \
		--api $(API_URL) \
		--count $(SHIFT_COUNT) --workers $(SHIFT_WORKERS) \
		--direction IN --unique-users=true --ramp $(SHIFT_RAMP)

# =============================================================================
# Demo scripts (all read API_URL / REPORT_URL / REDIS_ADDR from .env)
# =============================================================================
demo-ban:
	@./scripts/demo/demo-ban.sh

demo-report:
	@chmod +x scripts/demo/demo-report.sh
	@./scripts/demo/demo-report.sh

demo-full:
	@chmod +x scripts/demo/demo-full-flow.sh
	@./scripts/demo/demo-full-flow.sh

demo-passback-alert:
	@chmod +x scripts/demo/demo-passback-alert.sh
	@./scripts/demo/demo-passback-alert.sh

# =============================================================================
# Verification & benchmarks
# =============================================================================
verify-pipeline:
	@./scripts/demo/verify-pipeline.sh

verify-performance:
	@chmod +x scripts/demo/verify-performance.sh
	@./scripts/demo/verify-performance.sh

test-report-cache:
	@chmod +x scripts/demo/test-report-cache.sh
	@./scripts/demo/test-report-cache.sh

benchmark-report-api:
	@chmod +x scripts/demo/benchmark-report-api.sh
	@./scripts/demo/benchmark-report-api.sh

# ASCII performance dashboard (bar charts, histograms, SLO gauges)
perf-viz:
	@chmod +x scripts/demo/perf-viz.sh
	@TERM=xterm-256color ./scripts/demo/perf-viz.sh

# =============================================================================
# Unit & integration tests (local)
# =============================================================================
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

# =============================================================================
# Report export (REPORT_URL from .env)
# =============================================================================
ORG_UNIT     ?= a0000000-0000-0000-0000-000000000003
REPORT_USER  ?= 22222222-2222-2222-2222-222222222222

report-export-pdf:
	curl -sf -H "X-User-ID: $(REPORT_USER)" \
	  "$(REPORT_URL)/reports/export?orgUnitId=$(ORG_UNIT)&startDate=$(shell date +%Y-%m-01)&endDate=$(shell date +%Y-%m-%d)&format=pdf&type=department" \
	  -o report_export.pdf && echo "wrote report_export.pdf"
