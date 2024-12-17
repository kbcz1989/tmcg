# Variables
APP_NAME := tmcg
SRC_DIR := ./cmd/tmcg
BUILD_DIR := ./bin
TERRAFORM_DIR := ./terraform
GOFLAGS := -v
PLATFORMS := linux/amd64 linux/arm64 windows/amd64 windows/arm64 darwin/amd64 darwin/arm64
OUTPUT_DIR := build
VERSION := $(shell git describe --tags --always --dirty)
COMMIT := $(shell git rev-parse --short HEAD)
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

# Default target is to build the app
.PHONY: all
all: clean build

# Build target
.PHONY: build
build:
	@echo "Building the application..."
	@mkdir -p $(BUILD_DIR)
	mkdir -p $(OUTPUT_DIR)
	for platform in $(PLATFORMS); do \
		EXT=""; \
		if [ $${platform%/*} = "windows" ]; then EXT=".exe"; fi; \
		GOOS=$${platform%/*} GOARCH=$${platform#*/} go build -ldflags="-X 'main.version=$(VERSION)' -X 'main.commit=$(COMMIT)' -X 'main.buildDate=$(BUILD_DATE)'" -o $(OUTPUT_DIR)/$(APP_NAME)-$${platform%/*}-$${platform#*/}$$EXT $(SRC_DIR); \
	done

# Clean target to remove build artifacts and the terraform directory
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@if [ -d $(BUILD_DIR) ]; then rm -vrf $(BUILD_DIR); fi
	@if [ -d $(TERRAFORM_DIR) ]; then rm -vrf $(TERRAFORM_DIR); fi
	@if [ -d $(OUTPUT_DIR) ]; then rm -vrf $(OUTPUT_DIR); fi

# Run target to run the built binary
.PHONY: run
run: build
	@echo "Running the application..."
	$(BUILD_DIR)/$(APP_NAME)

# Test target to run unit tests
.PHONY: test
test:
	@echo "Running tests..."
	gotestsum --format=short-verbose --junitfile=unit-test-report.xml ./...

# Lint target to check code with golangci-lint
.PHONY: lint
lint:
	@echo "Linting code..."
	@golangci-lint run ./...

# Format target to ensure code style
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Help target to list available commands
.PHONY: help
help:
	@echo "Makefile targets:"
	@echo "  all    - Build the application (default)."
	@echo "  build  - Build the application."
	@echo "  clean  - Clean the build artifacts and terraform directory."
	@echo "  run    - Run the application."
	@echo "  test   - Run tests."
	@echo "  lint   - Run linter."
	@echo "  fmt    - Format code."
	@echo "  help   - Show this help message."

