.PHONY: build test run clean deps migrate migrate-up migrate-down migrate-create migrate-version migrate-force install-migrate telegram-webhook-set telegram-webhook-delete telegram-webhook-info db-create db-create-test db-drop db-drop-test db-reset db-setup db-test fmt lint lint-fix lint-unused lint-all check help

# Load environment variables from .env if it exists
ifneq (,$(wildcard .env))
    include .env
    export
endif

# Default target - show help
.DEFAULT_GOAL := help

# Help command - shows available targets
help:
	@echo "🤖 AI Trading Bot - Available Commands"
	@echo ""
	@echo "📦 Setup & Installation:"
	@echo "  make setup              - Full development setup (DB + migrations + .env)"
	@echo "  make deps               - Download Go dependencies"
	@echo "  make install-migrate    - Install golang-migrate CLI tool"
	@echo ""
	@echo "🗄️  Database Management:"
	@echo "  make db-test            - Test database connection"
	@echo "  make db-create          - Create database"
	@echo "  make db-setup           - Create database + run migrations"
	@echo "  make db-reset           - Drop, recreate and migrate database (⚠️  deletes all data!)"
	@echo "  make db-drop            - Drop database (⚠️  requires confirmation)"
	@echo ""
	@echo "🔄 Database Migrations:"
	@echo "  make migrate-up         - Run all pending migrations"
	@echo "  make migrate-down       - Rollback last migration"
	@echo "  make migrate-version    - Show current migration version"
	@echo "  make migrate-create     - Create new migration files"
	@echo "  make migrate-force      - Force migration version (⚠️  use carefully!)"
	@echo ""
	@echo "🏗️  Build & Run:"
	@echo "  make build              - Build bot binary"
	@echo "  make run                - Run trading bot"
	@echo "  make paper              - Run in paper trading mode"
	@echo ""
	@echo "🧪 Testing:"
	@echo "  make test               - Run unit tests"
	@echo "  make test-db            - Run all tests with PostgreSQL"
	@echo "  make test-coverage      - Generate coverage report"
	@echo "  make test-pkg           - Test specific package"
	@echo ""
	@echo "🔍 Code Quality:"
	@echo "  make fmt                - Format code with gofmt"
	@echo "  make check              - Quick checks with native Go tools (recommended)"
	@echo "  make lint-unused        - Check for unused code with go vet"
	@echo "  make lint-all           - Full static analysis with native tools"
	@echo "  make lint               - Run golangci-lint v2 (comprehensive linting)"
	@echo "  make lint-fix           - Format code with golangci-lint v2 (gofmt + goimports)"
	@echo "  make lint-new           - Lint only new/changed code (fast for CI)"
	@echo ""
	@echo "🧹 Cleanup:"
	@echo "  make clean              - Remove build artifacts"
	@echo ""
	@echo "⚙️  Configuration:"
	@echo "  DB_USER=user DB_NAME=mydb make db-create    - Custom PostgreSQL settings"
	@echo "  DB_URL=postgres://... make migrate-up        - Custom database URL"
	@echo ""
	@echo "📚 Documentation: docs/MIGRATIONS.md"

# Build binary
build:
	@echo "Building binary..."
	@mkdir -p bin
	go build -o bin/bot cmd/bot/main.go

# Run unit tests only (no database)
test:
	@echo "🧪 Running unit tests..."
	go test -v -short -race ./internal/... ./pkg/...

# Run all tests with PostgreSQL database (integration tests)
# Uses existing PostgreSQL instance from .env configuration
test-db:
	@echo "🐘 Preparing test database..."
	@echo "   Using: $(DB_USER)@$(DB_HOST):$(DB_PORT)"
	@echo ""
	@echo "📦 Creating test database '$(DB_TEST_NAME)'..."
	@PGPASSWORD='$(DB_PASSWORD)' psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d postgres -tc \
		"SELECT 1 FROM pg_database WHERE datname = '$(DB_TEST_NAME)'" | grep -q 1 || \
		PGPASSWORD='$(DB_PASSWORD)' psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d postgres -c "CREATE DATABASE $(DB_TEST_NAME);"
	@echo "✅ Test database created!"
	@echo ""
	@echo "⬆️  Running migrations on test database..."
	@migrate -path=./migrations -database "postgres://$(DB_USER)$(if $(DB_PASSWORD),:$(DB_PASSWORD),)@$(DB_HOST):$(DB_PORT)/$(DB_TEST_NAME)?sslmode=disable" up > /dev/null 2>&1
	@echo "✅ Migrations applied!"
	@echo ""
	@echo "🧪 Running all tests..."
	@TEST_DATABASE_URL="host=$(DB_HOST) port=$(DB_PORT) user=$(DB_USER) password=$(DB_PASSWORD) dbname=$(DB_TEST_NAME) sslmode=disable" \
		go test -v -race ./... || (echo "❌ Tests failed!"; PGPASSWORD='$(DB_PASSWORD)' psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d postgres -c "DROP DATABASE IF EXISTS $(DB_TEST_NAME);" 2>/dev/null; exit 1)
	@echo ""
	@echo "🧹 Cleaning up test database..."
	@PGPASSWORD='$(DB_PASSWORD)' psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d postgres -c "DROP DATABASE IF EXISTS $(DB_TEST_NAME);" 2>/dev/null || true
	@echo "✅ All tests passed!"

