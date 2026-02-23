# TaskWing Makefile
# Build, test, and automation targets

# Variables
BINARY_NAME=taskwing
BUILD_DIR=.
TEST_DIR=./test-results
COVERAGE_FILE=$(TEST_DIR)/coverage.out
COVERAGE_HTML=$(TEST_DIR)/coverage.html
CORE_PKGS=./ ./cmd/... ./internal/...

# Version from git tags (falls back to "dev" if not in a git repo)
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X github.com/josephgoksu/TaskWing/cmd.version=$(VERSION)

# Use local workspace cache/temp to work in sandboxed environments
GOENV := GOCACHE=$(PWD)/$(TEST_DIR)/go-build GOTMPDIR=$(PWD)/$(TEST_DIR)/tmp
GO := env $(GOENV) go

# Default target
.PHONY: all
all: clean build test

# Build the TaskWing binary
.PHONY: build
build:
	@echo "🔨 Building TaskWing ($(VERSION))..."
	mkdir -p $(TEST_DIR) $(TEST_DIR)/go-build $(TEST_DIR)/tmp
	$(GO) generate $(CORE_PKGS)
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) main.go
	@echo "✅ Build complete: $(BINARY_NAME) ($(VERSION))"

# Clean build artifacts
.PHONY: clean
clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -f $(BINARY_NAME)
	rm -rf $(TEST_DIR)
	# Avoid cleaning global cache to respect sandbox
	@echo "✅ Clean complete"

# Run all tests
.PHONY: test
test: test-unit test-integration test-mcp

# Run unit tests
.PHONY: test-unit
test-unit:
	@echo "🧪 Running unit tests..."
	mkdir -p $(TEST_DIR)
	$(GO) test -v $(CORE_PKGS) | tee $(TEST_DIR)/unit-tests.log
	@echo "✅ Unit tests complete"

# Run integration tests
.PHONY: test-integration
test-integration: build
	@echo "🔧 Running integration tests..."
	mkdir -p $(TEST_DIR)
	$(GO) test -v ./tests/integration/... | tee $(TEST_DIR)/integration-tests.log
	@echo "✅ Integration tests complete"

# Run MCP tools tests
.PHONY: test-mcp
test-mcp: build
	@echo "🎯 Running MCP protocol and functional tests..."
	mkdir -p $(TEST_DIR)
	$(GO) test -v ./cmd -run "TestMCP.*" | tee $(TEST_DIR)/mcp-protocol.log
	@echo "✅ MCP protocol and functional tests complete"

# Run comprehensive MCP functional tests (all tools) - same as test-mcp smoke tests
.PHONY: test-mcp-functional
test-mcp-functional: test-mcp
	@echo "✅ MCP functional tests complete (using test-mcp)"

# Run OpenCode integration tests
# IMPORTANT: Uses local binary (./bin/taskwing or make build), NOT system-installed taskwing
.PHONY: test-opencode
test-opencode: build
	@echo "🎯 Running OpenCode integration tests..."
	mkdir -p $(TEST_DIR)
	$(GO) test -v ./tests/integration/... -run "TestOpenCode" | tee $(TEST_DIR)/opencode-integration.log
	@echo "✅ OpenCode integration tests complete"


# Generate test coverage
.PHONY: coverage
coverage:
	@echo "📊 Generating test coverage..."
	mkdir -p $(TEST_DIR)
	$(GO) test -coverprofile=$(COVERAGE_FILE) $(CORE_PKGS)
	go tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	go tool cover -func=$(COVERAGE_FILE) | grep "total:" | tee $(TEST_DIR)/coverage-summary.txt
	@echo "✅ Coverage report generated: $(COVERAGE_HTML)"

# Run linting and formatting
.PHONY: lint
lint:
	@echo "🔍 Running linting and formatting..."
	mkdir -p $(TEST_DIR) $(TEST_DIR)/go-build $(TEST_DIR)/tmp
	$(GO) fmt $(CORE_PKGS)
	@echo "🔍 Running go vet..."
	$(GO) vet ./...
	@echo "🔍 Running staticcheck..."
	@if command -v staticcheck >/dev/null 2>&1; then \
		staticcheck ./...; \
	else \
		echo "⚠️  staticcheck not installed, installing..."; \
		go install honnef.co/go/tools/cmd/staticcheck@latest; \
		staticcheck ./...; \
	fi
	@if [ -n "$(SKIP_GOLANGCI)" ]; then \
		echo "⏭️  SKIP_GOLANGCI set; skipping golangci-lint"; \
	elif command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "⚠️  golangci-lint not installed, skipping linting"; \
	fi
	@echo "✅ Linting complete"

# Run comprehensive test suite
.PHONY: test-all
test-all: clean build lint coverage test-integration test-mcp
	@echo "🎊 All tests complete!"
	@echo "📄 Check $(TEST_DIR)/ for detailed results"

