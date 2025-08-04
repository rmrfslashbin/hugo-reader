package search

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	mcp_golang "github.com/metoro-io/mcp-golang"
	"github.com/rmrfslashbin/mcp/hugo-reader/internal/cache"
	"github.com/rmrfslashbin/mcp/hugo-reader/internal/tools"
	"github.com/tidwall/gjson"
)

// ToolOption is a function that configures a Tool.
type ToolOption func(*Tool) error

// Tool performs search across Hugo site content with Hugo-specific optimizations.
type Tool struct {
	log        *slog.Logger
	name       string
	description string
	httpClient *http.Client
	cache      *cache.Cache
}

// SearchRequest represents the request parameters for the search tool.
type SearchRequest struct {
	HugoSitePath string `json:"hugo_site_path" jsonschema:"title=Hugo Site Path"`
	Query        string `json:"query" jsonschema:"title=Search Query"`
	ContentType  string `json:"content_type,omitempty" jsonschema:"title=Content Type Filter"`
	Taxonomy     string `json:"taxonomy,omitempty" jsonschema:"title=Taxonomy Filter"`
	Term         string `json:"term,omitempty" jsonschema:"title=Taxonomy Term Filter"`
	Limit        int    `json:"limit,omitempty" jsonschema:"title=Result Limit,minimum=1,maximum=100"`
}

// EndpointConfig represents an endpoint with its validation function
type EndpointConfig struct {
	path      string
	params    map[string]string
	validator func([]byte) bool
}

