.PHONY: build run test clean benchmark

# Variables
APP_NAME=ws-pub-sub
GO_FILES=$(shell find . -type f -name "*.go")
BENCHMARK_TOOL=tools/benchmark.go

# Build the application
build:
	@echo "Building $(APP_NAME)..."
	@go build -o $(APP_NAME) .

# Run the application
run: build
	@echo "Running $(APP_NAME)..."
	@./$(APP_NAME)

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go get -u github.com/gorilla/websocket

# Test the application
test:
	@echo "Running tests..."
	@go test -v ./...

# Run the benchmark tool
benchmark: build
	@echo "Building benchmark tool..."
	@go build -o benchmark $(BENCHMARK_TOOL)
	@echo "Running benchmark..."
	@./benchmark

# Clean build artifacts
clean:
	@echo "Cleaning up..."
	@rm -f $(APP_NAME) benchmark

# Show help
help:
	@echo "Available targets:"
	@echo "  build     - Build the application"
	@echo "  run       - Run the application"
	@echo "  deps      - Install dependencies"
	@echo "  test      - Run tests"
	@echo "  benchmark - Run the benchmark tool"
	@echo "  clean     - Remove build artifacts"
	@echo "  help      - Show this help message"

# Default target
default: build
