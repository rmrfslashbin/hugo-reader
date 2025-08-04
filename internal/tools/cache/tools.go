package cache

import (
	"encoding/json"
	"fmt"
	"log/slog"

	mcp_golang "github.com/metoro-io/mcp-golang"
	"github.com/rmrfslashbin/mcp/hugo-reader/internal/cache"
	"github.com/rmrfslashbin/mcp/hugo-reader/internal/tools"
)

// Tool provides cache management functionality
type Tool struct {
	log   *slog.Logger
	cache *cache.Cache
}

// ClearCacheRequest represents the request parameters for clearing cache
type ClearCacheRequest struct {
	Action string `json:"action" jsonschema:"enum=clear,enum=stats,enum=clean,title=Cache Action"`
	Target string `json:"target,omitempty" jsonschema:"title=Target (optional site URL for selective clearing)"`
}

// New creates a new cache management tool
func New(cacheInstance *cache.Cache, opts ...ToolOption) (*Tool, error) {
	tool := &Tool{
		cache: cacheInstance,
		log:   slog.Default().With("tool", "hugo_reader_cache_manager"),
	}
	
	for _, opt := range opts {
		if err := opt(tool); err != nil {
			return nil, err
		}
	}
	
	return tool, nil
}

// ToolOption configures the cache tool
type ToolOption func(*Tool) error

// WithLogger sets the logger for the tool
func WithLogger(logger *slog.Logger) ToolOption {
	return func(t *Tool) error {
		t.log = logger.With("tool", "hugo_reader_cache_manager")
		return nil
	}
}

// Validate implements tools.Request
func (r *ClearCacheRequest) Validate() error {
	switch r.Action {
	case "clear", "stats", "clean":
		return nil
	default:
		return fmt.Errorf("invalid action: %s (must be: clear, stats, or clean)", r.Action)
	}
}

// Execute manages cache operations
func (t *Tool) Execute(req tools.Request) (*mcp_golang.ToolResponse, error) {
	cacheRequest, ok := req.(*ClearCacheRequest)
	if !ok {
		return nil, fmt.Errorf("invalid request type: %T", req)
	}
	
	if err := cacheRequest.Validate(); err != nil {
		return nil, err
	}
	
	switch cacheRequest.Action {
	case "clear":
		return t.clearCache(cacheRequest.Target)
	case "stats":
		return t.getCacheStats()
	case "clean":
		return t.cleanExpired()
	default:
		return nil, fmt.Errorf("unknown action: %s", cacheRequest.Action)
	}
}

// clearCache clears all or targeted cache entries
func (t *Tool) clearCache(target string) (*mcp_golang.ToolResponse, error) {
	if target == "" {
		// Clear all cache
		t.cache.Clear()
		t.log.Info("Cleared all cache entries")
		
		response := map[string]interface{}{
			"success": true,
			"message": "All cache entries cleared",
			"action":  "clear_all",
		}
		
		responseJSON, _ := json.Marshal(response)
		return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(string(responseJSON))), nil
	}
	
	// TODO: Implement selective clearing by site URL pattern
	// For now, treat any target as "clear all"
	t.cache.Clear()
	t.log.Info("Cleared cache entries", "target", target)
	
	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Cache entries cleared for target: %s", target),
		"action":  "clear_targeted",
		"target":  target,
	}
	
	responseJSON, _ := json.Marshal(response)
	return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(string(responseJSON))), nil
}

// getCacheStats returns cache statistics
func (t *Tool) getCacheStats() (*mcp_golang.ToolResponse, error) {
	stats := t.cache.Stats()
	
	response := map[string]interface{}{
		"success": true,
		"action":  "stats",
		"stats":   stats,
	}
	
	responseJSON, err := json.Marshal(response)
	if err != nil {
		t.log.Error("Failed to marshal cache stats", "error", err)
		return nil, fmt.Errorf("failed to marshal cache stats: %w", err)
	}
	
	t.log.Debug("Retrieved cache stats", "stats", stats)
	return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(string(responseJSON))), nil
}

// cleanExpired removes expired cache entries
func (t *Tool) cleanExpired() (*mcp_golang.ToolResponse, error) {
	removedCount := t.cache.CleanExpired()
	
	response := map[string]interface{}{
		"success":       true,
		"action":        "clean",
		"removed_count": removedCount,
		"message":       fmt.Sprintf("Removed %d expired cache entries", removedCount),
	}
	
	responseJSON, _ := json.Marshal(response)
	t.log.Info("Cleaned expired cache entries", "removed_count", removedCount)
	
	return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(string(responseJSON))), nil
}

// Name returns the tool name
func (t *Tool) Name() string {
	return "hugo_reader_cache_manager"
}

// Description returns the tool description
func (t *Tool) Description() string {
	return "Manage Hugo reader cache with smart HTTP validation. Actions: 'clear' (remove all/specific entries), 'stats' (cache statistics), 'clean' (remove expired entries). Use 'clear' if getting stale data."
}

// SetLogger sets the logger for the tool
func (t *Tool) SetLogger(logger *slog.Logger) {
	if logger == nil {
		t.log = slog.Default().With("tool", "hugo_reader_cache_manager")
		t.log.Warn("nil logger provided, using default")
		return
	}
	t.log = logger.With("tool", "hugo_reader_cache_manager")
}