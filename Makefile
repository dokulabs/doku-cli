.PHONY: help build install clean test lint fmt run dev deps

# Variables
BINARY_NAME=doku
VERSION?=dev
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildDate=${BUILD_DATE}"

# Default target
help:
	@echo "Doku CLI - Build Commands"
	@echo ""
	@echo "Usage:"
	@echo "  make build          Build the binary"
	@echo "  make install        Install the binary to GOPATH/bin"
	@echo "  make run            Run the application"
	@echo "  make dev            Run in development mode with hot reload"
	@echo "  make test           Run tests"
	@echo "  make test-coverage  Run tests with coverage"
	@echo "  make lint           Run linter"
	@echo "  make fmt            Format code"
	@echo "  make clean          Clean build artifacts"
	@echo "  make deps           Download dependencies"
	@echo ""

# Build the binary
build:
	@echo "Building ${BINARY_NAME}..."
	@mkdir -p bin
	go build ${LDFLAGS} -o bin/${BINARY_NAME} ./cmd/doku
	@echo "Build complete: bin/${BINARY_NAME}"

# Install to GOPATH/bin
install:
	@echo "Installing ${BINARY_NAME}..."
	go install ${LDFLAGS} ./cmd/doku
	@echo "Installed to $(shell go env GOPATH)/bin/${BINARY_NAME}"

# Run the application
run:
	go run ${LDFLAGS} ./cmd/doku $(ARGS)

# Development mode (requires air: go install github.com/cosmtrek/air@latest)
dev:
	@which air > /dev/null || (echo "Installing air..." && go install github.com/cosmtrek/air@latest)
	air

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.txt -covermode=atomic ./...
	go tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report: coverage.html"

# Run linter (requires golangci-lint)
lint:
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	@echo "Code formatted"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -rf dist/
	rm -f coverage.txt coverage.html
	go clean
	@echo "Clean complete"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy
	@echo "Dependencies ready"

# Build for all platforms
build-all:
	@echo "Building for all platforms..."
	@mkdir -p dist
	GOOS=darwin GOARCH=amd64 go build ${LDFLAGS} -o dist/${BINARY_NAME}-darwin-amd64 ./cmd/doku
	GOOS=darwin GOARCH=arm64 go build ${LDFLAGS} -o dist/${BINARY_NAME}-darwin-arm64 ./cmd/doku
	GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o dist/${BINARY_NAME}-linux-amd64 ./cmd/doku
	GOOS=linux GOARCH=arm64 go build ${LDFLAGS} -o dist/${BINARY_NAME}-linux-arm64 ./cmd/doku
	GOOS=windows GOARCH=amd64 go build ${LDFLAGS} -o dist/${BINARY_NAME}-windows-amd64.exe ./cmd/doku
	@echo "Cross-platform builds complete"
