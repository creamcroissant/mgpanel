# MGPanel Go Edition Makefile
# 项目定位: 非商业化、轻量自托管面板

# Build Variables
BINARY_NAME := mgpanel
AGENT_BINARY_NAME := agent
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -s -w -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)

# Directories
DIST_DIR := dist
USER_FRONTEND_DIR := web/user-vite

# Go environment
GO := go
GOFLAGS := -trimpath

.PHONY: all build build-frontend build-linux build-linux-arm64 build-darwin build-darwin-arm64 build-windows build-all dev test e2e smoke deploy-cmd regression lint sync-check clean help install install-panel install-agent agent agent-linux agent-linux-arm64 agent-darwin-arm64 agent-all

# Default target
all: build

# Help
help:
	@echo "MGPanel Go Edition Build Targets:"
	@echo ""
	@echo "  make build           Build for current platform (with embedded frontend)"
	@echo "  make build-frontend  Build frontend assets only"
	@echo "  make build-backend   Build backend only (no frontend)"
	@echo "  make build-linux     Build for Linux (amd64 and arm64)"
	@echo "  make build-linux-arm64  Build for Linux (arm64)"
	@echo "  make build-darwin    Build for macOS (amd64 and arm64)"
	@echo "  make build-darwin-arm64 Build for macOS (arm64)"
	@echo "  make build-windows   Build for Windows (amd64)"
	@echo "  make build-all       Build for all platforms"
	@echo ""
	@echo "  make agent           Build agent for current platform"
	@echo "  make agent-linux     Build agent for Linux (amd64 and arm64)"
	@echo "  make agent-linux-arm64  Build agent for Linux (arm64)"
	@echo "  make agent-darwin-arm64 Build agent for macOS (arm64)"
	@echo "  make agent-all       Build agent for all platforms"
	@echo ""
	@echo "  make dev             Run in development mode"
	@echo "  make dev-agent       Run agent in development mode"
	@echo "  make test            Run unit tests"
	@echo "  make e2e             Run E2E tests (playwright only)"
	@echo "  make smoke           Run smoke tests (self-bootstrap)"
	@echo "  make deploy-cmd      Run deploy command closure regression"
	@echo "  make regression      Run full regression (e2e + smoke + latest gates)"
	@echo "  make lint            Run linters"
	@echo "  make gen-types        Generate TypeScript types from Go structs (config center)"
	@echo "  make sync-check      Check managed mgpanel sync diff"
	@echo "  make clean           Clean build artifacts"
	@echo "  make install         Install panel and print agent install guidance"
	@echo "  make install-panel   Install panel only"
	@echo "  make install-agent   Install agent only"
	@echo ""
	@echo "Variables:"
	@echo "  VERSION=$(VERSION)"
	@echo "  COMMIT=$(COMMIT)"
	@echo "  BUILD_TIME=$(BUILD_TIME)"

# Build frontend assets
build-frontend:
	@echo "==> Building User Frontend (includes Admin)..."
	cd $(USER_FRONTEND_DIR) && npm ci && npm run build
	@echo "==> Frontend build complete"

# Build for current platform
build: build-frontend
	@echo "==> Building $(BINARY_NAME) for current platform..."
	@mkdir -p $(DIST_DIR)
	@if [ -d "$(DIST_DIR)/$(BINARY_NAME)" ]; then \
		echo "==> Error: $(DIST_DIR)/$(BINARY_NAME) is a directory, cannot overwrite"; \
		exit 1; \
	fi
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME) ./cmd/mgpanel
	@echo "==> Build complete: ./$(DIST_DIR)/$(BINARY_NAME)"

# Build without frontend (for development)
build-backend:
	@echo "==> Building $(BINARY_NAME) (backend only)..."
	@mkdir -p $(DIST_DIR)
	@if [ -d "$(DIST_DIR)/$(BINARY_NAME)" ]; then \
		echo "==> Error: $(DIST_DIR)/$(BINARY_NAME) is a directory, cannot overwrite"; \
		exit 1; \
	fi
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME) ./cmd/mgpanel
	@echo "==> Build complete: ./$(DIST_DIR)/$(BINARY_NAME)"

# Build for Linux
build-linux: build-frontend
	@mkdir -p $(DIST_DIR)
	@echo "==> Building for linux/amd64..."
	GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/mgpanel
	@echo "==> Building for linux/arm64..."
	GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/mgpanel
	@echo "==> Linux builds complete"

# Build for Linux (arm64 only)
build-linux-arm64: build-frontend
	@mkdir -p $(DIST_DIR)
	@echo "==> Building for linux/arm64..."
	GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/mgpanel
	@echo "==> Linux arm64 build complete"

# Build for macOS
build-darwin: build-frontend
	@mkdir -p $(DIST_DIR)
	@echo "==> Building for darwin/amd64..."
	GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/mgpanel
	@echo "==> Building for darwin/arm64..."
	GOOS=darwin GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/mgpanel
	@echo "==> macOS builds complete"

