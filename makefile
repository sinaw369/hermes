# Makefile for the GitSyncer Go application

# Go parameters
GO        := go
APP_NAME  := GitSyncer
SRC_DIR   := .
BIN_DIR   := bin
BUILD_DIR := build

.PHONY: all build run clean test fmt lint deps build-linux build-windows build-darwin build-all

all: build

# Build the application
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/$(APP_NAME) $(SRC_DIR)/main.go

# Run the application
run: build
	@echo "Running $(APP_NAME)..."
	./$(BIN_DIR)/$(APP_NAME)

# Clean build artifacts
clean:
	@echo "Cleaning up..."
	@rm -rf $(BIN_DIR)
	@rm -rf $(BUILD_DIR)

# Run tests
test:
	@echo "Running tests..."
	$(GO) test ./...

# Install dependencies
deps:
	@echo "Installing dependencies..."
	$(GO) mod download

# Format code
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...

# Lint code
lint:
	@echo "Linting code..."
	@golangci-lint run || echo "Please install golangci-lint for linting"

# Generate code (if needed)
generate:
	@echo "Generating code..."
	$(GO) generate ./...

# Build for Linux
build-linux:
	@echo "Building $(APP_NAME) for Linux..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(APP_NAME)_linux_amd64 $(SRC_DIR)/main.go

# Build for Windows
build-windows:
	@echo "Building $(APP_NAME) for Windows..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(APP_NAME)_windows_amd64.exe $(SRC_DIR)/main.go

# Build for macOS
build-darwin:
	@echo "Building $(APP_NAME) for macOS..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(APP_NAME)_darwin_amd64 $(SRC_DIR)/main.go

# Cross-compile for all platforms
build-all: build-linux build-windows build-darwin
