package tools

import (
	"log/slog"

	mcp_golang "github.com/metoro-io/mcp-golang"
)

// Base Request interface that all tool-specific requests must implement
type Request interface {
	Validate() error
}

// Tooler is the interface that all tools must implement
type Tooler interface {
	// Execute runs the tool with the given request and returns a response
	Execute(request Request) (*mcp_golang.ToolResponse, error)

	// Name returns the name of the tool
	Name() string

	// Description returns the description of the tool
	Description() string

	// SetLogger sets the logger for the tool
	SetLogger(logger *slog.Logger)
}
