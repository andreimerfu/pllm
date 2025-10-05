# pLLM CLI Makefile

.PHONY: build clean install uninstall test help completion

BINARY_NAME=pllm
BUILD_DIR=../../bin
GO_FILES=$(shell find . -name '*.go')

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: $(BUILD_DIR)/$(BINARY_NAME) ## Build the CLI binary

$(BUILD_DIR)/$(BINARY_NAME): $(GO_FILES)
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) .

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -f $(BUILD_DIR)/$(BINARY_NAME)

install: build ## Install CLI to system path
	@echo "Installing $(BINARY_NAME) to $(GOPATH)/bin..."
	@go install .

uninstall: ## Remove CLI from system path
	@echo "Removing $(BINARY_NAME) from $(GOPATH)/bin..."
	@rm -f $(GOPATH)/bin/$(BINARY_NAME)

test: ## Run tests
	@echo "Running tests..."
	@go test ./... -v

completion-bash: build ## Generate bash completion script
	@echo "Generating bash completion..."
	@$(BUILD_DIR)/$(BINARY_NAME) completion bash > $(BINARY_NAME).bash
	@echo "Install with: sudo cp $(BINARY_NAME).bash /etc/bash_completion.d/"

completion-zsh: build ## Generate zsh completion script
	@echo "Generating zsh completion..."
	@$(BUILD_DIR)/$(BINARY_NAME) completion zsh > _$(BINARY_NAME)
	@echo "Install with: cp _$(BINARY_NAME) ~/.oh-my-zsh/completions/ or add to fpath"

dev-run: build ## Run with database access for development
	@$(BUILD_DIR)/$(BINARY_NAME) --db-url "postgres://pllm:pllm@localhost:5432/pllm?sslmode=disable" $(ARGS)

dev-api: build ## Run with API access for development
	@$(BUILD_DIR)/$(BINARY_NAME) --api-url "http://localhost:8080" --api-key "$(API_KEY)" $(ARGS)

# Example usage:
# make build
# make install
# make completion-bash
# make dev-run ARGS="user list"
# make dev-api API_KEY="your-key" ARGS="budget status"