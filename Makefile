.PHONY: build run test clean docker-build docker-run migrate-up migrate-down migrate-create dev lint check-imports

# Variables
APP_NAME=william
BUILD_DIR=bin
DOCKER_IMAGE=william-bot
DB_URL ?= postgres://localhost/william?sslmode=disable

# Build the application
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/william

# Run the application
run:
	@echo "Running $(APP_NAME)..."
	go run ./cmd/william

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html

# Run linters
lint:
	@echo "Running linters..."
	golangci-lint run

# Check for unauthorized imports
check-imports:
	@echo "Checking imports..."
	@if [ -f allowed-mods.txt ]; then \
		go list -m all | grep -vFf allowed-mods.txt | grep -v "github.com/xdefrag/william" && exit 1 || true; \
	fi

# Development mode with hot reload
dev:
	@echo "Starting development mode..."
	air

# Database migrations
migrate-up:
	@echo "Running migrations up..."
	goose -dir migrations postgres "$(DB_URL)" up

migrate-down:
	@echo "Running migrations down..."
	goose -dir migrations postgres "$(DB_URL)" down

migrate-status:
	@echo "Migration status..."
	goose -dir migrations postgres "$(DB_URL)" status

migrate-create:
	@echo "Creating new migration: $(name)"
	@if [ -z "$(name)" ]; then echo "Usage: make migrate-create name=migration_name"; exit 1; fi
	goose -dir migrations create $(name) sql

# Docker operations
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE) .

docker-run:
	@echo "Running Docker container..."
	docker run --rm --env-file .env $(DOCKER_IMAGE)

# Setup development environment
setup-dev:
	@echo "Setting up development environment..."
	go install github.com/pressly/goose/v3/cmd/goose@latest
	go install github.com/cosmtrek/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Verify all required tools are installed
verify-tools:
	@echo "Verifying tools..."
	@command -v goose >/dev/null 2>&1 || { echo "goose is not installed"; exit 1; }
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint is not installed"; exit 1; }
	@echo "All tools are installed"

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Run go mod tidy
tidy:
	@echo "Running go mod tidy..."
	go mod tidy

# Full check (lint, test, build)
check: check-imports lint test build
	@echo "All checks passed!"

# Help
help:
	@echo "Available commands:"
	@echo "  build           Build the application"
	@echo "  run             Run the application"
	@echo "  test            Run tests"
	@echo "  test-coverage   Run tests with coverage"
	@echo "  clean           Clean build artifacts"
	@echo "  lint            Run linters"
	@echo "  check-imports   Check for unauthorized imports"
	@echo "  dev             Start development mode with hot reload"
	@echo "  migrate-up      Run database migrations up"
	@echo "  migrate-down    Run database migrations down"
	@echo "  migrate-status  Show migration status"
	@echo "  migrate-create  Create new migration (use name=migration_name)"
	@echo "  docker-build    Build Docker image"
	@echo "  docker-run      Run Docker container"
	@echo "  setup-dev       Setup development environment"
	@echo "  verify-tools    Verify all required tools are installed"
	@echo "  fmt             Format code"
	@echo "  tidy            Run go mod tidy"
	@echo "  check           Run all checks (lint, test, build)"
	@echo "  help            Show this help" 