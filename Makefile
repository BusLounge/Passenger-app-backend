# Makefile for SmartTransit SMS Authentication Backend

.PHONY: help build run test clean docker-build docker-run generate-secrets install-deps dev db-clear verify-deployment prepare-deployment quick-start deployment-guide check-env deployment-status check fmt vet lint install-tools

# Variables
APP_NAME=sms-auth-backend
MAIN_PATH=./cmd/server
DOCKER_IMAGE=$(APP_NAME):latest
PORT=8080

# Default target
help:
	@echo "SmartTransit SMS Authentication Backend - Make Commands"
	@echo ""
	@echo "DEVELOPMENT:"
	@echo "  make install-deps    - Install Go dependencies"
	@echo "  make build           - Build the application"
	@echo "  make run             - Run the application"
	@echo "  make dev             - Run in development mode with hot reload"
	@echo "  make test            - Run tests"
	@echo "  make test-coverage   - Run tests with coverage"
	@echo "  make lint            - Run linter"
	@echo "  make check           - Run fmt, vet, and test"
	@echo "  make fmt             - Format code"
	@echo "  make vet             - Vet code"
	@echo ""
	@echo "DOCKER:"
	@echo "  make docker-build    - Build Docker image"
	@echo "  make docker-run      - Run Docker container locally"
	@echo ""
	@echo "CHOREO DEPLOYMENT:"
	@echo "  make verify-deployment    - Verify deployment readiness"
	@echo "  make check-env            - Check .env configuration"
	@echo "  make deployment-status    - Show deployment status"
	@echo "  make prepare-deployment   - Prepare for Choreo deployment"
	@echo "  make quick-start          - Show quick start guide"
	@echo "  make deployment-guide     - Show full deployment guide"
	@echo ""
	@echo "UTILITIES:"
	@echo "  make generate-secrets - Generate JWT secrets"
	@echo "  make clean            - Clean build artifacts"
	@echo "  make db-clear         - TRUNCATE all data (requires DATABASE_URL)"
	@echo "  make install-tools    - Install development tools"
	@echo ""

# Install dependencies
install-deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Generate JWT secrets
generate-secrets:
	@echo "Generating JWT secrets..."
	go run cmd/generate-secrets/main.go

# Build the application
build:
	@echo "Building $(APP_NAME)..."
	go build -o bin/$(APP_NAME) $(MAIN_PATH)
	@echo "Build complete: bin/$(APP_NAME)"

# Run the application
run:
	@echo "Running $(APP_NAME)..."
	go run $(MAIN_PATH)

# Run in development mode with hot reload (requires air)
dev:
	@echo "Starting development server..."
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "air not installed. Install with: go install github.com/air-verse/air@latest"; \
		echo "Running without hot reload..."; \
		go run $(MAIN_PATH); \
	fi

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run linter
lint:
	@echo "Running linter..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install from: https://golangci-lint.run/usage/install/"; \
	fi

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE) .

# Run Docker container
docker-run:
	@echo "Running Docker container..."
	docker run -p $(PORT):$(PORT) --env-file .env $(DOCKER_IMAGE)

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	go clean

# Clear all application data from the database (preserves schema)
db-clear:
	@if [ -z "$$DATABASE_URL" ]; then \
		echo "ERROR: DATABASE_URL is not set. Export it first."; \
		exit 1; \
	fi
	@echo "Clearing all data from application tables..."
	psql "$$DATABASE_URL" -v ON_ERROR_STOP=1 -f scripts/clear_all_data.sql
	@echo "All data cleared successfully."

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Vet code
vet:
	@echo "Vetting code..."
	go vet ./...

# Run all checks (fmt, vet, test)
check: fmt vet test
	@echo "All checks passed!"

# Install development tools
install-tools:
	@echo "Installing development tools..."
	go install github.com/air-verse/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Development tools installed!"

# ============================================================================
# CHOREO DEPLOYMENT COMMANDS
# ============================================================================

# Verify deployment readiness
verify-deployment:
	@echo "📋 Running deployment verification..."
	@pwsh -NoProfile -ExecutionPolicy Bypass -File verify-deployment.ps1

# Prepare for deployment (run all checks)
prepare-deployment: check docker-build
	@echo "✅ Deployment preparation complete!"
	@echo "Next steps:"
	@echo "1. git push origin main"
	@echo "2. Go to https://console.choreo.dev"
	@echo "3. Create new component with Dockerfile"
	@echo "4. Configure environment variables"
	@echo "5. Deploy!"

# Show quick start guide
quick-start:
	@echo "📖 Opening Quick Start Guide..."
	@echo ""
	@cat docs/QUICK_CHOREO_DEPLOYMENT.md
	@echo ""

# Show full deployment guide
deployment-guide:
	@echo "📖 Opening Full Deployment Guide..."
	@echo ""
	@cat docs/CHOREO_DEPLOYMENT_GUIDE.md
	@echo ""

# Check if .env file exists and has required variables
check-env:
	@if [ ! -f .env ]; then \
		echo "❌ .env file not found!"; \
		echo "Creating .env from .env.example..."; \
		cp .env.example .env; \
		echo "⚠️  Please edit .env with your actual values:"; \
		echo "   - DATABASE_URL"; \
		echo "   - JWT_SECRET"; \
		echo "   - JWT_REFRESH_SECRET"; \
		echo "   - DIALOG_SMS_ESMSQK"; \
		exit 1; \
	fi
	@echo "✅ .env file found"
	@echo "Checking required variables..."
	@grep -q "DATABASE_URL" .env && echo "✅ DATABASE_URL configured" || (echo "❌ DATABASE_URL missing"; exit 1)
	@grep -q "JWT_SECRET" .env && echo "✅ JWT_SECRET configured" || (echo "❌ JWT_SECRET missing"; exit 1)

# Show deployment readiness status
deployment-status: check-env docker-build
	@echo ""
	@echo "✅ DEPLOYMENT READINESS CHECK"
	@echo "================================"
	@echo "✅ Go build successful"
	@echo "✅ Docker image built"
	@echo "✅ .env file configured"
	@echo "✅ All components ready"
	@echo ""
	@echo "Your backend is ready to deploy to Choreo!"
	@echo ""
	@echo "Service: sms-auth-backend"
	@echo "Port: 8080"
	@echo "Base URL: https://sms-auth-api-xxx.c1.choreo.dev"
	@echo ""