# Run with coverage
test-coverage:
	@echo "📊 Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out
	@go tool cover -html=coverage.out -o coverage.html
	@echo ""
	@echo "✅ Coverage report: coverage.html"

# Run specific package tests
test-pkg:
	@read -p "Package path (e.g., ./internal/risk): " pkg; \
	go test -v -race $$pkg

# Run the bot
run:
	@mkdir -p logs
	go run cmd/bot/main.go

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf logs/
	rm -f coverage.out coverage.html

# Download dependencies
deps:
	go mod download
	go mod tidy

# PostgreSQL configuration (loaded from .env, can be overridden)
DB_HOST ?= localhost
DB_PORT ?= 5432
DB_USER ?= $(USER)
DB_PASSWORD ?=
DB_NAME ?= trader
DB_TEST_NAME ?= trader_test

# Database URL for migrations (constructed from .env variables)
DB_URL ?= postgres://$(DB_USER)$(if $(DB_PASSWORD),:$(DB_PASSWORD),)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable

# Test database connection
db-test:
	@echo "🧪 Testing database connection..."
	@echo "   Host: $(DB_HOST)"
	@echo "   Port: $(DB_PORT)"
	@echo "   User: $(DB_USER)"
	@echo "   Password set: $$(if [ -n '$(DB_PASSWORD)' ]; then echo 'YES (length: $$(echo '$(DB_PASSWORD)' | wc -c | xargs))'; else echo 'NO'; fi)"
	@echo ""
	@echo "Attempting connection..."
	@PGPASSWORD='$(DB_PASSWORD)' psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d postgres -c "SELECT current_user, version();" && echo "✅ Connection successful!" || echo "❌ Connection failed!"

# Create database
db-create:
	@echo "📦 Creating database '$(DB_NAME)'..."
	@echo "🔍 Debug config:"
	@echo "   Host: $(DB_HOST)"
	@echo "   Port: $(DB_PORT)"
	@echo "   User: $(DB_USER)"
	@echo "   Password length: $$(echo '$(DB_PASSWORD)' | wc -c)"
	@echo "   Database: $(DB_NAME)"
	@echo ""
	@PGPASSWORD='$(DB_PASSWORD)' psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d postgres -tc \
		"SELECT 1 FROM pg_database WHERE datname = '$(DB_NAME)'" | grep -q 1 || \
		PGPASSWORD='$(DB_PASSWORD)' psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d postgres -c "CREATE DATABASE $(DB_NAME);"
	@echo "✅ Database '$(DB_NAME)' ready!"

# Create test database
db-create-test:
	@echo "📦 Creating test database '$(DB_TEST_NAME)'..."
	@PGPASSWORD='$(DB_PASSWORD)' psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d postgres -tc \
		"SELECT 1 FROM pg_database WHERE datname = '$(DB_TEST_NAME)'" | grep -q 1 || \
		PGPASSWORD='$(DB_PASSWORD)' psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d postgres -c "CREATE DATABASE $(DB_TEST_NAME);"
	@echo "✅ Test database '$(DB_TEST_NAME)' ready!"

# Drop database (careful!)
db-drop:
	@echo "⚠️  Dropping database '$(DB_NAME)'..."
	@read -p "Are you sure? This will delete all data! (yes/NO): " confirm; \
	if [ "$$confirm" = "yes" ]; then \
		PGPASSWORD='$(DB_PASSWORD)' psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d postgres -c "DROP DATABASE IF EXISTS $(DB_NAME);"; \
		echo "✅ Database '$(DB_NAME)' dropped"; \
	else \
		echo "❌ Aborted"; \
	fi

# Drop test database
db-drop-test:
	@echo "🗑️  Dropping test database '$(DB_TEST_NAME)'..."
	@PGPASSWORD='$(DB_PASSWORD)' psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d postgres -c "DROP DATABASE IF EXISTS $(DB_TEST_NAME);" 2>/dev/null || true
	@echo "✅ Test database dropped"

# Reset database (drop + create + migrate)
db-reset: db-drop db-create migrate-up
	@echo "✅ Database reset complete!"

# Full database setup (create + migrate)
db-setup: db-create migrate-up
	@echo "✅ Database setup complete!"

# Install migrate CLI tool (official golang-migrate CLI)
install-migrate:
	@echo "📦 Installing golang-migrate CLI..."
	@go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@echo "✅ Done! Run 'migrate -version' to verify"

# Run all pending migrations using official CLI
migrate-up:
	@echo "⬆️  Running migrations up..."
	@migrate -path=./migrations -database "$(DB_URL)" up
	@echo "✅ Migrations completed"

