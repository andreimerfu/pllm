.PHONY: help
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*##"; printf "\033[36m\033[0m"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

##@ Helm

.PHONY: helm-validate
helm-validate: ## Validate Helm chart
	@./scripts/helm-validate.sh

.PHONY: helm-package
helm-package: ## Package Helm chart
	@cd deploy/helm && helm package pllm

.PHONY: helm-install
helm-install: ## Install Helm chart locally (requires k8s cluster)
	@helm upgrade --install pllm deploy/helm/pllm --create-namespace --namespace pllm-system

.PHONY: helm-uninstall
helm-uninstall: ## Uninstall Helm chart
	@helm uninstall pllm --namespace pllm-system

.PHONY: deps
deps: ## Install Go dependencies
	go mod download
	go mod tidy

.PHONY: deps-update
deps-update: ## Update Go dependencies
	go get -u ./...
	go mod tidy

.PHONY: build
build: ui-build docs-build ## Build the binary with embedded UI and docs
	go build -o bin/pllm cmd/server/main.go

.PHONY: web-build
web-build: ## Build frontend assets
	@echo "Building frontend..."
	cd web && npm run build

.PHONY: ui-build
ui-build: web-build ## Copy built frontend to internal/ui/dist for embedding
	@mkdir -p internal/ui/dist
	@cp -r web/dist/* internal/ui/dist/
	@echo "✅ Frontend copied to internal/ui/dist/"

.PHONY: build-worker
build-worker: ## Build the worker binary for background processing
	go build -o bin/pllm-worker cmd/worker/main.go

.PHONY: build-all
build-all: build build-worker ## Build both server and worker binaries

.PHONY: run
run: ## Run the server locally
	go run cmd/server/main.go

.PHONY: run-worker
run-worker: ## Run the worker locally
	go run cmd/worker/main.go

.PHONY: dev
dev: ## Run with hot reload (requires air)
	@which air > /dev/null || go install github.com/cosmtrek/air@latest
	air

.PHONY: dev-worker
dev-worker: ## Run worker with hot reload (requires air)
	@which air > /dev/null || go install github.com/cosmtrek/air@latest
	air -c .air-worker.toml

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

##@ Kubernetes & Helm

.PHONY: helm-deps
helm-deps: ## Update Helm chart dependencies
	@which helm > /dev/null || (echo "Helm not found. Install from https://helm.sh/docs/intro/install/" && exit 1)
	cd deploy/helm/pllm && helm dependency update

.PHONY: helm-lint
helm-lint: helm-deps ## Lint Helm chart
	cd deploy/helm && helm lint pllm

.PHONY: helm-template
helm-template: helm-deps ## Generate Kubernetes manifests from Helm chart
	cd deploy/helm && helm template pllm pllm \
		--set pllm.secrets.jwtSecret="demo-jwt-secret" \
		--set pllm.secrets.masterKey="sk-master-demo" \
		--set dex.config.issuer="http://pllm-dex.pllm.svc.cluster.local:5556/dex" \
		--namespace pllm

.PHONY: helm-install
helm-install: helm-deps ## Install PLLM using Helm chart (with demo values)
	@echo "Installing PLLM with demo configuration..."
	@echo "⚠️  This uses demo secrets. Update values.yaml for production!"
	cd deploy/helm && helm upgrade --install pllm pllm \
		--set pllm.secrets.jwtSecret="demo-jwt-secret-$(shell date +%s)" \
		--set pllm.secrets.masterKey="sk-master-demo-$(shell date +%s)" \
		--set pllm.secrets.openaiApiKey="$$OPENAI_API_KEY" \
		--set pllm.secrets.anthropicApiKey="$$ANTHROPIC_API_KEY" \
		--set dex.config.issuer="http://pllm-dex.pllm.svc.cluster.local:5556/dex" \
		--create-namespace \
		--namespace pllm \
		--wait \
		--timeout 300s
	@echo ""
	@echo "✅ PLLM installed successfully!"
	@echo ""

.PHONY: helm-install-minikube
helm-install-minikube: helm-deps ## Install PLLM with Minikube ingress configuration
	@echo "Installing PLLM with Minikube ingress configuration..."
	@echo "⚠️  Using predefined demo secrets from values-minikube.yaml"
	cd deploy/helm && helm upgrade --install pllm pllm \
		-f values-minikube.yaml \
		--create-namespace \
		--namespace pllm \
		--wait \
		--timeout 300s
	@echo ""
	@echo "✅ PLLM installed with Minikube ingress!"
	@echo ""
	@echo "🔗 Access PLLM:"
	@echo "   http://pllm.local"
	@echo ""
	@echo "🔐 Access Dex Authentication:"
	@echo "   http://dex.local/dex"
	@echo ""
	@echo "🔑 Master key for admin access:"
	@echo "   sk-master-demo-minikube"
	@echo ""
	@echo "📝 Note: Make sure pllm.local is in your /etc/hosts:"
	@echo "   $$(minikube ip) pllm.local"

	@MINIKUBE_IP=$$(minikube ip); \
	if ! grep -q "pllm.local" /etc/hosts; then \
		echo "🔧 Adding pllm.local to /etc/hosts (requires sudo)..."; \
		echo "$$MINIKUBE_IP pllm.local" | sudo tee -a /etc/hosts; \
	else \
		echo "🔧 Updating pllm.local in /etc/hosts (requires sudo)..."; \
		sudo sed -i.bak "s/.* pllm.local.*/$$MINIKUBE_IP pllm.local/" /etc/hosts; \
	fi

	@echo ""
	@echo "✅ Ingress setup complete!"
	@echo "🌐 Access PLLM at: http://pllm.local"
	@echo "🔐 Access Dex at: http://pllm.local/dex"
	@echo "🔑 Get master key: kubectl get secret -n pllm pllm -o jsonpath='{.data.master-key}' | base64 -d"

