.DEFAULT_GOAL := help
.PHONY: help build test lint fmt vet tidy clean

GO_SERVICES := $(wildcard services/*/go.mod)
GO_PKGS     := $(wildcard packages/*/go.mod)

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
	  awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build all Go services and packages
	@for mod in $(GO_SERVICES) $(GO_PKGS); do \
	  dir=$$(dirname $$mod); \
	  echo "→ building $$dir"; \
	  (cd $$dir && go build ./...); \
	done

test: ## Run all Go tests
	@for mod in $(GO_SERVICES) $(GO_PKGS); do \
	  dir=$$(dirname $$mod); \
	  echo "→ testing $$dir"; \
	  (cd $$dir && go test -race -count=1 ./...); \
	done

lint: ## Run golangci-lint on all Go modules
	@for mod in $(GO_SERVICES) $(GO_PKGS); do \
	  dir=$$(dirname $$mod); \
	  echo "→ linting $$dir"; \
	  (cd $$dir && golangci-lint run ./...); \
	done

fmt: ## Format all Go source files
	@for mod in $(GO_SERVICES) $(GO_PKGS); do \
	  dir=$$(dirname $$mod); \
	  (cd $$dir && gofmt -w .); \
	done

vet: ## Run go vet on all Go modules
	@for mod in $(GO_SERVICES) $(GO_PKGS); do \
	  dir=$$(dirname $$mod); \
	  (cd $$dir && go vet ./...); \
	done

tidy: ## Run go mod tidy on all Go modules
	@for mod in $(GO_SERVICES) $(GO_PKGS); do \
	  dir=$$(dirname $$mod); \
	  (cd $$dir && go mod tidy); \
	done

clean: ## Remove build artifacts
	@for mod in $(GO_SERVICES) $(GO_PKGS); do \
	  dir=$$(dirname $$mod); \
	  (cd $$dir && go clean ./...); \
	done

ts-build: ## Build all TypeScript packages
	npx turbo build

ts-test: ## Run all TypeScript tests
	npx turbo test

ts-lint: ## Lint all TypeScript packages
	npx turbo lint

ts-typecheck: ## Type-check all TypeScript packages
	npx turbo typecheck
