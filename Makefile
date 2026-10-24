.PHONY: build web dev dev-web test test-go install uninstall up down restart status logs clean help
.DEFAULT_GOAL := help

BIN := bin/server
UNIT := awesome-harness.service
UNIT_SRC := $(CURDIR)/systemd/$(UNIT)
UNIT_DST := $(HOME)/.config/systemd/user/$(UNIT)

# HOST/PORT from .env (the service reads the same file), for the URL `up` prints.
-include .env
HOST ?= 127.0.0.1
PORT ?= 8769

build: ## Build the web bundle and compile the server binary (UI embedded)
	pnpm install
	pnpm build
	@touch web/dist/.gitkeep
	go build -o $(BIN) ./cmd/server


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

up: ## Start the service in the background (runs install first if the unit isn't installed)
	@test -e "$(UNIT_DST)" || $(MAKE) install
	systemctl --user start $(UNIT)
	@echo "Awesome Harnesses is up: http://$(HOST):$(PORT)"

down: ## Stop the service (it stays installed; `make up` starts it again)
	systemctl --user stop $(UNIT)

restart: ## Restart the user systemd unit
	systemctl --user restart $(UNIT)

status: ## Show the status of the user systemd unit
	systemctl --user status $(UNIT)

logs: ## Follow the service log
	journalctl --user -u $(UNIT) -f

clean: ## Remove the binary, the built web bundle and test output
	rm -rf bin test-results playwright-report .e2e-home
	find web/dist -mindepth 1 ! -name .gitkeep -delete

help: ## Show this help
	@grep -E '^[a-zA-Z0-9_-]+:.*##' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'
