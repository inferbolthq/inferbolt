.PHONY: help build test proto tidy run/gateway test-unit test-race test-cover test-python test-integration test-load lint fmt build-docker dev start stop restart logs clean

PROTO_DIR := proto
GEN_DIR   := gen

# Default target
help:
	@echo "Available targets:"
	@echo "  build          - Build all binaries"
	@echo "  test           - Run Go unit tests"
	@echo "  test-unit      - Run Go unit tests"
	@echo "  test-race      - Run Go unit tests under the race detector"
	@echo "  test-cover     - Run Go unit tests with a coverage summary"
	@echo "  test-python    - Run the Python worker test suite"
	@echo "  test-integration - Run integration tests"
	@echo "  test-load      - Run load tests"
	@echo "  lint           - Lint Go (golangci-lint) and Python (ruff)"
	@echo "  fmt            - Format Go and Python sources"
	@echo "  build-docker   - Build all Docker images"
	@echo "  dev            - Start development environment"
	@echo "  start          - Start all services"
	@echo "  stop           - Stop all services"
	@echo "  restart        - Restart all services"
	@echo "  logs           - Follow logs from all services"
	@echo "  clean          - Clean build artifacts and volumes"
	@echo "  proto          - Generate protobuf code"
	@echo "  tidy           - Tidy Go modules"

proto:
	protoc \
		--go_out=$(GEN_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(GEN_DIR) --go-grpc_opt=paths=source_relative \
		--proto_path=$(PROTO_DIR) \
		$(PROTO_DIR)/inferx/v1/inferx.proto

build:
	@echo "Building all binaries..."
	@mkdir -p bin
	go build -o bin/gateway ./cmd/gateway
	go build -o bin/orchestrator ./cmd/orchestrator
	go build -o bin/router ./cmd/router
	go build -o bin/collector ./cmd/collector
	go build -o bin/operator ./cmd/operator
	go build -o bin/inferbolt ./cmd/inferbolt

test: test-unit

test-unit:
	@echo "Running unit tests..."
	go test ./...

test-race:
	@echo "Running unit tests under the race detector..."
	go test -race ./...

test-cover:
	@echo "Running unit tests with coverage..."
	go test -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out | tail -1

test-python:
	@echo "Running Python tests..."
	pytest worker/tests -q

lint:
	@echo "Linting Go..."
	golangci-lint run ./...
	@echo "Linting Python..."
	ruff check worker/
	ruff format --check worker/

fmt:
	@echo "Formatting..."
	gofmt -w $$(git ls-files '*.go')
	ruff format worker/

test-integration:
	@echo "Starting integration tests..."
	@chmod +x scripts/test-integration.sh
	@./scripts/test-integration.sh

test-load:
	@echo "Running load tests..."
	@chmod +x scripts/test-load.sh
	@./scripts/test-load.sh

build-docker:
	@echo "Building Docker images..."
	docker build -t inferbolt-router:latest -f Dockerfile.router .
	docker build -t inferbolt-collector:latest -f Dockerfile.collector .
	docker build -t inferbolt-operator:latest -f Dockerfile.operator .

# Development targets
dev: stop
	@echo "Starting development environment..."
	docker-compose up -d postgres
	@echo "Waiting for PostgreSQL to be ready..."
	@sleep 10
	@echo "Development environment ready!"
	@echo "Router:    http://localhost:8082"
	@echo "Collector: http://localhost:8083"
	@echo "Database:  postgres://inferbolt:inferbolt@localhost:5432/inferbolt"

start:
	@echo "Starting all services..."
	docker-compose up -d

stop:
	@echo "Stopping all services..."
	docker-compose down

restart: stop start

logs:
	@echo "Following logs..."
	docker-compose logs -f

clean:
	@echo "Cleaning up..."
	rm -rf bin/
	docker-compose down -v
	docker image prune -f

tidy:
	go mod tidy

run/gateway:
	go run ./cmd/gateway

# Quick test commands
test-router:
	@echo "Testing router API..."
	@curl -s -X POST http://localhost:8082/v1/route \
		-H "Content-Type: application/json" \
		-d '{"prompt_tokens":512,"output_tokens":256,"concurrency":32,"gpu_profile":"a100-80gb","tenant_id":"test"}' | jq . || echo "jq not found, raw output above"

test-collector:
	@echo "Testing collector API..."
	@curl -s http://localhost:8083/health || echo "Collector not running"

test-health:
	@echo "Testing all health endpoints..."
	@curl -s http://localhost:8081/health && echo "Orchestrator: OK" || echo "Orchestrator: FAIL"
	@curl -s http://localhost:8082/health && echo "Router: OK" || echo "Router: FAIL"
	@curl -s http://localhost:8083/health && echo "Collector: OK" || echo "Collector: FAIL"
