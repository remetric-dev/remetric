GO               ?= go
GOLANGCI_VERSION := v2.12.1
GOLANGCI_IMAGE   := golangci/golangci-lint:$(GOLANGCI_VERSION)
GOFUMPT_VERSION  := v0.7.0
GOVULN_VERSION   := v1.3.0
GORELEASER_VERSION := v2.10.0
LOCAL_PREFIX     := github.com/remetric-dev/remetric

VERSION          ?= $(shell git describe --tags --dirty --always 2>/dev/null || echo dev)
LDFLAGS          := -s -w -X main.version=$(VERSION)

.PHONY: help build test test-race fmt vet lint vuln clean e2e-up e2e-down e2e release-check release-snapshot docker-build install-check

help: ## Show this help
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-14s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build static binary into ./bin/remetric
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/remetric ./cmd/remetric

test: ## Run unit tests
	$(GO) test ./...

test-race: ## Run unit tests with -race
	$(GO) test -race ./...

clean: ## Remove build outputs
	rm -rf bin coverage.out

fmt: ## Format with gofumpt + goimports
	$(GO) run mvdan.cc/gofumpt@$(GOFUMPT_VERSION) -l -w .
	$(GO) run golang.org/x/tools/cmd/goimports@latest -local $(LOCAL_PREFIX) -l -w .

vet: ## go vet all packages
	$(GO) vet ./...

lint: ## golangci-lint (local if installed, otherwise pinned docker image)
	@if command -v golangci-lint >/dev/null 2>&1; then \
	  echo ">> using local golangci-lint ($$(golangci-lint version 2>/dev/null | head -n1))"; \
	  golangci-lint run --timeout 5m; \
	else \
	  echo ">> golangci-lint not found locally, using docker $(GOLANGCI_IMAGE)"; \
	  docker run --rm -v $(PWD):/app -w /app $(GOLANGCI_IMAGE) golangci-lint run --timeout 5m; \
	fi

vuln: ## Scan for known vulnerabilities via govulncheck
	$(GO) run golang.org/x/vuln/cmd/govulncheck@$(GOVULN_VERSION) ./...

e2e-up: ## Start e2e Prometheus stack
	docker compose -f e2e/docker-compose.yml up -d

e2e-down: ## Tear down e2e stack
	docker compose -f e2e/docker-compose.yml down -v

e2e: ## Run e2e tests (requires e2e-up)
	$(GO) test -tags=e2e -count=1 ./e2e/...

release-check: ## Verify .goreleaser.yml is well-formed
	$(GO) run github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION) check

release-snapshot: ## Build cross-platform archives locally (no publish)
	$(GO) run github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION) release --snapshot --clean

DOCKER_IMAGE ?= remetric:dev

docker-build: ## Build docker image (single-arch, host CPU)
	docker build --build-arg VERSION=$(VERSION) -t $(DOCKER_IMAGE) .

install-check: ## Lint install.sh with shellcheck if installed
	@if command -v shellcheck >/dev/null 2>&1; then \
	  shellcheck install.sh; \
	else \
	  echo ">> shellcheck not found, skipping (install via: brew install shellcheck)"; \
	fi
