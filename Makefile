.PHONY: build build-all test clean help

# Application name
APP_NAME := test-branch-manager
MODULE := test-branch-creator
CMD := ./cmd/test-branch-manager

# Version (from git tag or default)
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

# Build directory
DIST := dist

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build        Build for the current platform"
	@echo "  build-all    Build for all supported platforms"
	@echo "  test         Run all tests"
	@echo "  test-v       Run all tests with verbose output"
	@echo "  coverage     Run tests with coverage report"
	@echo "  clean        Remove build artifacts"
	@echo "  lint         Run go vet"
	@echo ""

## build: Build for the current platform
build:
	go build $(LDFLAGS) -o $(DIST)/$(APP_NAME) $(CMD)

## build-all: Build for all supported platforms
build-all: clean
	@mkdir -p $(DIST)
	@echo "Building for Windows (amd64)..."
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(DIST)/$(APP_NAME)-windows-amd64.exe $(CMD)
	@echo "Building for macOS (amd64)..."
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(DIST)/$(APP_NAME)-darwin-amd64 $(CMD)
	@echo "Building for macOS (arm64)..."
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(DIST)/$(APP_NAME)-darwin-arm64 $(CMD)
	@echo "Building for Linux (amd64)..."
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(DIST)/$(APP_NAME)-linux-amd64 $(CMD)
	@echo "All builds complete. Artifacts in $(DIST)/"
	@ls -la $(DIST)/

## test: Run all tests
test:
	go test ./...

## test-v: Run all tests with verbose output
test-v:
	go test -v ./...

## coverage: Run tests with coverage report
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## lint: Run static analysis
lint:
	go vet ./...

## clean: Remove build artifacts
clean:
	rm -rf $(DIST)
	rm -f coverage.out coverage.html
