.PHONY: all build clean test run deps build-all build-darwin-amd64 build-darwin-arm64 build-linux-amd64 build-linux-arm64 build-windows-amd64 build-windows-arm64 build-freebsd-amd64 build-linux-armv7

# Binary name and directory
BINARY=hugo-reader
BINDIR=bin

# Default target
all: build

# Build the application
build:
	go build -ldflags "-X github.com/rmrfslashbin/mcp/hugo-reader/cmd/hugo.GitCommit=$(shell git rev-parse --short HEAD)" -o $(BINDIR)/$(BINARY)

# Clean build artifacts
clean:
	go clean
	rm -rf $(BINDIR)

# Run tests
test:
	go test -v ./...

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
	GOOS=darwin GOARCH=amd64 go build -ldflags "-X github.com/rmrfslashbin/mcp/hugo-reader/cmd/hugo.GitCommit=$(shell git rev-parse --short HEAD)" -o $(BINDIR)/$(BINARY)_darwin_amd64

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -ldflags "-X github.com/rmrfslashbin/mcp/hugo-reader/cmd/hugo.GitCommit=$(shell git rev-parse --short HEAD)" -o $(BINDIR)/$(BINARY)_darwin_arm64

build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build -ldflags "-X github.com/rmrfslashbin/mcp/hugo-reader/cmd/hugo.GitCommit=$(shell git rev-parse --short HEAD)" -o $(BINDIR)/$(BINARY)_linux_amd64

build-linux-arm64:
	GOOS=linux GOARCH=arm64 go build -ldflags "-X github.com/rmrfslashbin/mcp/hugo-reader/cmd/hugo.GitCommit=$(shell git rev-parse --short HEAD)" -o $(BINDIR)/$(BINARY)_linux_arm64

build-windows-amd64:
	GOOS=windows GOARCH=amd64 go build -ldflags "-X github.com/rmrfslashbin/mcp/hugo-reader/cmd/hugo.GitCommit=$(shell git rev-parse --short HEAD)" -o $(BINDIR)/$(BINARY)_windows_amd64.exe

build-windows-arm64:
	GOOS=windows GOARCH=arm64 go build -ldflags "-X github.com/rmrfslashbin/mcp/hugo-reader/cmd/hugo.GitCommit=$(shell git rev-parse --short HEAD)" -o $(BINDIR)/$(BINARY)_windows_arm64.exe

build-freebsd-amd64:
	GOOS=freebsd GOARCH=amd64 go build -ldflags "-X github.com/rmrfslashbin/mcp/hugo-reader/cmd/hugo.GitCommit=$(shell git rev-parse --short HEAD)" -o $(BINDIR)/$(BINARY)_freebsd_amd64

build-linux-armv7:
	GOOS=linux GOARCH=arm GOARM=7 go build -ldflags "-X github.com/rmrfslashbin/mcp/hugo-reader/cmd/hugo.GitCommit=$(shell git rev-parse --short HEAD)" -o $(BINDIR)/$(BINARY)_linux_armv7
