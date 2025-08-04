package content

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	mcp_golang "github.com/metoro-io/mcp-golang"
	"github.com/rmrfslashbin/mcp/hugo-reader/internal/cache"
	"github.com/rmrfslashbin/mcp/hugo-reader/internal/tools"
	"github.com/tidwall/gjson"
)

// ToolOption is a function that configures a Tool.
type ToolOption func(*Tool) error

// Tool retrieves content from Hugo sites with bulk support.
type Tool struct {
	log        *slog.Logger
	name       string
	description string
	httpClient *http.Client
	cache      *cache.Cache
}

// ContentRequest represents the request parameters for the content tool.
type ContentRequest struct {
	HugoSitePath string   `json:"hugo_site_path" jsonschema:"title=Hugo Site Path"`
	Paths        []string `json:"paths" jsonschema:"title=Content Paths,minItems=1"`
	Include      []string `json:"include" jsonschema:"title=Include Fields,enum=metadata,enum=body,enum=both"`
	Limit        int      `json:"limit,omitempty" jsonschema:"title=Limit,minimum=1,maximum=100"`
}

// EndpointConfig represents an endpoint with its validation function
type EndpointConfig struct {
	path      string
	validator func([]byte) bool
}

// New creates a new Tool.
func New(opts ...ToolOption) (*Tool, error) {
	tool := &Tool{
		name:        "hugo_reader_get_content",
		description: "Get content from Hugo sites by path. Supports bulk retrieval and flexible response options (metadata, body, or both). Tries multiple endpoint patterns automatically. Example paths: '/posts/my-post/', '/recipes/cookies/', '/about/'. Use with or without trailing slashes.",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		cache: cache.New(cache.WithTTL(5 * time.Minute)),
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
		if t.cache != nil {
			t.cache = cache.New(cache.WithLogger(logger), cache.WithTTL(5*time.Minute))
		}
		return nil
	}
}

// WithCache sets the cache for the Tool.
func WithCache(c *cache.Cache) ToolOption {
	return func(t *Tool) error {
		t.cache = c
		return nil
	}
}

// Validate implements tools.Request
func (r *ContentRequest) Validate() error {
	if r.HugoSitePath == "" {
		return fmt.Errorf("hugo_site_path is required")
	}
	if len(r.Paths) == 0 {
		return fmt.Errorf("at least one path is required")
	}
	
	// Set default include if not specified
	if len(r.Include) == 0 {
		r.Include = []string{"both"}
	}
	
	// Validate include values
	validIncludes := map[string]bool{"metadata": true, "body": true, "both": true}
	for _, include := range r.Include {
		if !validIncludes[include] {
			return fmt.Errorf("invalid include value: %s (must be: metadata, body, or both)", include)
		}
	}
	
	// Set default limit if not specified or validate
	if r.Limit == 0 {
		r.Limit = 50 // Default limit
	} else if r.Limit < 1 || r.Limit > 100 {
		return fmt.Errorf("limit must be between 1 and 100")
	}
	
	return nil
}

// Execute retrieves content from a Hugo site.
func (t *Tool) Execute(req tools.Request) (*mcp_golang.ToolResponse, error) {
	// Check if logger is initialized
	if t.log == nil {
		t.log = slog.Default().With("tool", t.name)
	}

	contentRequest, ok := req.(*ContentRequest)
	if !ok {
		return nil, fmt.Errorf("invalid request type: %T", req)
	}

	if err := contentRequest.Validate(); err != nil {
		return nil, err
	}

	// Parse and validate the Hugo site URL
	siteURL, err := url.Parse(contentRequest.HugoSitePath)
	if err != nil {
		t.log.Error("Invalid Hugo site URL", "url", contentRequest.HugoSitePath, "error", err)
		return nil, fmt.Errorf("invalid Hugo site URL: %w", err)
	}

	// Ensure URL has scheme
	if siteURL.Scheme == "" {
		siteURL.Scheme = "https"
	}

	var allContent []map[string]interface{}
	var errors []string
	processedCount := 0

	for _, path := range contentRequest.Paths {
		if processedCount >= contentRequest.Limit {
			break
		}

		content, err := t.getContentForPath(siteURL, path, contentRequest.Include)
		if err != nil {
			t.log.Warn("Failed to retrieve content for path", "path", path, "error", err)
			errors = append(errors, fmt.Sprintf("Path '%s': %s", path, err.Error()))
			continue
		}

		if content != nil {
			allContent = append(allContent, content)
			processedCount++
		}
	}

	// Format response with comprehensive metadata
	responseData := fmt.Sprintf(`{
  "success": true,
  "content": %s,
  "metadata": {
    "requested_paths": %d,
    "retrieved_count": %d,
    "error_count": %d,
    "limit_applied": %d,
    "include_fields": %s
  },
  "errors": %s
}`, formatContent(allContent), len(contentRequest.Paths), len(allContent), len(errors), contentRequest.Limit, formatStringArray(contentRequest.Include), formatErrors(errors))

	t.log.Info("Successfully retrieved content", "requested", len(contentRequest.Paths), "retrieved", len(allContent), "errors", len(errors), "site", contentRequest.HugoSitePath)
	return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(responseData)), nil
}

