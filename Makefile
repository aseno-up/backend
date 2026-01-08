.PHONY: build run dev test lint clean docker-up docker-down migrate

# Binary name
BINARY_NAME=api
BUILD_DIR=./bin

# Go commands
GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt

# Build the binary
build:
	@echo "Building..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/api/main.go

# Run the application
run: build
	@echo "Running..."
	$(BUILD_DIR)/$(BINARY_NAME)

# Run in development mode with hot reload (requires air)
dev:
	@echo "Running in development mode..."
	$(GORUN) ./cmd/api/main.go

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -cover ./...

# Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Lint the code
lint:
	@echo "Linting..."
	$(GOFMT) -s -w .
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping..."; \
	fi

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Docker compose up (PostgreSQL + Redis)
docker-up:
	@echo "Starting Docker services..."
	docker-compose up -d

# Docker compose down
docker-down:
	@echo "Stopping Docker services..."
	docker-compose down

# Start pgAdmin (Database UI)
pgadmin:
	@echo "Starting pgAdmin on http://localhost:5050..."
	docker-compose --profile tools up -d pgadmin

# Stop pgAdmin
pgadmin-down:
	@echo "Stopping pgAdmin..."
	docker-compose --profile tools stop pgadmin

# Start RabbitMQ
rabbitmq:
	@echo "Starting RabbitMQ on amqp://localhost:5672..."
	@echo "Management UI: http://localhost:15672 (aseno/aseno_password)"
	docker-compose up -d rabbitmq

# Stop RabbitMQ
rabbitmq-down:
	@echo "Stopping RabbitMQ..."
	docker-compose stop rabbitmq

# RabbitMQ logs
rabbitmq-logs:
	docker-compose logs -f rabbitmq

# Create new migration (usage: make migration name=create_users_table)
migration:
	@echo "Creating migration..."
	@mkdir -p migrations
	@touch migrations/$$(date +%Y%m%d%H%M%S)_$(name).sql
	@echo "Created: migrations/$$(date +%Y%m%d%H%M%S)_$(name).sql"

# Show help
help:
	@echo "Available commands:"
	@echo "  make build          - Build the binary"
	@echo "  make run            - Build and run the application"
	@echo "  make dev            - Run in development mode"
	@echo "  make test           - Run tests"
	@echo "  make test-coverage  - Run tests with coverage report"
	@echo "  make lint           - Lint the code"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make deps           - Download dependencies"
	@echo "  make docker-up      - Start Docker services (Postgres, Redis, RabbitMQ)"
	@echo "  make docker-down    - Stop Docker services"
	@echo "  make pgadmin        - Start pgAdmin on http://localhost:5555"
	@echo "  make pgadmin-down   - Stop pgAdmin"
	@echo "  make rabbitmq       - Start RabbitMQ on http://localhost:15672"
	@echo "  make rabbitmq-down  - Stop RabbitMQ"
	@echo "  make rabbitmq-logs  - Show RabbitMQ logs"
	@echo "  make migration      - Create new migration file"
	@echo "  make help           - Show this help"