.PHONY: helm-install-prod
helm-install-prod: helm-deps ## Install PLLM with production values (requires values-prod.yaml)
	@if [ ! -f deploy/helm/values-prod.yaml ]; then \
		echo "❌ values-prod.yaml not found. Create it first:"; \
		echo "   cp deploy/helm/pllm/values.yaml deploy/helm/values-prod.yaml"; \
		echo "   # Edit deploy/helm/values-prod.yaml with your production config"; \
		exit 1; \
	fi
	cd deploy/helm && helm upgrade --install pllm pllm \
		-f values-prod.yaml \
		--create-namespace \
		--namespace pllm \
		--wait \
		--timeout 600s
	@echo "✅ PLLM installed in production mode!"

.PHONY: helm-upgrade
helm-upgrade: helm-deps ## Upgrade PLLM Helm release
	cd deploy/helm && helm upgrade pllm pllm \
		--namespace pllm \
		--reuse-values \
		--wait \
		--timeout 300s
	@echo "✅ PLLM upgraded successfully!"

.PHONY: helm-uninstall
helm-uninstall: ## Uninstall PLLM Helm release
	@echo "🗑️  Uninstalling PLLM..."
	helm uninstall pllm --namespace pllm
	@echo ""
	@read -p "❓ Delete persistent data? (PVCs will be removed) [y/N]: " confirm; \
	if [ "$$confirm" = "y" ] || [ "$$confirm" = "Y" ]; then \
		kubectl delete pvc -n pllm -l app.kubernetes.io/instance=pllm; \
		echo "💥 Persistent data deleted!"; \
	else \
		echo "💾 Persistent data preserved."; \
	fi
	@echo "✅ PLLM uninstalled!"

.PHONY: helm-status
helm-status: ## Show PLLM Helm release status
	@helm list -n pllm
	@echo ""
	@kubectl get pods -n pllm -l app.kubernetes.io/name=pllm
	@echo ""
	@echo "📊 Helm Status:"
	@helm status pllm -n pllm

.PHONY: helm-logs
helm-logs: ## Show PLLM pods logs
	kubectl logs -n pllm -l app.kubernetes.io/name=pllm --tail=100 -f

.PHONY: helm-shell
helm-shell: ## Open shell in PLLM pod
	kubectl exec -n pllm -it $$(kubectl get pod -n pllm -l app.kubernetes.io/name=pllm -o jsonpath='{.items[0].metadata.name}') -- sh

.PHONY: k8s-port-forward
k8s-port-forward: ## Forward PLLM ports to localhost
	@echo "🔗 Port forwarding PLLM services..."
	@echo "   Main API: http://localhost:8080"
	@echo "   Admin API: http://localhost:8081"
	@echo "   Metrics: http://localhost:9090"
	@echo ""
	@echo "Press Ctrl+C to stop port forwarding"
	kubectl port-forward -n pllm svc/pllm 8080:8080 8081:8081 9090:9090

.PHONY: minikube-url
minikube-url: ## Get Minikube service URL for PLLM
	@echo "🌐 Getting Minikube service URL..."
	@minikube service pllm -n pllm --url
	@echo ""
	@echo "💡 Tip: Use 'make minikube-open' to open in browser"

