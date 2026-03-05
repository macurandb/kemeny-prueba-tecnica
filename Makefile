.PHONY: check lint test security build format docker-up docker-down docker-build docker-logs

# Run all checks (use before every commit)
check: lint test security build
	@echo "All checks passed"

# Linting
lint: lint-backend lint-frontend

lint-backend:
	@echo "==> Linting backend (golangci-lint)..."
	cd backend && golangci-lint run ./...

lint-frontend:
	@echo "==> Linting frontend (ESLint)..."
	cd frontend && npm run lint
	@echo "==> Checking frontend format (Prettier)..."
	cd frontend && npm run format:check

# Tests
test: test-backend test-frontend

test-backend:
	@echo "==> Running backend tests..."
	cd backend && go test ./... -race -v

test-frontend:
	@echo "==> Running frontend tests..."
	cd frontend && npm test -- --passWithNoTests

# Security checks
security:
	@echo "==> Running security checks (gosec via golangci-lint)..."
	cd backend && golangci-lint run --enable gosec ./...

# Build verification
build: build-backend build-frontend

build-backend:
	@echo "==> Building backend..."
	cd backend && go build ./...

build-frontend:
	@echo "==> Building frontend..."
	cd frontend && npm run build

# Format code (auto-fix)
format:
	@echo "==> Formatting frontend (Prettier)..."
	cd frontend && npm run format

# Setup development environment
setup:
	@echo "==> Setting up git hooks..."
	git config core.hooksPath .githooks
	@echo "==> Installing frontend dependencies..."
	cd frontend && npm install
	@echo "==> Verifying Go tools..."
	@which golangci-lint > /dev/null 2>&1 || echo "WARNING: golangci-lint not found. Install: https://golangci-lint.run/welcome/install/"
	@echo "Setup complete"

# Docker
docker-up:
	docker-compose up --build

docker-down:
	docker-compose down

docker-build:
	docker-compose build

docker-logs:
	docker-compose logs -f
