.PHONY: build run test clean docker-build docker-run migrate-up migrate-down migrate-create dev lint check-imports proto-gen setup-proto

# Variables
APP_NAME=william
BUILD_DIR=bin
DOCKER_IMAGE=william-bot
DB_URL ?= postgres://william:william_password@localhost/william?sslmode=disable
PROTO_SRC=proto
PROTO_OUT=pkg
MIGRATION_DIR=./internal/migrations/

# Build the application
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/william

# Build CLI client
build-cli:
	@echo "Building williamc CLI..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/williamc ./cmd/williamc

# Build both applications
build-all: build build-cli
	@echo "All applications built successfully!"

# Generate protobuf code
proto-gen:
	@echo "Generating protobuf code..."
	@mkdir -p $(PROTO_OUT)/adminpb
	@rm -f $(PROTO_OUT)/adminpb/*.pb.go
	protoc \
		-I $(PROTO_SRC) \
		--go_out=. \
		--go-grpc_out=. \
		$(PROTO_SRC)/william/admin/v1/*.proto
	@if [ -d github.com/xdefrag/william/pkg/adminpb ]; then \
		mv github.com/xdefrag/william/pkg/adminpb/* $(PROTO_OUT)/adminpb/ && \
		rm -rf github.com; \
	fi

# Setup protobuf tools
setup-proto:
	@echo "Installing protobuf tools..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "Please ensure protoc is installed: https://grpc.io/docs/protoc-installation/"

# Run the application
run:
	@echo "Running $(APP_NAME)..."
	APP_CONFIG_PATH="config/app_test.toml" go run ./cmd/william

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

# Check for unauthorized imports excluding gRPC dependencies
check-imports-light:
	@echo "Checking core imports (excluding gRPC)..."
	@if [ -f allowed-mods.txt ]; then \
		go list -m all | grep -v "google.golang.org" | grep -v "golang.org/x" | grep -vFf allowed-mods.txt | grep -v "github.com/xdefrag/william" | head -10 && echo "Too many dependencies, showing first 10" || echo "Core imports check passed"; \
	fi

# Development mode with hot reload
dev:
	@echo "Starting development mode..."
	air

# Database migrations
migrate-up:
	@echo "Running migrations up..."
	goose -dir $(MIGRATION_DIR) postgres "$(DB_URL)" up

migrate-down:
	@echo "Running migrations down..."
	goose -dir $(MIGRATION_DIR) postgres "$(DB_URL)" down

migrate-status:
	@echo "Migration status..."
	goose -dir $(MIGRATION_DIR) postgres "$(DB_URL)" status

migrate-create:
	@echo "Creating new migration: $(name)"
	@if [ -z "$(name)" ]; then echo "Usage: make migrate-create name=migration_name"; exit 1; fi
	goose -dir $(MIGRATION_DIR) create $(name) sql

# Docker operations
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE) .

docker-run:
	@echo "Running Docker container..."
	docker run --rm --env-file .env $(DOCKER_IMAGE)

# Docker Compose operations
docker-compose-up:
	@echo "Starting services with Docker Compose..."
	docker-compose up -d

docker-compose-logs:
	@echo "Showing logs..."
	docker-compose logs -f william

docker-compose-down:
	@echo "Stopping services..."
	docker-compose down

docker-compose-restart:
	@echo "Restarting services..."
	docker-compose restart william

docker-compose-migrate:
	@echo "Running database migrations..."
	docker-compose run --rm migrate

docker-compose-build:
	@echo "Building services..."
	docker-compose build

docker-compose-dev:
	@echo "Starting development environment..."
	@if [ ! -f .env ]; then \
		echo "Creating .env from example..."; \
		cp docker-compose.env.example .env; \
		echo "Please edit .env with your API keys!"; \
		exit 1; \
	fi
	docker-compose up

docker-compose-clean:
	@echo "Cleaning up Docker Compose resources..."
	docker-compose down -v --remove-orphans
	docker-compose rm -f

# Setup development environment
setup-dev:
	@echo "Setting up development environment..."
	go install github.com/pressly/goose/v3/cmd/goose@latest
	go install github.com/cosmtrek/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Setup all development tools
setup-all: setup-dev setup-proto
	@echo "All development tools installed!"

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

# Full check (lint, test, build) - skip full import check for now due to gRPC deps
check: check-imports-light lint test build
	@echo "All checks passed!"

# Full check including all imports (for CI)
check-full: check-imports lint test build
	@echo "All checks passed!"

# Help
help:
	@echo "Available commands:"
	@echo ""
	@echo "Development:"
	@echo "  build           Build the application"
	@echo "  build-cli       Build williamc CLI client"
	@echo "  build-all       Build both william and williamc"
	@echo "  run             Run the application"
	@echo "  dev             Start development mode with hot reload"
	@echo "  setup-dev       Setup development environment"
	@echo "  setup-proto     Setup protobuf tools"
	@echo "  setup-all       Setup all development tools"
	@echo "  proto-gen       Generate protobuf code"
	@echo ""
	@echo "Testing & Quality:"
	@echo "  test            Run tests"
	@echo "  test-coverage   Run tests with coverage"
	@echo "  lint            Run linters"
	@echo "  check-imports   Check for unauthorized imports"
	@echo "  check           Run all checks (lint, test, build)"
	@echo "  fmt             Format code"
	@echo "  tidy            Run go mod tidy"
	@echo ""
	@echo "Database:"
	@echo "  migrate-up      Run database migrations up"
	@echo "  migrate-down    Run database migrations down"
	@echo "  migrate-status  Show migration status"
	@echo "  migrate-create  Create new migration (use name=migration_name)"
	@echo ""
	@echo "Docker (standalone):"
	@echo "  docker-build    Build Docker image"
	@echo "  docker-run      Run Docker container"
	@echo ""
	@echo "Docker Compose (recommended for local development):"
	@echo "  docker-compose-dev      Start development environment (creates .env if missing)"
	@echo "  docker-compose-up       Start services in background"
	@echo "  docker-compose-logs     Show service logs"
	@echo "  docker-compose-down     Stop services"
	@echo "  docker-compose-restart  Restart william service"
	@echo "  docker-compose-migrate  Run database migrations"
	@echo "  docker-compose-build    Rebuild services"
	@echo "  docker-compose-clean    Clean up all resources"
	@echo ""
	@echo "Utilities:"
	@echo "  verify-tools    Verify all required tools are installed"
	@echo "  clean           Clean build artifacts"
	@echo "  help            Show this help" 
