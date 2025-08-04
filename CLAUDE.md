# OMDB Project Guide

## Special Instructions
- This project is a Model Control Protocol (MCP) server that provides access to The Open Movie Database
- Always maintain backward compatibility with the MCP protocol
- All tools must include comprehensive error handling and logging
- When responding to requests, prefer being concise and direct
- Use "server" terminology rather than "service" in documentation and code
- Never expose sensitive information in logs or responses

## Development Workflow
- Use the `go` command for all package management and operations
- Check if we're on the `main` branch at the start of a session; if so, confirm with the user whether to switch to or create a new branch
- Commit changes periodically to create snapshots and maintain history
- CRITICAL: The MCP server communicates via stdin/stdout - never log or write directly to stdout as this will disrupt the protocol
- All logging must be directed to stderr or files, never stdout

## Build Commands
- `make build` - Build the application
- `make run` - Build and run the application
- `make test` - Run all tests
- `make clean` - Clean build artifacts
- `make deps` - Install dependencies
- `go test ./...` - Run all tests
- `go test ./internal/tools/now/...` - Run tests for a specific package
- `go test -v -run TestName ./internal/...` - Run a specific test

## Code Style Guidelines
- **Imports**: Group standard library first, then external packages, then internal packages
- **Formatting**: Follow standard Go formatting with `gofmt`
- **Types**: Define types at package level with descriptive comments
- **Naming**: Use CamelCase for exported identifiers, camelCase for unexported
- **Error Handling**: Check errors explicitly, return early, and log appropriately with slog
- **Configuration**: Use functional options pattern for configurable components
- **Logging**: Use structured logging with slog package
- **Environment**: Load environment variables with godotenv, default values where appropriate
- **Testing**: Write unit tests for all public functions

## Project Structure
- `main.go` - Application entry point
- `internal/` - Private implementation packages
- `internal/logging/` - Logging utilities
- `internal/tools/` - MCP tool implementations