# Quick test (faster version for development)
.PHONY: test-quick
test-quick: build
	@echo "⚡ Running quick tests..."
	mkdir -p $(TEST_DIR)
	$(GO) test $(CORE_PKGS)
	$(GO) test -v ./cmd -run "TestMCPProtocolStdio"
	@echo "✅ Quick tests complete"

# Development setup
.PHONY: dev-setup
dev-setup:
	@echo "🛠️  Setting up development environment..."
	@go version | grep -E "go1\.2[4-9]" >/dev/null || echo "⚠️  Warning: Go 1.24+ recommended"
	@if [ ! -f .env ] && [ -f example.env ]; then \
		echo "📄 Copying example.env to .env..."; \
		cp example.env .env; \
	fi
	$(GO) mod tidy
	$(GO) generate $(CORE_PKGS)
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "📦 Installing golangci-lint from source..."; \
		$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.61.0; \
	fi
	@echo "✅ Development setup complete"

# Release snapshot (local testing)
.PHONY: release-snapshot
release-snapshot: clean lint test-all
	@echo "🚀 Building release snapshot..."
	@if command -v goreleaser >/dev/null 2>&1; then \
		goreleaser build --snapshot --clean; \
	else \
		go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) main.go; \
	fi
	@echo "✅ Release snapshot complete"

# Interactive release (tag and push)
.PHONY: release
release:
	@echo "🚀 Starting release process..."
	@./scripts/release.sh

# Install for local use
.PHONY: install
install: build
	@echo "📦 Installing TaskWing locally..."
	cp $(BINARY_NAME) $(HOME)/.local/bin/$(BINARY_NAME)
	@echo "✅ TaskWing installed to $(HOME)/.local/bin/$(BINARY_NAME)"

# Uninstall
.PHONY: uninstall
uninstall:
	@echo "🗑️  Uninstalling TaskWing..."
	rm -f $(HOME)/.local/bin/$(BINARY_NAME)
	@echo "✅ TaskWing uninstalled"

# ─── Proto / gRPC ───────────────────────────────────────────────────────────

# Generate Go code from proto definitions
.PHONY: proto-generate
proto-generate:
	@echo "🔧 Generating Go code from proto definitions..."
	cd proto && buf generate
	@echo "✅ Proto generation complete: gen/go/taskwing/v1/"

# Lint proto files
.PHONY: proto-lint
proto-lint:
	@echo "🔍 Linting proto files..."
	cd proto && buf lint
	@echo "✅ Proto lint passed"

# Detect breaking changes vs main branch
.PHONY: proto-breaking
proto-breaking:
	@echo "🔍 Checking for breaking proto changes..."
	cd proto && buf breaking --against '../.git#subdir=proto'
	@echo "✅ No breaking changes detected"

# Run MCP server for testing
.PHONY: mcp-server
mcp-server: build
	@echo "🖥️  Starting MCP server (use Ctrl+C to stop)..."
	./$(BINARY_NAME) mcp -v

# Show help
.PHONY: help
help:
	@echo "TaskWing Makefile Commands:"
	@echo ""
	@echo "Build Commands:"
	@echo "  build            - Build the TaskWing binary"
	@echo "  clean            - Clean build artifacts"
	@echo "  release          - Interactive release (bump version, tag, push)"
	@echo "  release-snapshot - Build release snapshot locally (no publish)"
	@echo ""
	@echo "Test Commands:"
		@echo "  test        - Run all tests (unit, integration, MCP)"
		@echo "  test-unit   - Run unit tests only"
		@echo "  test-integration - Run integration tests"
		@echo "  test-mcp    - Run MCP protocol tests (JSON-RPC stdio)"
		@echo "  test-opencode - Run OpenCode integration tests"
		@echo "  test-quick  - Run quick tests for development"
		@echo "  test-all    - Run comprehensive test suite"
	@echo ""
	@echo "Quality Commands:"
	@echo "  lint        - Run linting and formatting"
	@echo "  coverage    - Generate test coverage report"
	@echo ""
	@echo "Proto Commands:"
	@echo "  proto-generate - Generate Go code from proto definitions"
	@echo "  proto-lint     - Lint proto files"
	@echo "  proto-breaking - Detect breaking proto changes vs main"
	@echo ""
	@echo "Development Commands:"
	@echo "  dev-setup   - Setup development environment"
	@echo "  mcp-server  - Start MCP server for testing"
	@echo ""
	@echo "Install Commands:"
	@echo "  install     - Install TaskWing locally"
	@echo "  uninstall   - Remove local installation"
	@echo ""
	@echo "Results are saved in: $(TEST_DIR)/"

# Default help if no target specified
.DEFAULT_GOAL := help
