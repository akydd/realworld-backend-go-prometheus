.PHONY: int-tests lint start

start:
	docker compose up -d
	until docker compose exec -T db pg_isready -U admin -d app; do sleep 1; done
	go build ./cmd/server
	./server

lint:
	golangci-lint run ./...

SPECS_DIR ?= ../realworld

int-tests:
	docker compose -f compose.test.yaml up -d
	until docker compose -f compose.test.yaml exec -T test_db pg_isready -U admin -d test-app; do sleep 1; done
	go build ./cmd/server
	./server -env .env_test & echo $$! > server.pid
	sleep 2
	HOST=http://localhost:8097 $(SPECS_DIR)/specs/api/run-api-tests-hurl.sh; \
	RESULT=$$?; \
	kill $$(cat server.pid) 2>/dev/null || true; \
	rm -f server.pid; \
	docker compose -f compose.test.yaml exec -T test_db psql -U admin -d test-app -c "TRUNCATE TABLE users CASCADE;"; \
	docker compose -f compose.test.yaml down; \
	exit $$RESULT
