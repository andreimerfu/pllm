.PHONY: help
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*##"; printf "\033[36m\033[0m"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: deps
deps: ## Install Go dependencies
	go mod download
	go mod tidy

.PHONY: deps-update
deps-update: ## Update Go dependencies
	go get -u ./...
	go mod tidy

.PHONY: build
build: ## Build the binary
	go build -o bin/pllm cmd/server/main.go

.PHONY: run
run: ## Run the server locally
	go run cmd/server/main.go

.PHONY: dev
dev: ## Run with hot reload (requires air)
	@which air > /dev/null || go install github.com/cosmtrek/air@latest
	air

##@ Docker

.PHONY: docker-up
docker-up: ## Start all dependencies with Docker Compose
	docker-compose up -d postgres redis prometheus grafana jaeger
	@echo "Waiting for services to be ready..."
	@sleep 5
	@echo "Services are running:"
	@echo "  PostgreSQL: localhost:5432"
	@echo "  Redis: localhost:6379"
	@echo "  Prometheus: localhost:9091"
	@echo "  Grafana: localhost:3001 (admin/admin)"
	@echo "  Jaeger: localhost:16686"

.PHONY: docker-down
docker-down: ## Stop all Docker services
	docker-compose down

.PHONY: docker-clean
docker-clean: ## Stop and remove all Docker containers and volumes
	docker-compose down -v

.PHONY: docker-logs
docker-logs: ## Show Docker logs
	docker-compose logs -f

.PHONY: docker-build
docker-build: ## Build Docker image for pllm
	docker build -t pllm:latest .

##@ Database

.PHONY: db-migrate
db-migrate: ## Run database migrations
	@echo "Migrations are auto-run on server start"

.PHONY: db-reset
db-reset: ## Reset database (drop and recreate)
	docker-compose exec postgres psql -U pllm -c "DROP DATABASE IF EXISTS pllm;"
	docker-compose exec postgres psql -U pllm -c "CREATE DATABASE pllm;"

.PHONY: db-shell
db-shell: ## Open PostgreSQL shell
	docker-compose exec postgres psql -U pllm -d pllm

.PHONY: redis-shell
redis-shell: ## Open Redis shell
	docker-compose exec redis redis-cli

##@ Testing

.PHONY: test
test: ## Run tests
	go test -v ./...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage
	go test -v -cover -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

.PHONY: test-integration
test-integration: ## Run integration tests
	go test -v -tags=integration ./...

.PHONY: lint
lint: ## Run linter
	@which golangci-lint > /dev/null || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	golangci-lint run

.PHONY: fmt
fmt: ## Format code
	go fmt ./...
	gofmt -s -w .

##@ Utilities

.PHONY: install-tools
install-tools: ## Install development tools
	go install github.com/cosmtrek/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/swaggo/swag/cmd/swag@latest

.PHONY: swagger
swagger: ## Generate Swagger documentation
	@which swag > /dev/null || go install github.com/swaggo/swag/cmd/swag@latest
	swag init -g cmd/server/main.go

.PHONY: clean
clean: ## Clean build artifacts
	rm -rf bin/ tmp/ coverage.* *.out

.PHONY: env-setup
env-setup: ## Create .env file from example
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo ".env file created. Please update it with your API keys."; \
	else \
		echo ".env file already exists."; \
	fi

##@ Quick Start

.PHONY: setup
setup: env-setup deps install-tools docker-up ## Complete setup for development
	@echo "Setup complete! Now run 'make dev' to start the server with hot reload"

.PHONY: start
start: docker-up run ## Start dependencies and run server

.PHONY: stop
stop: docker-down ## Stop everything

##@ API Testing

.PHONY: test-health
test-health: ## Test health endpoint
	curl -s http://localhost:8080/health | jq

.PHONY: test-register
test-register: ## Test user registration
	curl -X POST http://localhost:8080/v1/register \
		-H "Content-Type: application/json" \
		-d '{"email":"test@example.com","username":"testuser","password":"Test123!@#"}' | jq

.PHONY: test-login
test-login: ## Test user login
	curl -X POST http://localhost:8080/v1/login \
		-H "Content-Type: application/json" \
		-d '{"email":"test@example.com","password":"Test123!@#"}' | jq

.PHONY: test-chat
test-chat: ## Test chat completion (requires valid token)
	@echo "First login to get a token, then use it in the Authorization header"
	curl -X POST http://localhost:8080/v1/chat/completions \
		-H "Content-Type: application/json" \
		-H "Authorization: Bearer YOUR_TOKEN_HERE" \
		-d '{"model":"gpt-3.5-turbo","messages":[{"role":"user","content":"Hello!"}]}' | jq