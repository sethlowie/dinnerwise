SHELL := /bin/bash

BUF_VERSION := v1.61.0

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

## ---- build (used by Tilt) ----
.PHONY: build-server
build-server: ## Cross-compile the server binary for the linux container
	CGO_ENABLED=0 GOOS=linux GOARCH=$${GOARCH:-amd64} go build -trimpath -o ./build/dinnerwise ./cmd/dinnerwise

## ---- dev loop ----
.PHONY: dev
dev: ## Bring up the full stack via Tilt
	tilt up

.PHONY: down
down: ## Tear down the Tilt stack
	tilt down

.PHONY: db-shell
db-shell: ## Open a mongosh shell against the in-cluster MongoDB
	kubectl exec -it deploy/mongodb -- mongosh dinnerwise

## ---- quality ----
.PHONY: test
test: ## Run Go unit tests
	go test -race -short ./...

.PHONY: tidy
tidy: ## Tidy go modules
	go mod tidy
