.DEFAULT_GOAL := help

BINARY := tfjournal

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "%-12s %s\n", $$1, $$2}'

web: ## Build web UI
	cd web && npm install && npm run build
	rm -rf server/dist
	cp -r web/dist server/

build: web ## Build binary
	go build -ldflags="-s -w" -o $(BINARY) .

dev: ## Run web UI dev server (vite)
	cd web && npm run dev

test: ## Run tests
	go test -v ./...

lint: ## Run golangci-lint
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found, skipping"; \
	fi

fmt: ## Format code
	gofmt -s -w .
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	fi

vet: ## Run go vet
	go vet ./...

check: fmt vet lint test ## Run all checks

install: build ## Install to GOPATH/bin
	go install .

clean: ## Remove build artifacts
	rm -f $(BINARY)

.PHONY: help web build dev test lint fmt vet check install clean
