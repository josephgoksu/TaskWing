# TaskWing Makefile
# Build, test, and automation targets

# Variables
BINARY_NAME=taskwing
BUILD_DIR=.
TEST_DIR=./test-results
COVERAGE_FILE=$(TEST_DIR)/coverage.out
COVERAGE_HTML=$(TEST_DIR)/coverage.html

# Default target
.PHONY: all
all: clean build test

# Build the TaskWing binary
.PHONY: build
build:
	@echo "🔨 Building TaskWing..."
	go generate ./...
	go build -o $(BINARY_NAME) main.go
	@echo "✅ Build complete: $(BINARY_NAME)"

# Clean build artifacts
.PHONY: clean
clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -f $(BINARY_NAME)
	rm -rf $(TEST_DIR)
	go clean -cache
	@echo "✅ Clean complete"

# Run all tests
.PHONY: test
test: test-unit test-integration test-mcp

# Run unit tests
.PHONY: test-unit
test-unit:
	@echo "🧪 Running unit tests..."
	mkdir -p $(TEST_DIR)
	go test -v ./... | tee $(TEST_DIR)/unit-tests.log
	@echo "✅ Unit tests complete"

# Run integration tests
.PHONY: test-integration
test-integration: build
	@echo "🔧 Running integration tests..."
	mkdir -p $(TEST_DIR)
	go test -v ./cmd -run "TestMCP|TestTaskWing|TestBasic" | tee $(TEST_DIR)/integration-tests.log
	@echo "✅ Integration tests complete"

# Run MCP tools tests
.PHONY: test-mcp
test-mcp: build
	@echo "🎯 Running MCP protocol and functional tests..."
	mkdir -p $(TEST_DIR)
	go test -v ./cmd -run "TestMCP.*" | tee $(TEST_DIR)/mcp-protocol.log
	@echo "✅ MCP protocol and functional tests complete"

# Run comprehensive MCP functional tests (all tools)
.PHONY: test-mcp-functional
test-mcp-functional: build  
	@echo "🔧 Running comprehensive MCP functional tests..."
	mkdir -p $(TEST_DIR)
	go test -v ./cmd -run "TestMCPAllToolsSmoke" | tee $(TEST_DIR)/mcp-functional.log
	@echo "✅ MCP functional tests complete"


# Generate test coverage
.PHONY: coverage
coverage:
	@echo "📊 Generating test coverage..."
	mkdir -p $(TEST_DIR)
	go test -coverprofile=$(COVERAGE_FILE) ./...
	go tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	go tool cover -func=$(COVERAGE_FILE) | grep "total:" | tee $(TEST_DIR)/coverage-summary.txt
	@echo "✅ Coverage report generated: $(COVERAGE_HTML)"

# Run linting and formatting
.PHONY: lint
lint:
	@echo "🔍 Running linting and formatting..."
	go fmt ./...
	@if command -v golangci-lint >/dev/null 2>&1; then \
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
	go test ./...
	go test -v ./cmd -run "TestMCPProtocolStdio"
	@echo "✅ Quick tests complete"

# Development setup
.PHONY: dev-setup
dev-setup:
	@echo "🛠️  Setting up development environment..."
	go mod tidy
	go generate ./...
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "📦 Installing golangci-lint..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v1.54.2; \
	fi
	@echo "✅ Development setup complete"

# Release build
.PHONY: release
release: clean lint test-all
	@echo "🚀 Building release version..."
	@if command -v goreleaser >/dev/null 2>&1; then \
		goreleaser build --snapshot --clean; \
	else \
		go build -ldflags="-s -w" -o $(BINARY_NAME) main.go; \
	fi
	@echo "✅ Release build complete"

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
	@echo "  build       - Build the TaskWing binary"
	@echo "  clean       - Clean build artifacts"
	@echo "  release     - Build release version with optimizations"
	@echo ""
	@echo "Test Commands:"
		@echo "  test        - Run all tests (unit, integration, MCP)"
		@echo "  test-unit   - Run unit tests only"
		@echo "  test-integration - Run integration tests"
		@echo "  test-mcp    - Run MCP protocol tests (JSON-RPC stdio)"
		@echo "  test-quick  - Run quick tests for development"
		@echo "  test-all    - Run comprehensive test suite"
	@echo ""
	@echo "Quality Commands:"
	@echo "  lint        - Run linting and formatting"
	@echo "  coverage    - Generate test coverage report"
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
