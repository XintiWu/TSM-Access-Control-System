.PHONY: up down build seed migrate demo demo-ban demo-report swipe swipe-demo ban unban test test-unit test-integration test-e2e-pipeline verify-pipeline hooks load-demo

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
	@$(MAKE) migrate
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

# Apply migrations on existing MariaDB volumes (002+). Fresh installs also run via docker-entrypoint-initdb.d.
migrate:
	@test -f migrations/002_employee.sql
	$(COMPOSE) exec -T mariadb mariadb -uaccess -paccess access_control < migrations/002_employee.sql
	@echo "Migration 002 applied"
	@test -f migrations/003_org_unit.sql
	$(COMPOSE) exec -T mariadb mariadb -uaccess -paccess access_control < migrations/003_org_unit.sql
	@echo "Migration 003 applied"
	@test -f migrations/004_pre_aggregated_reports.sql
	$(COMPOSE) exec -T mariadb mariadb -uaccess -paccess access_control < migrations/004_pre_aggregated_reports.sql
	@echo "Migration 004 applied"

seed:
	@chmod +x scripts/seed-redis.sh scripts/demo.sh scripts/demo-ban.sh scripts/verify-pipeline.sh
	@./scripts/seed-redis.sh

demo: swipe-demo

demo-ban:
	@./scripts/demo-ban.sh

swipe-demo:
	@./scripts/demo.sh

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
	@./scripts/verify-pipeline.sh

demo-report:
	@./scripts/demo-report.sh

logs:
	$(COMPOSE) logs -f access-api admin-api aggregation-worker cache-invalidation-worker report-api

# Generate traffic spike for Grafana dashboard demo
LOAD_COUNT ?= 200
LOAD_INTERVAL ?= 20ms
load-demo:
	cd badge-reader-sim && go run ./cmd/sim \
		--api http://localhost:8080 \
		--count $(LOAD_COUNT) --interval $(LOAD_INTERVAL)
