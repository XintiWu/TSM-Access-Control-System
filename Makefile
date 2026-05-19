.PHONY: up down build seed demo swipe swipe-demo test test-unit test-integration

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

logs:
	$(COMPOSE) logs -f access-api aggregation-worker
