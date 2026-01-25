# cloudcoop Makefile
# Build and development automation for the cloudcoop TUI

# Variables
BINARY_NAME := cloudcoop
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

# Coverage settings
# Start with 40% threshold, increase as test coverage improves
COVERAGE_THRESHOLD := 40

# Go build settings
GO := go
GOFLAGS :=
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)"

# Output directories
BIN_DIR := bin
COVERAGE_DIR := coverage

# Source files
SRC := $(shell find . -name '*.go' -type f 2>/dev/null)

# Default target
.DEFAULT_GOAL := all

# Phony targets
.PHONY: all build test test-coverage test-coverage-check lint fmt clean install help

## all: Build and run tests
all: fmt lint test build

## build: Compile the binary
build: $(BIN_DIR)/$(BINARY_NAME)

$(BIN_DIR)/$(BINARY_NAME): $(SRC)
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) ./...

## test: Run all tests
test:
	$(GO) test $(GOFLAGS) ./...

## test-coverage: Run tests with coverage report
test-coverage:
	@mkdir -p $(COVERAGE_DIR)
	$(GO) test $(GOFLAGS) -race -coverprofile=$(COVERAGE_DIR)/coverage.out -covermode=atomic ./...
	$(GO) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo "Coverage report: $(COVERAGE_DIR)/coverage.html"
	$(GO) tool cover -func=$(COVERAGE_DIR)/coverage.out

## test-coverage-check: Run tests with coverage and enforce threshold
test-coverage-check: test-coverage
	@echo "Checking coverage threshold ($(COVERAGE_THRESHOLD)%)..."
	@COVERAGE=$$($(GO) tool cover -func=$(COVERAGE_DIR)/coverage.out | grep total | awk '{print $$3}' | tr -d '%'); \
	if [ -z "$$COVERAGE" ]; then \
		echo "Warning: No coverage data found (no tests yet?)"; \
		exit 0; \
	elif [ $$(echo "$$COVERAGE < $(COVERAGE_THRESHOLD)" | bc -l) -eq 1 ]; then \
		echo "FAIL: Coverage $$COVERAGE% is below threshold $(COVERAGE_THRESHOLD)%"; \
		exit 1; \
	else \
		echo "OK: Coverage $$COVERAGE% meets threshold $(COVERAGE_THRESHOLD)%"; \
	fi

## lint: Run golangci-lint
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi

## fmt: Format Go source files
fmt:
	gofmt -w -s .
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	fi

## clean: Remove build artifacts
clean:
	rm -rf $(BIN_DIR)
	rm -rf $(COVERAGE_DIR)
	$(GO) clean -cache -testcache

## install: Install binary to GOPATH/bin
install:
	$(GO) install $(GOFLAGS) $(LDFLAGS) ./...

## help: Show this help message
help:
	@echo "cloudcoop Makefile targets:"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /' | column -t -s ':'
	@echo ""
	@echo "Variables:"
	@echo "  BINARY_NAME         = $(BINARY_NAME)"
	@echo "  VERSION             = $(VERSION)"
	@echo "  COMMIT              = $(COMMIT)"
	@echo "  BIN_DIR             = $(BIN_DIR)"
	@echo "  COVERAGE_THRESHOLD  = $(COVERAGE_THRESHOLD)%"