# Rollback last migration
migrate-down:
	@echo "⬇️  Rolling back last migration..."
	@migrate -path=./migrations -database "$(DB_URL)" down 1
	@echo "✅ Rollback completed"

# Show current migration version
migrate-version:
	@echo "📊 Current migration version:"
	@migrate -path=./migrations -database "$(DB_URL)" version

# Force migration version (use carefully!)
migrate-force:
	@read -p "⚠️  Force version to (number): " version; \
	migrate -path=./migrations -database "$(DB_URL)" force $$version

# Create new migration file
migrate-create:
	@read -p "Migration name (e.g., add_user_preferences): " name; \
	migrate create -ext sql -dir ./migrations -seq $$name

# Legacy alias (runs migrate-up)
migrate: migrate-up

# Setup development environment (full setup)
setup: db-setup
	@echo "🔧 Setting up development environment..."
	@mkdir -p logs
	@mkdir -p bin
	@if [ ! -f .env ]; then \
		cp env.example .env 2>/dev/null || cp .env.example .env 2>/dev/null || echo "# Add your config here" > .env; \
		echo "📝 Created .env file - please edit with your API keys"; \
	else \
		echo "✅ .env file already exists"; \
	fi
	@echo ""
	@echo "✅ Setup complete! Next steps:"
	@echo "   1. Edit .env with your API keys"
	@echo "   2. Run: make run"

# Format code
fmt:
	go fmt ./...

# Lint code with golangci-lint v2
lint:
	@echo "🔍 Running golangci-lint v2..."
	@golangci-lint run ./...

# Format code with golangci-lint v2 formatters (replaces lint-fix in v2)
lint-fix:
	@echo "🔧 Formatting code with golangci-lint v2..."
	@golangci-lint fmt ./...

# Format code (alias for lint-fix)
fmt-v2:
	@echo "🔧 Formatting code with golangci-lint v2..."
	@golangci-lint fmt ./...

# Lint only new/changed code (fast, great for CI/pre-commit)
lint-new:
	@echo "🆕 Running golangci-lint v2 on new code only..."
	@golangci-lint run --new ./...

# Check for unused variables and code using native Go tools
lint-unused:
	@echo "🔍 Checking for unused variables, functions, and types..."
	@echo "Running go vet..."
	@go vet ./...
	@echo "✅ go vet passed!"
	@echo ""
	@echo "Checking for unused imports and variables..."
	@goimports -l . | grep -v "^$$" || echo "✅ No unused imports found"

# Full static analysis with native Go tools
lint-all:
	@echo "🔍 Running full static analysis with native tools..."
	@echo ""
	@echo "1️⃣  Running go vet..."
	@go vet ./...
	@echo "✅ go vet passed!"
	@echo ""
	@echo "2️⃣  Checking code formatting..."
	@test -z "$$(gofmt -l . | grep -v vendor)" || (echo "❌ Code not formatted. Run 'make fmt'" && gofmt -l . && exit 1)
	@echo "✅ Code is formatted!"
	@echo ""
	@echo "3️⃣  Building all packages..."
	@go build ./...
	@echo "✅ Build successful!"
	@echo ""
	@echo "✅ All checks passed!"

# Check code without golangci-lint (uses only Go native tools)
check:
	@echo "🔍 Running native Go checks..."
	@go vet ./... && echo "✅ go vet passed" || exit 1
	@go build ./... && echo "✅ build passed" || exit 1
	@test -z "$$(gofmt -l . | grep -v vendor)" && echo "✅ formatting passed" || (echo "❌ needs formatting" && exit 1)

# Run in paper trading mode
paper:
	@echo "Starting paper trading mode..."
	@mkdir -p logs
	MODE=paper go run cmd/bot/main.go

# Telegram webhook commands
telegram-webhook:
	@echo "🔗 Setting Telegram webhook..."
	@if [ -z "$(TELEGRAM_BOT_TOKEN)" ]; then \
		echo "❌ Error: TELEGRAM_BOT_TOKEN not set"; \
		echo "Usage: TELEGRAM_BOT_TOKEN=your_token WEBHOOK_URL=https://your-domain.com make telegram-webhook-set"; \
		exit 1; \
	fi; \
	if [ -z "$(WEBHOOK_URL)" ]; then \
		echo "❌ Error: WEBHOOK_URL not set"; \
		echo "Usage: TELEGRAM_BOT_TOKEN=your_token WEBHOOK_URL=https://your-domain.com make telegram-webhook-set"; \
		exit 1; \
	fi; \
	curl -X POST "https://api.telegram.org/bot$(TELEGRAM_BOT_TOKEN)/setWebhook" \
		-H "Content-Type: application/json" \
		-d '{"url":"$(WEBHOOK_URL)/telegram/webhook","drop_pending_updates":true}' | jq; \
	echo "✅ Webhook set to: $(WEBHOOK_URL)/telegram/webhook"

# Docker commands
docker-build:
	docker build -t trader-bot .

docker-run:
	docker-compose up -d

docker-logs:
	docker-compose logs -f bot

docker-stop:
	docker-compose down

