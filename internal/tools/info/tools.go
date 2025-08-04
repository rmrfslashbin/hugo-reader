package info

import (
	"fmt"
	"log/slog"
	"runtime"
	"time"

	mcp_golang "github.com/metoro-io/mcp-golang"
	"github.com/rmrfslashbin/mcp/hugo-reader/internal/tools"
)

// ToolOption is a function that configures a Tool.
type ToolOption func(*Tool) error

// Tool provides version and build information about the Hugo Reader MCP server.
type Tool struct {
	log        *slog.Logger
	name       string
	description string
	gitCommit  string
	buildTime  string
	version    string
}

// InfoRequest represents the request parameters for the info tool.
type InfoRequest struct {
	IncludeRuntime bool `json:"include_runtime,omitempty" jsonschema:"title=Include Runtime Info"`
	IncludeTools   bool `json:"include_tools,omitempty" jsonschema:"title=Include Tools List"`
}

// New creates a new Tool.
func New(gitCommit string, opts ...ToolOption) (*Tool, error) {
	tool := &Tool{
		name:        "hugo_reader_info",
		description: "Get version, build, and runtime information about the Hugo Reader MCP server. Useful for debugging and version verification.",
		gitCommit:   gitCommit,
		buildTime:   time.Now().Format(time.RFC3339), // Will be set at build time
		version:     "1.0.0",
	}
	for _, opt := range opts {
		if err := opt(tool); err != nil {
			return nil, err
		}
	}

	return tool, nil
}

// WithLogger sets the logger for the Tool.
func WithLogger(logger *slog.Logger) ToolOption {
	return func(t *Tool) error {
		t.log = logger.With("tool", t.name)
		return nil
	}
}

// WithBuildTime sets the build time for the Tool.
func WithBuildTime(buildTime string) ToolOption {
	return func(t *Tool) error {
		t.buildTime = buildTime
		return nil
	}
}

// WithVersion sets the version for the Tool.
func WithVersion(version string) ToolOption {
	return func(t *Tool) error {
		t.version = version
		return nil
	}
}

// Validate implements tools.Request
func (r *InfoRequest) Validate() error {
	// No validation needed for info request
	return nil
}

// Execute returns version and build information.
func (t *Tool) Execute(req tools.Request) (*mcp_golang.ToolResponse, error) {
	// Check if logger is initialized
	if t.log == nil {
		t.log = slog.Default().With("tool", t.name)
	}

	infoRequest, ok := req.(*InfoRequest)
	if !ok {
		return nil, fmt.Errorf("invalid request type: %T", req)
	}

	if err := infoRequest.Validate(); err != nil {
		return nil, err
	}

	// Build basic info
	info := map[string]interface{}{
		"name":        "Hugo Reader MCP Server",
		"version":     t.version,
		"git_commit":  t.gitCommit,
		"build_time":  t.buildTime,
		"description": "Model Control Protocol server for Hugo static sites",
		"repository":  "https://github.com/rmrfslashbin/mcp/hugo-reader",
	}

	// Add runtime info if requested
	if infoRequest.IncludeRuntime {
		info["runtime"] = map[string]interface{}{
			"go_version":    runtime.Version(),
			"go_os":         runtime.GOOS,
			"go_arch":       runtime.GOARCH,
			"num_cpu":       runtime.NumCPU(),
			"num_goroutine": runtime.NumGoroutine(),
		}
	}

	// Add tools list if requested
	if infoRequest.IncludeTools {
		tools := []map[string]interface{}{
			{
				"name":        "hugo_reader_get_taxonomies",
				"description": "Get all taxonomies defined in a Hugo site",
				"purpose":     "Site structure analysis",
			},
			{
				"name":        "hugo_reader_get_taxonomy_terms",
				"description": "Get all terms for a specific taxonomy",
				"purpose":     "Content organization exploration",
			},
			{
				"name":        "hugo_reader_get_content",
				"description": "Get content from Hugo sites by path",
				"purpose":     "Content retrieval",
			},
			{
				"name":        "hugo_reader_search",
				"description": "Search content across Hugo sites",
				"purpose":     "Content discovery",
			},
			{
				"name":        "hugo_reader_discover_site",
				"description": "Discover available content and structure",
				"purpose":     "Site exploration",
			},
			{
				"name":        "hugo_reader_cache_manager",
				"description": "Manage cache for performance",
				"purpose":     "Performance optimization",
			},
			{
				"name":        "hugo_reader_info",
				"description": "Get version and build information",
				"purpose":     "Version management",
			},
		}
		info["tools"] = tools
	}

	// Add MCP protocol info
	info["mcp"] = map[string]interface{}{
		"protocol_version": "1.0",
		"transport":        "stdio",
		"capabilities": []string{
			"tool_execution",
			"caching",
			"error_handling",
			"request_validation",
		},
	}

	// Format response
	responseData := fmt.Sprintf(`{
  "success": true,
  "info": %s,
  "timestamp": "%s",
  "errors": []
}`, formatInfoSimple(info), time.Now().Format(time.RFC3339))

	t.log.Info("Info request completed", "include_runtime", infoRequest.IncludeRuntime, "include_tools", infoRequest.IncludeTools)
	return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(responseData)), nil
}