# Build for macOS (arm64 only)
build-darwin-arm64: build-frontend
	@mkdir -p $(DIST_DIR)
	@echo "==> Building for darwin/arm64..."
	GOOS=darwin GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/mgpanel
	@echo "==> macOS arm64 build complete"

# Build for Windows
build-windows: build-frontend
	@mkdir -p $(DIST_DIR)
	@echo "==> Building for windows/amd64..."
	GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/mgpanel
	@echo "==> Windows build complete"

# Build for all platforms
build-all: build-frontend build-linux build-darwin build-windows
	@echo "==> Generating SHA256 checksums..."
	cd $(DIST_DIR) && sha256sum * > SHA256SUMS.txt || shasum -a 256 * > SHA256SUMS.txt
	@echo "==> All platform builds complete"

# Build everything for release (Panel + Agent)
release-all: build-all agent-all
	@echo "==> Generating SHA256 checksums (including agents)..."
	cd $(DIST_DIR) && sha256sum * > SHA256SUMS.txt || shasum -a 256 * > SHA256SUMS.txt
	@echo "==> Release build complete"

# Install locally
install: install-panel
	@echo "==> Panel installed. Install agents with: sudo ./deploy/agent.sh -k <communication-key> -g <panel-host:port>"

install-panel:
	@echo "==> Installing MGPanel panel..."
	sudo ./deploy/panel.sh

install-agent:
	@echo "==> Installing MGPanel agent..."
	sudo ./deploy/agent.sh

# Build Docker image
docker:
	@echo "==> Building Docker image..."
	docker build -t mgpanel .

# Development mode
dev:
	@echo "==> Starting development server..."
	$(GO) run ./cmd/mgpanel serve --config config.yml

# Run tests
test:
	@echo "==> Running tests..."
	$(GO) test -v -race -cover ./...

# Run E2E tests
# Optional overrides: E2E_API_PORT, E2E_GRPC_PORT, E2E_DB_PATH, SMOKE_ACCOUNT, SMOKE_PASSWORD, SMOKE_ADMIN_EMAIL, SMOKE_ADMIN_USERNAME
e2e:
	@echo "==> Running E2E tests (Playwright only)..."
	E2E_MODE=e2e ./scripts/e2e-test.sh

# Run smoke tests
# Optional overrides: E2E_API_PORT, E2E_GRPC_PORT, E2E_DB_PATH, SMOKE_ACCOUNT, SMOKE_PASSWORD, SMOKE_ADMIN_EMAIL, SMOKE_ADMIN_USERNAME
smoke:
	@echo "==> Running smoke tests (self-bootstrap)..."
	E2E_MODE=smoke ./scripts/e2e-test.sh

# Deploy command regression
# Optional overrides: E2E_API_PORT, E2E_GRPC_PORT, E2E_DB_PATH, SMOKE_ACCOUNT, SMOKE_PASSWORD, SMOKE_ADMIN_EMAIL, SMOKE_ADMIN_USERNAME
deploy-cmd:
	@echo "==> Running deploy command regression..."
	E2E_MODE=deploy-cmd ./scripts/e2e-test.sh

# Full regression (backend + e2e + smoke with latest assertions)
# Optional overrides: E2E_API_PORT, E2E_GRPC_PORT, E2E_DB_PATH, SMOKE_ACCOUNT, SMOKE_PASSWORD, SMOKE_ADMIN_EMAIL, SMOKE_ADMIN_USERNAME
regression:
	@echo "==> Running full regression..."
	E2E_MODE=full ./scripts/e2e-test.sh

# Run linters
lint:
	@echo "==> Running golangci-lint..."
	@which golangci-lint > /dev/null 2>&1 || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...
	@echo "==> Running frontend lint..."
	cd $(USER_FRONTEND_DIR) && npm run lint || true

# Check managed sync diff against mgpanel
sync-check:
	@echo "==> Checking managed mgpanel sync diff..."
	./scripts/repo-diff.sh --check-managed

# Clean build artifacts
clean:
	@echo "==> Cleaning build artifacts..."
	@if [ -f "$(BINARY_NAME)" ]; then \
		rm -f "$(BINARY_NAME)"; \
		echo "==> Removed $(BINARY_NAME)"; \
	elif [ -d "$(BINARY_NAME)" ]; then \
		echo "==> Skip $(BINARY_NAME): is a directory"; \
	else \
		echo "==> Skip $(BINARY_NAME): not found"; \
	fi
	@if [ -f "$(AGENT_BINARY_NAME)" ]; then \
		rm -f "$(AGENT_BINARY_NAME)"; \
		echo "==> Removed $(AGENT_BINARY_NAME)"; \
	elif [ -d "$(AGENT_BINARY_NAME)" ]; then \
		echo "==> Skip $(AGENT_BINARY_NAME): is a directory"; \
	else \
		echo "==> Skip $(AGENT_BINARY_NAME): not found"; \
	fi
	@rm -rf $(DIST_DIR)
	@rm -rf $(USER_FRONTEND_DIR)/dist
	@echo "==> Clean complete"

