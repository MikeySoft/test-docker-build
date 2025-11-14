SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c

REGISTRY ?= ghcr.io/mikeysoft
SERVER_IMAGE ?= $(REGISTRY)/flotilla-server
AGENT_IMAGE ?= $(REGISTRY)/flotilla-agent
PLATFORMS ?= linux/amd64,linux/arm64
ARCH ?= linux/amd64
ARCH_SUFFIX := $(notdir $(ARCH))
ARCH_LIST ?=
BUILDKIT_CACHE ?= false
CACHE_NAMESPACE ?= flotilla
CACHE_READONLY ?= false
BIN_PLATFORMS := linux/amd64 linux/arm64 darwin/arm64
DIST_DIR := dist/release
TMP_DIR := $(DIST_DIR)/tmp

VERSION ?= $(shell ./scripts/resolve-version.sh)
SAFE_VERSION := $(subst /,-,$(VERSION))
MAJOR := $(word 1,$(subst ., ,$(VERSION)))
MINOR := $(word 2,$(subst ., ,$(VERSION)))
PATCH := $(word 3,$(subst ., ,$(VERSION)))

ifeq ($(findstring -,$(VERSION)),)
IS_RELEASE := true
else
IS_RELEASE := false
endif

ifeq ($(IS_RELEASE),true)
PRIMARY_TAG := v$(VERSION)
else
PRIMARY_TAG := $(SAFE_VERSION)
endif

SERVER_TAGS_BASE := $(SERVER_IMAGE):$(PRIMARY_TAG)
AGENT_TAGS_BASE := $(AGENT_IMAGE):$(PRIMARY_TAG)
ifeq ($(IS_RELEASE),true)
SERVER_TAGS_BASE += $(SERVER_IMAGE):v$(MAJOR).$(MINOR) $(SERVER_IMAGE):v$(MAJOR) $(SERVER_IMAGE):latest
AGENT_TAGS_BASE += $(AGENT_IMAGE):v$(MAJOR).$(MINOR) $(AGENT_IMAGE):v$(MAJOR) $(AGENT_IMAGE):latest
endif
ifeq ($(strip $(EDGE_TAG)),true)
SERVER_TAGS_BASE += $(SERVER_IMAGE):edge
AGENT_TAGS_BASE += $(AGENT_IMAGE):edge
endif

ifeq ($(strip $(PUBLISH)),true)
SERVER_TAGS := $(foreach tag,$(SERVER_TAGS_BASE),$(tag)-$(ARCH_SUFFIX))
AGENT_TAGS := $(foreach tag,$(AGENT_TAGS_BASE),$(tag)-$(ARCH_SUFFIX))
DOCKER_OUTPUT := --push
else
SERVER_TAGS := $(firstword $(SERVER_TAGS_BASE))
AGENT_TAGS := $(firstword $(AGENT_TAGS_BASE))
DOCKER_OUTPUT := --load
endif

ifeq ($(strip $(BUILDKIT_CACHE)),true)
SERVER_CACHE_SCOPE := $(CACHE_NAMESPACE)-server-$(ARCH_SUFFIX)
AGENT_CACHE_SCOPE := $(CACHE_NAMESPACE)-agent-$(ARCH_SUFFIX)
SERVER_CACHE_ARGS := --cache-from type=gha,scope=$(SERVER_CACHE_SCOPE)
AGENT_CACHE_ARGS := --cache-from type=gha,scope=$(AGENT_CACHE_SCOPE)
ifneq ($(strip $(CACHE_READONLY)),true)
SERVER_CACHE_ARGS += --cache-to type=gha,mode=max,scope=$(SERVER_CACHE_SCOPE)
AGENT_CACHE_ARGS += --cache-to type=gha,mode=max,scope=$(AGENT_CACHE_SCOPE)
endif
else
SERVER_CACHE_ARGS :=
AGENT_CACHE_ARGS :=
endif

.PHONY: help deps lint lint-go lint-frontend fmt build-server build-agent build-agent-linux build-frontend build-all run-server run-agent run-dev stop-dev clean dist-clean test test-frontend test-coverage generate-api-key db-migrate dev-setup generate-certs frontend-dev package-binaries docker-build-server docker-build-agent docker-release docker-manifests checksums release release-artifacts release-clean release-deps

