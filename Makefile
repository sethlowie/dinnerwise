SHELL := /bin/bash

BUF_VERSION := v1.61.0
DINNERWISE_DB ?= dinnerwise.db

.PHONY: help
help: ## List targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

## ---- tooling ----
.PHONY: install-tools
install-tools: ## Install codegen tools (buf)
	go install github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)

## ---- codegen ----
.PHONY: gen
gen: ## Generate Go + TS from protos
	buf generate

## ---- build ----
.PHONY: build
build: ## Build the server binary to ./build/dinnerwise
	CGO_ENABLED=0 go build -trimpath -o ./build/dinnerwise ./cmd/server

## ---- dev loop ----
.PHONY: run
run: ## Run the Connect server (DB path via DINNERWISE_DB, default dinnerwise.db)
	go run ./cmd/server

.PHONY: web
web: ## Run the Vite dev server (web/app)
	cd web/app && pnpm dev

.PHONY: db-shell
db-shell: ## Open a sqlite3 shell against the local database
	sqlite3 $(DINNERWISE_DB)

## ---- quality ----
.PHONY: test
test: ## Run Go unit tests
	go test -race -short ./...

.PHONY: tidy
tidy: ## Tidy go modules
	go mod tidy

## ---- observability ----
.PHONY: obs obs-down
obs: ## Start the local Grafana otel-lgtm stack via Tilt (Grafana :3000, OTLP :4317/:4318)
	tilt up

obs-down: ## Tear down the observability stack
	tilt down