// New creates a new Tool.
func New(opts ...ToolOption) (*Tool, error) {
	tool := &Tool{
		name:        "hugo_reader_search",
		description: "Search content across Hugo sites by keywords. Tries Hugo-native search endpoints first, then falls back to content scanning. Supports filters by content_type, taxonomy, and term. Use for finding content when you don't know exact paths.",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		cache: cache.New(cache.WithTTL(2 * time.Minute)), // Shorter TTL for search results
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
			t.cache = cache.New(cache.WithLogger(logger), cache.WithTTL(2*time.Minute))
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
func (r *SearchRequest) Validate() error {
	if r.HugoSitePath == "" {
		return fmt.Errorf("hugo_site_path is required")
	}
	if r.Query == "" {
		return fmt.Errorf("query is required")
	}
	
	// Set default limit if not specified or validate
	if r.Limit == 0 {
		r.Limit = 20 // Default limit for search results
	} else if r.Limit < 1 || r.Limit > 100 {
		return fmt.Errorf("limit must be between 1 and 100")
	}
	
	return nil
}

// Execute performs search across Hugo site content.
func (t *Tool) Execute(req tools.Request) (*mcp_golang.ToolResponse, error) {
	// Check if logger is initialized
	if t.log == nil {
		t.log = slog.Default().With("tool", t.name)
	}

	searchRequest, ok := req.(*SearchRequest)
	if !ok {
		return nil, fmt.Errorf("invalid request type: %T", req)
	}

	if err := searchRequest.Validate(); err != nil {
		return nil, err
	}

	// Parse and validate the Hugo site URL
	siteURL, err := url.Parse(searchRequest.HugoSitePath)
	if err != nil {
		t.log.Error("Invalid Hugo site URL", "url", searchRequest.HugoSitePath, "error", err)
		return nil, fmt.Errorf("invalid Hugo site URL: %w", err)
	}

	// Ensure URL has scheme
	if siteURL.Scheme == "" {
		siteURL.Scheme = "https"
	}

	// Try Hugo-specific search endpoints first, then fallback to content scanning
	searchResults, searchMetadata, err := t.performHugoSearch(siteURL, searchRequest)
	if err != nil {
		t.log.Debug("Hugo-specific search failed, falling back to content scanning", "error", err)
		searchResults, searchMetadata, err = t.performContentScanSearch(siteURL, searchRequest)
		if err != nil {
			t.log.Error("All search methods failed", "error", err)
			return nil, fmt.Errorf("search failed: %w", err)
		}
		searchMetadata["fallback_used"] = true
	} else {
		searchMetadata["fallback_used"] = false
	}

	// Apply limit
	if len(searchResults) > searchRequest.Limit {
		searchResults = searchResults[:searchRequest.Limit]
		searchMetadata["limited"] = true
	} else {
		searchMetadata["limited"] = false
	}

	// Format response
	responseData := fmt.Sprintf(`{
  "success": true,
  "query": "%s",
  "results": %s,
  "metadata": %s,
  "errors": []
}`, searchRequest.Query, formatSearchResults(searchResults), formatMetadata(searchMetadata))

	t.log.Info("Search completed", "query", searchRequest.Query, "results", len(searchResults), "site", searchRequest.HugoSitePath, "fallback", searchMetadata["fallback_used"])
	return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(responseData)), nil
}

// performHugoSearch attempts to use Hugo's built-in search indices
func (t *Tool) performHugoSearch(siteURL *url.URL, req *SearchRequest) ([]map[string]interface{}, map[string]interface{}, error) {
	// Try common Hugo search endpoint patterns
	searchEndpoints := []EndpointConfig{
		{path: "/search.json", params: map[string]string{"q": req.Query}, validator: validateSearchResults},
		{path: "/api/search.json", params: map[string]string{"query": req.Query}, validator: validateSearchResults},
		{path: "/search/index.json", params: map[string]string{"q": req.Query}, validator: validateSearchResults},
		{path: "/index.json", params: map[string]string{"search": req.Query}, validator: validateHugoIndexForSearch},
	}

	for _, endpoint := range searchEndpoints {
		searchURL := siteURL.ResolveReference(&url.URL{Path: endpoint.path})
		
		// Add query parameters
		params := url.Values{}
		for key, value := range endpoint.params {
			params.Add(key, value)
		}
		
		// Add filter parameters if specified
		if req.ContentType != "" {
			params.Add("type", req.ContentType)
		}
		if req.Taxonomy != "" && req.Term != "" {
			params.Add(req.Taxonomy, req.Term)
		}
		if req.Limit > 0 {
			params.Add("limit", strconv.Itoa(req.Limit))
		}
		
		searchURL.RawQuery = params.Encode()
		
		// Build cache key
		cacheParams := make(map[string]string)
		for key, values := range params {
			if len(values) > 0 {
				cacheParams[key] = values[0]
			}
		}
		cacheKey := t.cache.BuildKey(siteURL.String(), endpoint.path, cacheParams)
		
		t.log.Debug("Trying Hugo search endpoint", "url", searchURL.String(), "cache_key", cacheKey)

		// Check cache first
		if cachedData, hit := t.cache.Get(cacheKey); hit {
			t.log.Debug("Cache hit for search endpoint", "url", searchURL.String())
			if endpoint.validator(cachedData) {
				results := extractSearchResults(cachedData, req)
				metadata := map[string]interface{}{
					"search_method":    "hugo_native",
					"source_endpoint":  searchURL.String(),
					"result_count":     len(results),
					"cached":          true,
				}
				return results, metadata, nil
			} else {
				t.log.Debug("Cached search data failed validation, invalidating", "url", searchURL.String())
				t.cache.Delete(cacheKey)
			}
		}

		// Fetch from network
		resp, err := t.httpClient.Get(searchURL.String())
		if err != nil {
			t.log.Debug("Failed to fetch search endpoint", "url", searchURL.String(), "error", err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.log.Debug("Failed to read search response body", "url", searchURL.String(), "error", err)
				continue
			}

			// Validate response contains search results
			if endpoint.validator(body) {
				// Cache the validated response
				etag := resp.Header.Get("ETag")
				lastModified := resp.Header.Get("Last-Modified")
				t.cache.Set(cacheKey, body, etag, lastModified)
				
				results := extractSearchResults(body, req)
				metadata := map[string]interface{}{
					"search_method":    "hugo_native",
					"source_endpoint":  searchURL.String(),
					"result_count":     len(results),
					"cached":          false,
				}
				
				t.log.Info("Hugo search successful", "url", searchURL.String(), "results", len(results))
				return results, metadata, nil
			} else {
				t.log.Debug("Response failed search validation", "url", searchURL.String())
			}
		} else {
			t.log.Debug("HTTP error from search endpoint", "url", searchURL.String(), "status", resp.StatusCode)
		}
	}

	return nil, nil, fmt.Errorf("no Hugo search endpoints available")
}

// performContentScanSearch falls back to scanning available content
func (t *Tool) performContentScanSearch(siteURL *url.URL, req *SearchRequest) ([]map[string]interface{}, map[string]interface{}, error) {
	// Try to get all content and search through it
	contentEndpoints := []EndpointConfig{
		{path: "/index.json", validator: validateHugoIndexForSearch},
		{path: "/content/index.json", validator: validateSearchResults},
		{path: "/posts/index.json", validator: validateSearchResults},
		{path: "/api/content.json", validator: validateSearchResults},
		{path: "/all.json", validator: validateSearchResults},
		{path: "/site.json", validator: validateSearchResults},
	}

	for _, endpoint := range contentEndpoints {
		contentURL := siteURL.ResolveReference(&url.URL{Path: endpoint.path})
		cacheKey := t.cache.BuildKey(siteURL.String(), endpoint.path, nil)
		
		t.log.Debug("Trying content scan endpoint", "url", contentURL.String())

		var contentData []byte
		
		// Check cache first
		if cachedData, hit := t.cache.Get(cacheKey); hit {
			t.log.Debug("Cache hit for content scan", "url", contentURL.String())
			if endpoint.validator(cachedData) {
				contentData = cachedData
			} else {
				t.cache.Delete(cacheKey)
				continue
			}
		} else {
			// Fetch from network
			resp, err := t.httpClient.Get(contentURL.String())
			if err != nil {
				t.log.Debug("Failed to fetch content endpoint", "url", contentURL.String(), "error", err)
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.log.Debug("HTTP error from content endpoint", "url", contentURL.String(), "status", resp.StatusCode)
				continue
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.log.Debug("Failed to read content response body", "url", contentURL.String(), "error", err)
				continue
			}

			if !endpoint.validator(body) {
				t.log.Debug("Content data failed validation", "url", contentURL.String())
				continue
			}

			// Cache the validated response
			etag := resp.Header.Get("ETag")
			lastModified := resp.Header.Get("Last-Modified")
			t.cache.Set(cacheKey, body, etag, lastModified)
			contentData = body
		}

		// Perform client-side search
		results := performClientSideSearch(contentData, req)
		metadata := map[string]interface{}{
			"search_method":    "content_scan",
			"source_endpoint":  contentURL.String(),
			"result_count":     len(results),
			"cached":          contentData != nil,
		}
		
		t.log.Info("Content scan search completed", "url", contentURL.String(), "results", len(results))
		return results, metadata, nil
	}

	return nil, nil, fmt.Errorf("no content available for scanning")
}

// Validation functions
func validateSearchResults(data []byte) bool {
	if !gjson.ValidBytes(data) {
		return false
	}

	parsed := gjson.ParseBytes(data)
	
	// Check for search result structures
	if results := parsed.Get("results"); results.Exists() && results.IsArray() {
		return true
	}
	if results := parsed.Get("hits"); results.Exists() && results.IsArray() {
		return true
	}
	if parsed.IsArray() {
		// Direct array of results
		return parsed.Array() != nil
	}
	
	return false
}

func validateHugoIndexForSearch(data []byte) bool {
	if !gjson.ValidBytes(data) {
		return false
	}

	parsed := gjson.ParseBytes(data)
	
	// Check for pages that can be searched
	if pages := parsed.Get("pages"); pages.Exists() && pages.IsArray() {
		return len(pages.Array()) > 0
	}
	
	return parsed.IsArray() && len(parsed.Array()) > 0
}

// Search result extraction
func extractSearchResults(data []byte, req *SearchRequest) []map[string]interface{} {
	var results []map[string]interface{}
	parsed := gjson.ParseBytes(data)
	
	// Handle different search result formats
	var resultsArray gjson.Result
	if resultsField := parsed.Get("results"); resultsField.Exists() && resultsField.IsArray() {
		resultsArray = resultsField
	} else if hits := parsed.Get("hits"); hits.Exists() && hits.IsArray() {
		resultsArray = hits
	} else if parsed.IsArray() {
		resultsArray = parsed
	} else {
		return results
	}
	
	resultsArray.ForEach(func(key, item gjson.Result) bool {
		result := make(map[string]interface{})
		
		// Extract common fields
		if title := item.Get("title"); title.Exists() {
			result["title"] = title.String()
		}
		if url := item.Get("url"); url.Exists() {
			result["url"] = url.String()
		}
		if content := item.Get("content"); content.Exists() {
			result["content"] = content.String()
		}
		if summary := item.Get("summary"); summary.Exists() {
			result["summary"] = summary.String()
		}
		if date := item.Get("date"); date.Exists() {
			result["date"] = date.String()
		}
		
		// Extract taxonomies if they exist
		if categories := item.Get("categories"); categories.Exists() {
			result["categories"] = categories.Value()
		}
		if tags := item.Get("tags"); tags.Exists() {
			result["tags"] = tags.Value()
		}
		
		// Add relevance score if available
		if score := item.Get("score"); score.Exists() {
			result["score"] = score.Float()
		}
		
		results = append(results, result)
		return true
	})
	
	return results
}

// Client-side search implementation
func performClientSideSearch(data []byte, req *SearchRequest) []map[string]interface{} {
	var results []map[string]interface{}
	parsed := gjson.ParseBytes(data)
	
	query := strings.ToLower(req.Query)
	
	// Handle pages array
	var itemsToSearch gjson.Result
	if pages := parsed.Get("pages"); pages.Exists() && pages.IsArray() {
		itemsToSearch = pages
	} else if parsed.IsArray() {
		itemsToSearch = parsed
	} else {
		return results
	}
	
	itemsToSearch.ForEach(func(key, item gjson.Result) bool {
		// Check if item matches query
		matched := false
		relevanceScore := 0.0
		
		// Search in title (higher relevance)
		if title := item.Get("title"); title.Exists() {
			titleStr := strings.ToLower(title.String())
			if strings.Contains(titleStr, query) {
				matched = true
				relevanceScore += 10.0
				if titleStr == query {
					relevanceScore += 20.0 // Exact match bonus
				}
			}
		}
		
		// Search in content/body
		contentFields := []string{"content", "body", "summary"}
		for _, field := range contentFields {
			if content := item.Get(field); content.Exists() {
				contentStr := strings.ToLower(content.String())
				if strings.Contains(contentStr, query) {
					matched = true
					relevanceScore += 1.0
					// Count number of matches for better scoring
					relevanceScore += float64(strings.Count(contentStr, query))
				}
			}
		}
		
		// Apply filters
		if matched {
			// Content type filter
			if req.ContentType != "" {
				if contentType := item.Get("type"); contentType.Exists() {
					if !strings.EqualFold(contentType.String(), req.ContentType) {
						matched = false
					}
				}
			}
			
			// Taxonomy filter
			if req.Taxonomy != "" && req.Term != "" {
				if taxonomy := item.Get(req.Taxonomy); taxonomy.Exists() {
					found := false
					if taxonomy.IsArray() {
						taxonomy.ForEach(func(k, v gjson.Result) bool {
							if strings.EqualFold(v.String(), req.Term) {
								found = true
								return false
							}
							return true
						})
					} else if strings.EqualFold(taxonomy.String(), req.Term) {
						found = true
					}
					if !found {
						matched = false
					}
				} else {
					matched = false
				}
			}
		}
		
		if matched {
			result := make(map[string]interface{})
			
			// Extract fields
			if title := item.Get("title"); title.Exists() {
				result["title"] = title.String()
			}
			if url := item.Get("url"); url.Exists() {
				result["url"] = url.String()
			}
			if content := item.Get("content"); content.Exists() {
				// Truncate content for search results
				contentStr := content.String()
				if len(contentStr) > 200 {
					contentStr = contentStr[:200] + "..."
				}
				result["content"] = contentStr
			}
			if summary := item.Get("summary"); summary.Exists() {
				result["summary"] = summary.String()
			}
			if date := item.Get("date"); date.Exists() {
				result["date"] = date.String()
			}
			
			// Add taxonomies
			if categories := item.Get("categories"); categories.Exists() {
				result["categories"] = categories.Value()
			}
			if tags := item.Get("tags"); tags.Exists() {
				result["tags"] = tags.Value()
			}
			
			result["score"] = relevanceScore
			results = append(results, result)
		}
		
		return true
	})
	
	return results
}

// Formatting functions
func formatSearchResults(results []map[string]interface{}) string {
	if len(results) == 0 {
		return "[]"
	}
	
	var parts []string
	for _, result := range results {
		parts = append(parts, fmt.Sprintf("    %s", formatSearchResult(result)))
	}
	
	return "[\n" + strings.Join(parts, ",\n") + "\n  ]"
}

func formatSearchResult(result map[string]interface{}) string {
	var parts []string
	
	for key, value := range result {
		switch v := value.(type) {
		case string:
			parts = append(parts, fmt.Sprintf(`"%s": "%s"`, key, strings.ReplaceAll(v, `"`, `\"`)))
		case float64:
			parts = append(parts, fmt.Sprintf(`"%s": %.2f`, key, v))
		case []interface{}:
			var items []string
			for _, item := range v {
				items = append(items, fmt.Sprintf(`"%v"`, item))
			}
			parts = append(parts, fmt.Sprintf(`"%s": [%s]`, key, strings.Join(items, ", ")))
		default:
			parts = append(parts, fmt.Sprintf(`"%s": %v`, key, v))
		}
	}
	
	return "{" + strings.Join(parts, ", ") + "}"
}

func formatMetadata(metadata map[string]interface{}) string {
	var parts []string
	
	for key, value := range metadata {
		switch v := value.(type) {
		case string:
			parts = append(parts, fmt.Sprintf(`"%s": "%s"`, key, v))
		case bool:
			parts = append(parts, fmt.Sprintf(`"%s": %t`, key, v))
		case int:
			parts = append(parts, fmt.Sprintf(`"%s": %d`, key, v))
		default:
			parts = append(parts, fmt.Sprintf(`"%s": %v`, key, v))
		}
	}
	
	return "{\n    " + strings.Join(parts, ",\n    ") + "\n  }"
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