help:
	@echo "Flotilla Commands"
	@echo "  make deps              - Install Go module dependencies"
	@echo "  make lint              - Run Go and frontend linters"
	@echo "  make fmt               - Format all Go source files with gofmt"
	@echo "  make test              - Run Go unit tests"
	@echo "  make test-frontend     - Run frontend unit tests"
	@echo "  make build-server      - Build the management server binary"
	@echo "  make build-agent       - Build the agent binary"
	@echo "  make build-frontend    - Build the frontend bundle"
	@echo "  make build-all         - Build server, agent, and frontend"
	@echo "  make run-server        - Run the management server locally"
	@echo "  make run-agent         - Run the agent locally"
	@echo "  make run-dev           - Start docker-compose dev stack + server"
	@echo "  make package-binaries  - Cross-compile release archives"
	@echo "  make docker-release    - Build/push multi-arch Docker images"
	@echo "  make release           - End-to-end release automation"

deps:
	go mod download

lint: lint-go lint-frontend

lint-go:
	@echo "Running Go format check..."
	@files=$$(git ls-files '*.go' 2>/dev/null || true); \
	if [[ -z "$$files" ]]; then \
		files=$$(find . -type f -name '*.go' ! -path './vendor/*'); \
	fi; \
	if [[ -z "$$files" ]]; then \
		echo "No Go files found."; \
	else \
		fmt_output=$$(gofmt -l $$files); \
		if [[ -n "$$fmt_output" ]]; then \
			echo "The following files need gofmt:"; \
			echo "$$fmt_output"; \
			exit 1; \
		fi; \
	fi
	@echo "Running go vet..."
	go vet ./...

lint-frontend:
	@echo "Running frontend lint..."
	cd web && npm run lint

fmt:
	@echo "Formatting Go source files..."
	@files=$$(git ls-files '*.go' 2>/dev/null || true); \
	if [[ -z "$$files" ]]; then \
		files=$$(find . -type f -name '*.go' ! -path './vendor/*'); \
	fi; \
	if [[ -z "$$files" ]]; then \
		echo "No Go files found."; \
	else \
		gofmt -w $$files; \
	fi

build-server:
	@echo "Building management server (version $(VERSION))..."
	go build -trimpath -o bin/server ./cmd/server

build-agent:
	@echo "Building agent (version $(VERSION))..."
	go build -trimpath -o bin/agent ./cmd/agent

build-agent-linux:
	@echo "Building agent for linux/amd64..."
	GOOS=linux GOARCH=amd64 go build -trimpath -o bin/agent-linux-amd64 ./cmd/agent

build-frontend:
	@echo "Building frontend..."
	cd web && npm run build

build-all: build-server build-agent build-frontend

run-server: build-server
	@echo "Starting management server..."
	./bin/server

run-agent: build-agent
	@echo "Starting agent..."
	./bin/agent

run-dev: build-server
	@echo "Starting development docker-compose stack..."
	docker compose -f docker-compose.dev.yml up -d
	@echo "Waiting for PostgreSQL to be ready..."
	@sleep 5
	@echo "Starting server..."
	./bin/server

stop-dev:
	@echo "Stopping development stack..."
	docker compose -f docker-compose.dev.yml down

clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -rf web/dist/
	rm -rf web/build/

dist-clean:
	@echo "Cleaning release artifacts..."
	rm -rf dist/

test:
	@echo "Running Go unit tests..."
	go test ./...

test-frontend:
	@echo "Running frontend tests..."
	cd web && npm run test

test-coverage:
	@echo "Running Go tests with coverage..."
	go test -coverprofile=coverage.out ./...
	@coverage_report=$$(go tool cover -func=coverage.out); \
		echo "$$coverage_report" | tee coverage.txt >/dev/null; \
		total_cov=$$(echo "$$coverage_report" | awk '/^total:/ {print substr($$3, 1, length($$3)-1)}'); \
		required=80; \
		echo "Total coverage: $$total_cov% (required: $$required%)"; \
		awk -v cov="$$total_cov" -v req="$$required" 'BEGIN { exit (cov + 0 >= req ? 0 : 1) }' /dev/null || { \
			echo "Coverage threshold not met"; \
			exit 1; \
		}
	go tool cover -html=coverage.out -o coverage.html

generate-api-key:
	@echo "Generating API key..."
	openssl rand -hex 32

db-migrate:
	@echo "Database migrations are applied automatically on server startup."

dev-setup: deps build-all
	@echo "Development environment setup complete."

generate-certs:
	@echo "Generating TLS certificates for development..."
	command -v mkcert >/dev/null 2>&1 || { echo "mkcert not found; install via brew install mkcert nss"; exit 1; }
	mkdir -p tls
	mkcert -install
	mkcert -key-file tls/key.pem -cert-file tls/cert.pem "localhost" "127.0.0.1" "::1"
	@echo "TLS certificates generated in ./tls/"