// formatInfoSimple formats the info map as JSON
func formatInfoSimple(info map[string]interface{}) string {
	result := "{\n"
	
	// Basic info
	result += fmt.Sprintf(`    "name": "%s",`, info["name"])
	result += fmt.Sprintf(`\n    "version": "%s",`, info["version"])
	result += fmt.Sprintf(`\n    "git_commit": "%s",`, info["git_commit"])
	result += fmt.Sprintf(`\n    "build_time": "%s",`, info["build_time"])
	result += fmt.Sprintf(`\n    "description": "%s",`, info["description"])
	result += fmt.Sprintf(`\n    "repository": "%s"`, info["repository"])
	
	// Runtime info if present
	if runtime, exists := info["runtime"]; exists {
		if runtimeMap, ok := runtime.(map[string]interface{}); ok {
			result += `,\n    "runtime": {`
			result += fmt.Sprintf(`\n      "go_version": "%s",`, runtimeMap["go_version"])
			result += fmt.Sprintf(`\n      "go_os": "%s",`, runtimeMap["go_os"])
			result += fmt.Sprintf(`\n      "go_arch": "%s",`, runtimeMap["go_arch"])
			result += fmt.Sprintf(`\n      "num_cpu": %d,`, runtimeMap["num_cpu"])
			result += fmt.Sprintf(`\n      "num_goroutine": %d`, runtimeMap["num_goroutine"])
			result += `\n    }`
		}
	}
	
	// Tools list if present
	if tools, exists := info["tools"]; exists {
		if toolsList, ok := tools.([]map[string]interface{}); ok {
			result += `,\n    "tools": [`
			for i, tool := range toolsList {
				if i > 0 {
					result += `,`
				}
				result += fmt.Sprintf(`\n      {
        "name": "%s",
        "description": "%s",
        "purpose": "%s"
      }`, tool["name"], tool["description"], tool["purpose"])
			}
			result += `\n    ]`
		}
	}
	
	// MCP info
	if mcp, exists := info["mcp"]; exists {
		if mcpMap, ok := mcp.(map[string]interface{}); ok {
			result += `,\n    "mcp": {`
			result += fmt.Sprintf(`\n      "protocol_version": "%s",`, mcpMap["protocol_version"])
			result += fmt.Sprintf(`\n      "transport": "%s",`, mcpMap["transport"])
			result += `\n      "capabilities": ["tool_execution", "caching", "error_handling", "request_validation"]`
			result += `\n    }`
		}
	}
	
	result += `\n  }`
	return result
}

// Name returns the name of the tool.
func (t *Tool) Name() string {
	return t.name
}

// Description returns the description of the tool.
func (t *Tool) Description() string {
	return t.description
}

// SetLogger sets the logger for the Tool.
func (t *Tool) SetLogger(logger *slog.Logger) {
	if logger == nil {
		t.log = slog.Default().With("tool", t.name)
		t.log.Warn("nil logger provided, using default")
		return
	}
	t.log = logger.With("tool", t.name)
}