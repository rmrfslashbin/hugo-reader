.PHONY: all build clean test test-coverage lint run deps build-all build-darwin-amd64 build-darwin-arm64 build-linux-amd64 build-linux-arm64 build-windows-amd64 build-windows-arm64 build-freebsd-amd64 build-linux-armv7

# Binary name and directory
BINARY=hugo-reader
BINDIR=bin

# Build information
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -ldflags "-X github.com/rmrfslashbin/mcp/hugo-reader/cmd/hugo.GitCommit=$(COMMIT) -X github.com/rmrfslashbin/mcp/hugo-reader/cmd/hugo.Version=$(VERSION) -X github.com/rmrfslashbin/mcp/hugo-reader/cmd/hugo.BuildTime=$(BUILD_TIME)"

# Default target
all: build

# Build the application
build:
	go build $(LDFLAGS) -o $(BINDIR)/$(BINARY)

# Clean build artifacts
clean:
	go clean
	rm -rf $(BINDIR) coverage.out coverage.html

# Run tests
test:
	go test -v -race ./...

# Run tests with coverage
test-coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run linters (requires golangci-lint)
lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run

# Run the application
run:
	./$(BINDIR)/$(BINARY) server $(ARGS)

# Install dependencies
deps:
	go mod download
	go mod tidy

# Cross compilation targets
build-all: build-darwin-amd64 build-darwin-arm64 build-linux-amd64 build-linux-arm64 build-windows-amd64 build-windows-arm64 build-freebsd-amd64 build-linux-armv7

build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BINDIR)/$(BINARY)_darwin_amd64

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BINDIR)/$(BINARY)_darwin_arm64

build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINDIR)/$(BINARY)_linux_amd64

build-linux-arm64:
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BINDIR)/$(BINARY)_linux_arm64

build-windows-amd64:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BINDIR)/$(BINARY)_windows_amd64.exe

build-windows-arm64:
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o $(BINDIR)/$(BINARY)_windows_arm64.exe

build-freebsd-amd64:
	GOOS=freebsd GOARCH=amd64 go build $(LDFLAGS) -o $(BINDIR)/$(BINARY)_freebsd_amd64

build-linux-armv7:
	GOOS=linux GOARCH=arm GOARM=7 go build $(LDFLAGS) -o $(BINDIR)/$(BINARY)_linux_armv7
