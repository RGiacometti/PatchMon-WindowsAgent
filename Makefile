# Makefile for PatchMon Windows Agent

# Build variables
BINARY_NAME=patchmon-agent.exe
BUILD_DIR=build
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-s -w -X patchmon-agent/internal/version.Version=$(VERSION)"
# Disable VCS stamping
BUILD_FLAGS=-buildvcs=false

# Go variables
GOBASE=$(shell pwd)
GOBIN=$(GOBASE)/$(BUILD_DIR)
GO_CMD=go

# Windows install paths
INSTALL_DIR=C:\Program Files\PatchMon
CONFIG_DIR=C:\ProgramData\PatchMon
LOG_DIR=C:\ProgramData\PatchMon\logs

# Default target
.PHONY: all
all: build

# Build the application (Windows amd64)
.PHONY: build
build:
	@echo "Building $(BINARY_NAME) (version: $(VERSION))..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=windows GOARCH=amd64 CGO_ENABLED=0 $(GO_CMD) build $(BUILD_FLAGS) $(LDFLAGS) -o $(GOBIN)/$(BINARY_NAME) ./cmd/patchmon-agent

# Build for multiple Windows architectures
.PHONY: build-all
build-all:
	@echo "Building for multiple Windows architectures (version: $(VERSION))..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=windows GOARCH=amd64 CGO_ENABLED=0 $(GO_CMD) build $(BUILD_FLAGS) $(LDFLAGS) -o $(GOBIN)/patchmon-agent-windows-amd64.exe ./cmd/patchmon-agent
	@GOOS=windows GOARCH=arm64 CGO_ENABLED=0 $(GO_CMD) build $(BUILD_FLAGS) $(LDFLAGS) -o $(GOBIN)/patchmon-agent-windows-arm64.exe ./cmd/patchmon-agent
	@GOOS=windows GOARCH=386 CGO_ENABLED=0 $(GO_CMD) build $(BUILD_FLAGS) $(LDFLAGS) -o $(GOBIN)/patchmon-agent-windows-386.exe ./cmd/patchmon-agent

# Install binary to Program Files (requires Administrator)
.PHONY: install
install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_DIR)..."
	@echo "NOTE: This target requires Administrator privileges."
	@if not exist "$(INSTALL_DIR)" mkdir "$(INSTALL_DIR)"
	@copy /Y "$(GOBIN)\$(BINARY_NAME)" "$(INSTALL_DIR)\$(BINARY_NAME)"
	@echo "Installed to $(INSTALL_DIR)\$(BINARY_NAME)"

# Create configuration directory structure and sample config
.PHONY: config-init
config-init:
	@echo "Creating configuration directory structure..."
	@echo "NOTE: This target requires Administrator privileges."
	@if not exist "$(CONFIG_DIR)" mkdir "$(CONFIG_DIR)"
	@if not exist "$(LOG_DIR)" mkdir "$(LOG_DIR)"
	@echo --- > "$(CONFIG_DIR)\config.yml.sample"
	@echo patchmon_server: "https://your-patchmon-server.com" >> "$(CONFIG_DIR)\config.yml.sample"
	@echo api_version: "v1" >> "$(CONFIG_DIR)\config.yml.sample"
	@echo credentials_file: "C:\\ProgramData\\PatchMon\\credentials.yml" >> "$(CONFIG_DIR)\config.yml.sample"
	@echo log_file: "C:\\ProgramData\\PatchMon\\logs\\patchmon-agent.log" >> "$(CONFIG_DIR)\config.yml.sample"
	@echo log_level: "info" >> "$(CONFIG_DIR)\config.yml.sample"
	@echo skip_ssl_verify: false >> "$(CONFIG_DIR)\config.yml.sample"
	@echo update_interval: 60 >> "$(CONFIG_DIR)\config.yml.sample"
	@echo --- > "$(CONFIG_DIR)\credentials.yml.sample"
	@echo api_id: "your-api-id" >> "$(CONFIG_DIR)\credentials.yml.sample"
	@echo api_key: "your-api-key" >> "$(CONFIG_DIR)\credentials.yml.sample"
	@echo "Sample configuration files created in $(CONFIG_DIR)"

# Install dependencies
.PHONY: deps
deps:
	@echo "Installing dependencies..."
	@$(GO_CMD) mod download
	@$(GO_CMD) mod tidy

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	@$(GO_CMD) test -v ./...

# Run tests in short mode (skip integration tests)
.PHONY: test-short
test-short:
	@echo "Running tests (short mode)..."
	@$(GO_CMD) test -short -v ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	@$(GO_CMD) test -v -coverprofile=coverage.out ./...
	@$(GO_CMD) tool cover -html=coverage.out -o coverage.html

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	@$(GO_CMD) fmt ./...

# Lint code (using go vet)
.PHONY: lint
lint:
	@echo "Linting code..."
	@$(GO_CMD) vet ./...

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html

# Show help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build         Build the application (Windows amd64)"
	@echo "  build-all     Build for multiple Windows architectures (amd64, arm64, 386)"
	@echo "  install       Install binary to C:\Program Files\PatchMon (requires Admin)"
	@echo "  config-init   Create config directory structure and sample files (requires Admin)"
	@echo "  deps          Install dependencies"
	@echo "  test          Run tests"
	@echo "  test-short    Run tests in short mode (skip integration tests)"
	@echo "  test-coverage Run tests with coverage"
	@echo "  fmt           Format code"
	@echo "  lint          Lint code (go vet)"
	@echo "  clean         Clean build artifacts"
	@echo "  help          Show this help message"
	@echo ""
