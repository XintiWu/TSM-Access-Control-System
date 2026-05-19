.PHONY: up down build seed demo swipe swipe-demo test test-unit test-integration test-e2e-pipeline verify-pipeline hooks

# Install repo git hooks (strips Cursor Co-authored-by from commits)
hooks:
	git config core.hooksPath .githooks
	chmod +x .githooks/commit-msg .githooks/prepare-commit-msg
	@echo "Git hooks installed: .githooks (Cursor co-author will be removed)"

COMPOSE = docker compose

up:
	$(COMPOSE) up -d --build
	@echo "Waiting for services..."
	@sleep 15
	@$(MAKE) seed

down:
	$(COMPOSE) down -v

build:
	$(COMPOSE) build

seed:
	@chmod +x scripts/seed-redis.sh scripts/demo.sh
	@./scripts/seed-redis.sh

demo: swipe-demo

swipe-demo:
	@./scripts/demo.sh

# Simulate one badge swipe (default: IN). Example: make swipe DIRECTION=OUT
swipe:
	cd badge-reader-sim && go run ./cmd/sim \
		--api http://localhost:8080 \
		--direction $(or $(DIRECTION),IN)

test: test-unit

test-unit:
	cd access-api && go test ./...

test-integration:
	cd access-api/tests/integration && go mod tidy && go test -tags=integration . -count=1 -timeout=5m

test-e2e-pipeline:
	E2E_PIPELINE=1 $(MAKE) test-integration

verify-pipeline:
	@chmod +x scripts/verify-pipeline.sh scripts/demo.sh scripts/seed-redis.sh
	@./scripts/verify-pipeline.sh

logs:
	$(COMPOSE) logs -f access-api aggregation-worker
