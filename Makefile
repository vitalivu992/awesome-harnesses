.PHONY: build web run dev dev-web test test-go install uninstall up down restart status logs clean help
.DEFAULT_GOAL := help

# The service targets go through ctl.sh: a systemd user unit on Linux, a launchd
# agent on macOS (`./ctl.sh help`).
CTL := ./ctl.sh

build: ## Build the web bundle and compile the server binary (UI embedded)
	$(CTL) build

web: ## Install JS deps and build the React app into web/dist/ (embedded into the binary)
	pnpm install
	pnpm build
	@touch web/dist/.gitkeep

run: build ## Build, then run the server in the foreground with .env applied
	$(CTL) run

dev: web ## Build the app, then run the Go server locally on :8767
	go run ./cmd/server

dev-web: ## Vite dev server with hot reload; proxies /api to the Go server on :8767 (run `make dev` too)
	pnpm dev

test: test-go ## Run Go, unit (vitest) and e2e (playwright) tests
	pnpm test:unit
	pnpm test:e2e

test-go: ## Run the Go tests
	go test ./...

install: ## Build, then (re)install + restart the background service (systemd on Linux, launchd on macOS)
	$(CTL) install

uninstall: ## Stop and remove the background service
	$(CTL) uninstall

up: ## Start the service in the background (installs it the first time)
	$(CTL) up

down: ## Stop the service (it stays installed; `make up` starts it again)
	$(CTL) down

restart: ## Restart the service
	$(CTL) restart

status: ## Show the status of the service
	$(CTL) status

logs: ## Follow the service log
	$(CTL) logs

clean: ## Remove the binary, the built web bundle and test output
	rm -rf bin test-results playwright-report .e2e-home
	find web/dist -mindepth 1 ! -name .gitkeep -delete

help: ## Show this help
	@grep -E '^[a-zA-Z0-9_-]+:.*##' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'
