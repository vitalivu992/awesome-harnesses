.PHONY: build web dev dev-web test test-go install uninstall restart status help

BIN := bin/server
UNIT := awesome-harness.service
UNIT_SRC := $(CURDIR)/systemd/$(UNIT)
UNIT_DST := $(HOME)/.config/systemd/user/$(UNIT)

build: web ## Build the web bundle and compile the server binary (UI embedded)
	go build -o $(BIN) ./cmd/server

web: ## Install JS deps and build the React app into web/dist/ (embedded into the binary)
	pnpm install
	pnpm build
	@touch web/dist/.gitkeep

dev: web ## Build the app, then run the Go server locally on :8767
	go run ./cmd/server

dev-web: ## Vite dev server with hot reload; proxies /api to the Go server on :8767 (run `make dev` too)
	pnpm dev

test: test-go ## Run Go, unit (vitest) and e2e (playwright) tests
	pnpm test:unit
	pnpm test:e2e

test-go: ## Run the Go tests
	go test ./...

install: build ## Build the binary and (re)install + restart the user systemd unit
	@test -f .env || cp .env.example .env
	mkdir -p "$(HOME)/.config/systemd/user"
	ln -sfn "$(UNIT_SRC)" "$(UNIT_DST)"
	systemctl --user daemon-reload
	systemctl --user enable $(UNIT)
	systemctl --user restart $(UNIT)
	systemctl --user --no-pager --full status $(UNIT)

uninstall: ## Stop and disable the user systemd unit
	systemctl --user disable --now $(UNIT)
	rm -f "$(UNIT_DST)"
	systemctl --user daemon-reload

restart: ## Restart the user systemd unit
	systemctl --user restart $(UNIT)

status: ## Show the status of the user systemd unit
	systemctl --user status $(UNIT)

help: ## Show this help
	@grep -E '^[a-zA-Z0-9_-]+:.*##' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'