.PHONY: minikube-open
minikube-open: ## Open PLLM in browser via Minikube service
	@echo "🚀 Opening PLLM in browser..."
	minikube service pllm -n pllm

.PHONY: minikube-tunnel
minikube-tunnel: ## Start Minikube tunnel for LoadBalancer services (requires sudo)
	@echo "🚇 Starting Minikube tunnel..."
	@echo "⚠️  This requires sudo privileges and will run until stopped with Ctrl+C"
	@echo "💡 Use this if you have LoadBalancer services"
	sudo minikube tunnel

.PHONY: minikube-ingress-setup
minikube-ingress-setup: helm-deps ## Setup ingress for PLLM on Minikube
	@echo "🌐 Setting up ingress for Minikube..."

	# Enable ingress addon
	@minikube addons enable ingress
	@echo "✅ Ingress addon enabled"

	# Wait for ingress controller to be ready
	@echo "⏳ Waiting for ingress controller..."
	@kubectl wait --namespace ingress-nginx \
		--for=condition=ready pod \
		--selector=app.kubernetes.io/component=controller \
		--timeout=120s

	# Install/upgrade PLLM with ingress configuration
	@echo "🔧 Installing PLLM with ingress configuration..."
	@echo "⚠️  This uses demo secrets. Update values.yaml for production!"
	cd deploy/helm && helm upgrade --install pllm pllm \
		-f values-minikube.yaml \
		--set pllm.secrets.jwtSecret="demo-jwt-secret-$(shell date +%s)" \
		--set pllm.secrets.masterKey="sk-master-demo-$(shell date +%s)" \
		--set pllm.secrets.openaiApiKey="$$OPENAI_API_KEY" \
		--set pllm.secrets.anthropicApiKey="$$ANTHROPIC_API_KEY" \
		--create-namespace \
		--namespace pllm \
		--wait \
		--timeout 300s

	# Add to hosts file
	@MINIKUBE_IP=$$(minikube ip); \
	if ! grep -q "pllm.local" /etc/hosts; then \
		echo "🔧 Adding pllm.local to /etc/hosts (requires sudo)..."; \
		echo "$$MINIKUBE_IP pllm.local" | sudo tee -a /etc/hosts; \
	else \
		echo "🔧 Updating pllm.local in /etc/hosts (requires sudo)..."; \
		sudo sed -i.bak "s/.* pllm.local.*/$$MINIKUBE_IP pllm.local/" /etc/hosts; \
	fi

	@echo ""
	@echo "✅ Ingress setup complete!"
	@echo "🌐 Access PLLM at: http://pllm.local"
	@echo "🔐 Access Dex at: http://pllm.local/dex"
	@echo "🔑 Get master key: kubectl get secret -n pllm pllm -o jsonpath='{.data.master-key}' | base64 -d"

.PHONY: minikube-ingress-cleanup
minikube-ingress-cleanup: ## Remove ingress setup for PLLM on Minikube
	@echo "🧹 Cleaning up ingress setup..."

	# Remove from hosts file
	@if grep -q "pllm.local" /etc/hosts; then \
		echo "🔧 Removing pllm.local from /etc/hosts (requires sudo)..."; \
		sudo sed -i.bak '/pllm.local/d' /etc/hosts; \
	fi

	# Disable ingress in PLLM
	@cd deploy/helm && helm upgrade pllm pllm \
		--set ingress.enabled=false \
		--reuse-values \
		-n pllm

	@echo "✅ Ingress cleanup complete!"
	@echo "💡 Use 'make k8s-port-forward' or 'make minikube-open' to access PLLM"

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
test:
	mkdir -p internal/ui/dist
	mkdir -p internal/docs/dist
	touch internal/ui/dist/index.html
	touch internal/docs/dist/index.html
	go test -v ./...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage
	mkdir -p internal/ui/dist
	mkdir -p internal/docs/dist
	touch internal/ui/dist/index.html
	touch internal/docs/dist/index.html
	go test -v -cover -coverprofile=coverage.txt ./...

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
	swag init -g cmd/server/main.go -o internal/handlers/swagger

##@ Documentation

.PHONY: docs-dev
docs-dev: ## Run VitePress documentation in development mode
	cd docs && npm run dev

.PHONY: docs-build
docs-build: ## Build VitePress documentation
	cd docs && npm run build
	mkdir -p internal/docs/dist
	cp -r docs/.vitepress/dist/* internal/docs/dist/

.PHONY: docs-preview
docs-preview: ## Preview built documentation
	cd docs && npm run preview

.PHONY: clean
clean: ## Clean build artifacts
	rm -rf bin/ tmp/ coverage.* *.out internal/docs/dist

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
