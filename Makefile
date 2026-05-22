APP_NAME := social-notif

.PHONY: help run-api run-worker migrate test lint fmt tidy docker-up docker-down docker-logs build-api build-worker

help:
	@echo "Available targets:"
	@echo "  run-api       Run the HTTP API"
	@echo "  run-worker    Run the background worker"
	@echo "  migrate       Run database migrations"
	@echo "  test          Run tests"
	@echo "  lint          Run go vet"
	@echo "  fmt           Format Go code"
	@echo "  tidy          Tidy Go modules"
	@echo "  docker-up     Start local Docker stack"
	@echo "  docker-down   Stop local Docker stack"
	@echo "  docker-logs   Tail Docker logs"
	@echo "  build-api     Build API binary"
	@echo "  build-worker  Build worker binary"

run-api:
	go run ./cmd/api

run-worker:
	go run ./cmd/worker

migrate:
	go run ./cmd/migrate

test:
	go test ./...

lint:
	go vet ./...

fmt:
	go fmt ./...

tidy:
	go mod tidy

docker-up:
	docker compose up --build

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f api worker

build-api:
	go build -o bin/api ./cmd/api

build-worker:
	go build -o bin/worker ./cmd/worker