// getContentForPath retrieves content for a single path
func (t *Tool) getContentForPath(siteURL *url.URL, path string, include []string) (map[string]interface{}, error) {
	// Clean and normalize the path
	cleanPath := strings.TrimPrefix(path, "/")
	cleanPath = strings.TrimSuffix(cleanPath, "/")
	if cleanPath == "" {
		cleanPath = "index"
	}

	// Try common Hugo content endpoints with better path handling
	contentEndpoints := []EndpointConfig{
		{path: fmt.Sprintf("/%s.json", cleanPath), validator: validateContentStructure},
		{path: fmt.Sprintf("/%s/index.json", cleanPath), validator: validateContentStructure},
		{path: fmt.Sprintf("/content/%s.json", cleanPath), validator: validateContentStructure},
		{path: fmt.Sprintf("/content/%s/index.json", cleanPath), validator: validateContentStructure},
		{path: "/index.json", validator: validateHugoIndexForContent},
	}

	var contentData []byte
	var found bool
	var usedEndpoint string

	for _, endpointConfig := range contentEndpoints {
		contentURL := siteURL.ResolveReference(&url.URL{Path: endpointConfig.path})
		cacheKey := t.cache.BuildKey(siteURL.String(), endpointConfig.path, map[string]string{"path": path, "include": strings.Join(include, ",")})
		
		t.log.Debug("Trying content endpoint", "url", contentURL.String(), "cache_key", cacheKey)

		// Check cache first
		if cachedData, hit := t.cache.Get(cacheKey); hit {
			t.log.Debug("Cache hit for content endpoint", "url", contentURL.String())
			if endpointConfig.validator(cachedData) {
				contentData = cachedData
				found = true
				usedEndpoint = contentURL.String()
				break
			} else {
				t.log.Debug("Cached content data failed validation, invalidating", "url", contentURL.String())
				t.cache.Delete(cacheKey)
			}
		}

		// Fetch from network
		resp, err := t.httpClient.Get(contentURL.String())
		if err != nil {
			t.log.Debug("Failed to fetch content endpoint", "url", contentURL.String(), "error", err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.log.Debug("Failed to read content response body", "url", contentURL.String(), "error", err)
				continue
			}

			// Validate response contains content data
			if endpointConfig.validator(body) {
				// Cache the validated response
				etag := resp.Header.Get("ETag")
				lastModified := resp.Header.Get("Last-Modified")
				t.cache.Set(cacheKey, body, etag, lastModified)
				
				contentData = body
				found = true
				usedEndpoint = contentURL.String()
				t.log.Debug("Found and cached content", "url", contentURL.String(), "path", path)
				break
			} else {
				t.log.Debug("Response failed content validation", "url", contentURL.String(), "path", path)
			}
		} else {
			t.log.Debug("HTTP error from content endpoint", "url", contentURL.String(), "status", resp.StatusCode)
		}
	}

	if !found {
		return nil, fmt.Errorf("content not found")
	}

	// Extract content from validated JSON
	content := extractContent(contentData, path, include, usedEndpoint)
	return content, nil
}

// validateContentStructure checks if the JSON contains valid content data
func validateContentStructure(data []byte) bool {
	if !gjson.ValidBytes(data) {
		return false
	}

	parsed := gjson.ParseBytes(data)
	
	// Check for content fields that Hugo typically provides
	contentFields := []string{"title", "content", "body", "summary", "date", "slug", "url"}
	foundFields := 0
	
	for _, field := range contentFields {
		if parsed.Get(field).Exists() {
			foundFields++
		}
	}
	
	// Consider it valid content if we found at least 2 content-related fields
	return foundFields >= 2
}

// validateHugoIndexForContent checks if the Hugo index can provide content
func validateHugoIndexForContent(data []byte) bool {
	if !gjson.ValidBytes(data) {
		return false
	}

	parsed := gjson.ParseBytes(data)
	
	// Look for pages with content
	if pages := parsed.Get("pages"); pages.Exists() && pages.IsArray() {
		hasContentData := false
		pages.ForEach(func(key, page gjson.Result) bool {
			if page.Get("content").Exists() || page.Get("body").Exists() || page.Get("summary").Exists() {
				hasContentData = true
				return false // Stop iteration
			}
			return true
		})
		return hasContentData
	}
	
	return validateContentStructure(data)
}