# Database migration
migrate:
	$(GO) run ./cmd/mgpanel migrate

# Database backup
backup:
	$(GO) run ./cmd/mgpanel backup

# Install dependencies
deps:
	@echo "==> Installing Go dependencies..."
	$(GO) mod download
	@echo "==> Installing frontend dependencies..."
	cd $(USER_FRONTEND_DIR) && npm ci
	@echo "==> Dependencies installed"

# ========================================
# Agent Build Targets
# ========================================

# Build agent for current platform
agent:
	@echo "==> Building $(AGENT_BINARY_NAME) for current platform..."
	@mkdir -p $(DIST_DIR)
	@if [ -d "$(DIST_DIR)/$(AGENT_BINARY_NAME)" ]; then \
		echo "==> Error: $(DIST_DIR)/$(AGENT_BINARY_NAME) is a directory, cannot overwrite"; \
		exit 1; \
	fi
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(AGENT_BINARY_NAME) ./cmd/agent
	@echo "==> Build complete: ./$(DIST_DIR)/$(AGENT_BINARY_NAME)"

# Build agent for Linux
agent-linux:
	@mkdir -p $(DIST_DIR)
	@echo "==> Building agent for linux/amd64..."
	GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(AGENT_BINARY_NAME)-linux-amd64 ./cmd/agent
	@echo "==> Building agent for linux/arm64..."
	GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(AGENT_BINARY_NAME)-linux-arm64 ./cmd/agent
	@echo "==> Linux agent builds complete"

# Build agent for Linux (arm64 only)
agent-linux-arm64:
	@mkdir -p $(DIST_DIR)
	@echo "==> Building agent for linux/arm64..."
	GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(AGENT_BINARY_NAME)-linux-arm64 ./cmd/agent
	@echo "==> Linux arm64 agent build complete"

# Build agent for macOS
agent-darwin:
	@mkdir -p $(DIST_DIR)
	@echo "==> Building agent for darwin/amd64..."
	GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(AGENT_BINARY_NAME)-darwin-amd64 ./cmd/agent
	@echo "==> Building agent for darwin/arm64..."
	GOOS=darwin GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(AGENT_BINARY_NAME)-darwin-arm64 ./cmd/agent
	@echo "==> macOS agent builds complete"

# Build agent for macOS (arm64 only)
agent-darwin-arm64:
	@mkdir -p $(DIST_DIR)
	@echo "==> Building agent for darwin/arm64..."
	GOOS=darwin GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(AGENT_BINARY_NAME)-darwin-arm64 ./cmd/agent
	@echo "==> macOS arm64 agent build complete"

# Build agent for Windows
agent-windows:
	@mkdir -p $(DIST_DIR)
	@echo "==> Building agent for windows/amd64..."
	GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(AGENT_BINARY_NAME)-windows-amd64.exe ./cmd/agent
	@echo "==> Windows agent build complete"

# Build agent for all platforms
agent-all: agent-linux agent-darwin agent-windows
	@echo "==> All agent platform builds complete"

# Run agent in development mode
dev-agent:
	@echo "==> Starting agent..."
	$(GO) run ./cmd/agent --config agent_config.yml

# ========================================
# Proto Generation Targets
# ========================================

# Proto directories
PROTO_DIR := api/proto
PB_DIR := pkg/pb

# Generate protobuf code
.PHONY: proto
proto:
	@echo "==> Generating protobuf code..."
	protoc --go_out=$(PB_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(PB_DIR) --go-grpc_opt=paths=source_relative \
		-I $(PROTO_DIR) \
		$(PROTO_DIR)/agent/v1/*.proto
	@echo "==> Protobuf generation complete"

# Install protoc plugins
.PHONY: proto-deps
proto-deps:
	@echo "==> Installing protoc plugins..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "==> Protoc plugins installed"

# Clean generated protobuf files
.PHONY: proto-clean
proto-clean:
	@echo "==> Cleaning generated protobuf files..."
	rm -rf $(PB_DIR)/agent
	@echo "==> Proto clean complete"

# Generate TypeScript types from Go structs (config center inbound types)
gen-types:
	@echo "==> Generating TypeScript types from Go structs..."
	$(GO) run ./tools/gentypes/ -in internal/service/config_center_spec_validation.go,internal/template/unified.go,internal/template/types.go -out web/user-vite/src/types/_generated/inbound.ts -structs inboundSemanticSpec,XHTTPConfig,UnifiedInbound,UnifiedTransport,UnifiedTLS,UnifiedReality,UnifiedUser,UnifiedMultiplex,UnifiedBrutal,UnifiedSniffing
	@echo "==> Type generation complete"
