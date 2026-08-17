SHELL   := bash
BIN_DIR := bin

# Every subdir of cmd/ is a buildable app. APP selects one (default: first).
APPS := $(notdir $(wildcard cmd/*))
APP  ?= $(firstword $(APPS))

IMAGE ?= go-template
PORT  ?= 8080

# Pinned: .golangci.yml uses the v1 config format, which v2 does not read.
GOLANGCI_VERSION ?= 1.64.8

.PHONY: help run test cover vet lint lint-install fmt check tidy build build-all clean oapicodegen docker-build docker-run

help: ## List targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  %-12s %s\n", $$1, $$2}'
	@echo "  apps: $(APPS)"

run: ## Run an app (APP=name, default $(APP))
	go run ./cmd/$(APP)

test: ## Run tests with race detector
	go test -race ./...

cover: ## Run tests and open coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

vet: ## go vet
	go vet ./...

lint: ## golangci-lint, configured by .golangci.yml (see lint-install)
	golangci-lint run

lint-install: ## Install the golangci-lint version .golangci.yml is written for
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v$(GOLANGCI_VERSION)

fmt: ## Format and fix imports (go install golang.org/x/tools/cmd/goimports@latest)
	goimports -w .

tidy: ## Sync go.mod/go.sum
	go mod tidy

check: fmt vet lint test ## Format, vet, lint, test

EXE := $(shell go env GOEXE)

build: ## Build every app for the host OS (.exe on Windows)
	@for app in $(APPS); do \
		echo "building $$app$(EXE)"; \
		go build -o $(BIN_DIR)/$$app$(EXE) ./cmd/$$app; \
	done

build-all: ## Cross-compile every app for linux, windows, darwin (amd64 + arm64)
	@for app in $(APPS); do \
		for os in linux windows darwin; do \
			for arch in amd64 arm64; do \
				ext=$$( [ $$os = windows ] && echo .exe || echo ); \
				echo "building $$app $$os/$$arch"; \
				GOOS=$$os GOARCH=$$arch go build -o $(BIN_DIR)/$$app-$$os-$$arch$$ext ./cmd/$$app; \
			done; \
		done; \
	done

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR) coverage.out

oapicodegen: ## Generate OpenAPI server code
	bash scripts/oapicodegen.sh

docker-build: ## Build the image (APP=name, IMAGE=tag)
	docker build --build-arg APP=$(APP) -t $(IMAGE) .

docker-run: ## Run the image with configs/ mounted read-only (PORT must match server.port)
	docker run --rm -p $(PORT):$(PORT) -v "$(CURDIR)/configs:/app/configs:ro" $(IMAGE)