frontend-dev:
	@echo "Run the Vite dev server with: cd web && npm run dev"

package-binaries: release-clean
	@echo "Packaging binaries for version $(SAFE_VERSION)..."
	@mkdir -p $(TMP_DIR)
	@mkdir -p $(DIST_DIR)
	@for platform in $(BIN_PLATFORMS); do \
		goos=$${platform%/*}; \
		goarch=$${platform#*/}; \
		out_server=$(TMP_DIR)/server_$${goos}_$${goarch}; \
		out_agent=$(TMP_DIR)/agent_$${goos}_$${goarch}; \
		mkdir -p $$out_server $$out_agent; \
		echo "  -> Building server for $$goos/$$goarch"; \
		CGO_ENABLED=0 GOOS=$$goos GOARCH=$$goarch go build -trimpath -ldflags "-s -w" -o $$out_server/flotilla-server ./cmd/server; \
		echo "  -> Building agent for $$goos/$$goarch"; \
		CGO_ENABLED=0 GOOS=$$goos GOARCH=$$goarch go build -trimpath -ldflags "-s -w" -o $$out_agent/flotilla-agent ./cmd/agent; \
		cp LICENSE $$out_server/; \
		cp LICENSE $$out_agent/; \
		cp env.example $$out_server/server.env.example; \
		cp deployments/systemd/flotilla-server.service $$out_server/; \
		cp deployments/systemd/flotilla-agent.service $$out_agent/; \
		cp deployments/agent/env.example $$out_agent/agent.env.example; \
		tar -czf $(DIST_DIR)/flotilla-server_$(SAFE_VERSION)_$${goos}_$${goarch}.tar.gz -C $$out_server .; \
		tar -czf $(DIST_DIR)/flotilla-agent_$(SAFE_VERSION)_$${goos}_$${goarch}.tar.gz -C $$out_agent .; \
	done
	@rm -rf $(TMP_DIR)

docker-build-server:
	@echo "Building Flotilla server image ($(ARCH))..."
	docker buildx build \
		--platform $(ARCH) \
		$(DOCKER_OUTPUT) \
		$(SERVER_CACHE_ARGS) \
		$(foreach tag,$(SERVER_TAGS),--tag $(tag)) \
		-f Dockerfile \
		.

docker-build-agent:
	@echo "Building Flotilla agent image ($(ARCH))..."
	docker buildx build \
		--platform $(ARCH) \
		$(DOCKER_OUTPUT) \
		$(AGENT_CACHE_ARGS) \
		$(foreach tag,$(AGENT_TAGS),--tag $(tag)) \
		-f deployments/agent/Dockerfile \
		.

docker-release: docker-build-server docker-build-agent

docker-manifests:
	@if [ "$(strip $(PUBLISH))" != "true" ]; then \
		echo "docker-manifests requires PUBLISH=true"; \
		exit 1; \
	fi
	@if [ -z "$(strip $(ARCH_LIST))" ]; then \
		echo "docker-manifests requires ARCH_LIST to list target platforms (e.g. 'linux/amd64 linux/arm64')"; \
		exit 1; \
	fi
	@echo "Creating multi-architecture manifests..."
	@for base in $(SERVER_TAGS_BASE); do \
		repo="$${base%:*}"; \
		tag="$${base##*:}"; \
		refs=""; \
		for arch in $(ARCH_LIST); do \
			suffix="$${arch##*/}"; \
			refs="$$refs $${repo}:$${tag}-$$suffix"; \
		done; \
		echo "  -> $$base"; \
		docker buildx imagetools create --tag $$base $$refs; \
	done
	@for base in $(AGENT_TAGS_BASE); do \
		repo="$${base%:*}"; \
		tag="$${base##*:}"; \
		refs=""; \
		for arch in $(ARCH_LIST); do \
			suffix="$${arch##*/}"; \
			refs="$$refs $${repo}:$${tag}-$$suffix"; \
		done; \
		echo "  -> $$base"; \
		docker buildx imagetools create --tag $$base $$refs; \
	done

checksums:
	@echo "Computing checksums..."
	cd $(DIST_DIR) && shasum -a 256 *.tar.gz > SHA256SUMS

release-clean: dist-clean
	@mkdir -p $(DIST_DIR)

release-deps:
	go mod download
	cd web && npm ci

release-artifacts: lint test test-frontend package-binaries checksums

release: release-artifacts docker-release
	@echo "Release artifacts available in $(DIST_DIR)"