// extractContent parses content from validated JSON data
func extractContent(data []byte, requestedPath string, include []string, sourceEndpoint string) map[string]interface{} {
	parsed := gjson.ParseBytes(data)
	content := make(map[string]interface{})
	
	includeMetadata := contains(include, "metadata") || contains(include, "both")
	includeBody := contains(include, "body") || contains(include, "both")
	
	// Set the path
	content["path"] = requestedPath
	content["source_endpoint"] = sourceEndpoint
	
	// If this is a pages array, find the matching page
	if pages := parsed.Get("pages"); pages.Exists() && pages.IsArray() {
		var matchedPage gjson.Result
		pages.ForEach(func(key, page gjson.Result) bool {
			if pageURL := page.Get("url"); pageURL.Exists() {
				if strings.Contains(pageURL.String(), requestedPath) || strings.Contains(requestedPath, pageURL.String()) {
					matchedPage = page
					return false // Stop iteration
				}
			}
			if pageSlug := page.Get("slug"); pageSlug.Exists() {
				if strings.Contains(pageSlug.String(), requestedPath) || strings.Contains(requestedPath, pageSlug.String()) {
					matchedPage = page
					return false // Stop iteration
				}
			}
			return true
		})
		
		if matchedPage.Exists() {
			parsed = matchedPage
		}
	}
	
	// Extract metadata if requested
	if includeMetadata {
		metadata := make(map[string]interface{})
		
		metadataFields := []string{"title", "date", "slug", "url", "summary", "tags", "categories", "author", "description", "draft", "publishDate"}
		for _, field := range metadataFields {
			if value := parsed.Get(field); value.Exists() {
				metadata[field] = value.Value()
			}
		}
		
		// Add any custom front matter fields
		parsed.ForEach(func(key, value gjson.Result) bool {
			keyStr := key.String()
			if keyStr != "content" && keyStr != "body" && keyStr != "html" {
				if _, exists := metadata[keyStr]; !exists {
					metadata[keyStr] = value.Value()
				}
			}
			return true
		})
		
		content["metadata"] = metadata
	}
	
	// Extract body if requested
	if includeBody {
		body := make(map[string]interface{})
		
		if contentField := parsed.Get("content"); contentField.Exists() {
			body["content"] = contentField.String()
		}
		if bodyField := parsed.Get("body"); bodyField.Exists() {
			body["body"] = bodyField.String()
		}
		if htmlField := parsed.Get("html"); htmlField.Exists() {
			body["html"] = htmlField.String()
		}
		if summaryField := parsed.Get("summary"); summaryField.Exists() {
			body["summary"] = summaryField.String()
		}
		
		content["body"] = body
	}
	
	return content
}

// Helper functions
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func formatContent(content []map[string]interface{}) string {
	if len(content) == 0 {
		return "[]"
	}
	
	var parts []string
	for _, item := range content {
		// Simple JSON serialization for the content items
		parts = append(parts, fmt.Sprintf("    %s", formatContentItem(item)))
	}
	
	return "[\n" + strings.Join(parts, ",\n") + "\n  ]"
}

func formatContentItem(item map[string]interface{}) string {
	var parts []string
	
	for key, value := range item {
		switch v := value.(type) {
		case string:
			parts = append(parts, fmt.Sprintf(`"%s": "%s"`, key, strings.ReplaceAll(v, `"`, `\"`)))
		case map[string]interface{}:
			parts = append(parts, fmt.Sprintf(`"%s": %s`, key, formatContentItem(v)))
		default:
			parts = append(parts, fmt.Sprintf(`"%s": %v`, key, v))
		}
	}
	
	return "{" + strings.Join(parts, ", ") + "}"
}

func formatStringArray(arr []string) string {
	if len(arr) == 0 {
		return "[]"
	}
	
	var quoted []string
	for _, s := range arr {
		quoted = append(quoted, fmt.Sprintf(`"%s"`, s))
	}
	
	return "[" + strings.Join(quoted, ", ") + "]"
}

func formatErrors(errors []string) string {
	if len(errors) == 0 {
		return "[]"
	}
	
	var quoted []string
	for _, err := range errors {
		quoted = append(quoted, fmt.Sprintf(`"%s"`, strings.ReplaceAll(err, `"`, `\"`)))
	}
	
	return "[\n    " + strings.Join(quoted, ",\n    ") + "\n  ]"
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