.PHONY: build up down logs test test-unit test-demo test-tp test-all db-clean clean

BASE_URL ?= http://app:8080
K6 = docker run --rm --network distribuidos_chargeback-net -e BASE_URL=$(BASE_URL) -i grafana/k6 run -

build:
	docker-compose build

up:
	docker-compose up -d

down:
	docker-compose down

logs:
	docker-compose logs -f app

test: up db-clean
	@echo "=== Running k6 load test (with summary) ==="
	$(K6) < tests/load-test.js

test-demo: up db-clean
	@echo "=== Running k6 demo test (5 VUs - serialization visible) ==="
	$(K6) < tests/demo-test.js

test-tp: up db-clean
	@echo "=== Running k6 throughput comparison ==="
	$(K6) < tests/throughput-test.js

test-all: up db-clean
	@echo "=== Running ALL k6 tests ==="
	@echo ""
	@echo "########## TEST 1: LOAD TEST ##########"
	$(K6) < tests/load-test.js
	@echo ""
	@echo "########## TEST 2: DEMO TEST ##########"
	$(K6) < tests/demo-test.js
	@echo ""
	@echo "########## TEST 3: THROUGHPUT TEST ##########"
	$(K6) < tests/throughput-test.js

test-unit:
	go test ./pkg/... -v -race

db-clean:
	@echo "=== Cleaning database ==="
	docker-compose exec -T db psql -U postgres -d chargebacks -c "TRUNCATE TABLE chargebacks RESTART IDENTITY CASCADE;"
	@echo "Database cleaned."

clean:
	docker-compose down -v