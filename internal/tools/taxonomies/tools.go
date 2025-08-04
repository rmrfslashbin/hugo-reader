package taxonomies

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

// Tool is a tool that retrieves taxonomies from Hugo sites.
type Tool struct {
	log         *slog.Logger
	name        string
	description string
	httpClient  *http.Client
	cache       *cache.Cache
}

// TaxonomiesRequest represents the request parameters for the taxonomies tool.
type TaxonomiesRequest struct {
	HugoSitePath string `json:"hugo_site_path" jsonschema:"title=Hugo Site Path"`
}

// New creates a new Tool.
func New(opts ...ToolOption) (*Tool, error) {
	tool := &Tool{
		name:        "hugo_reader_get_taxonomies",
		description: "Get all taxonomies defined in a Hugo site (e.g., categories, tags, authors). Returns the taxonomy names and their configuration. Use this first to understand the site's content organization.",
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
func (r *TaxonomiesRequest) Validate() error {
	if r.HugoSitePath == "" {
		return &ErrHugoSitePathRequired{}
	}
	return nil
}

// Execute retrieves taxonomies from a Hugo site.
func (t *Tool) Execute(req tools.Request) (*mcp_golang.ToolResponse, error) {
	// Check if logger is initialized
	if t.log == nil {
		// Default to standard logger if not set
		t.log = slog.Default().With("tool", t.name)
	}

	taxonomiesRequest, ok := req.(*TaxonomiesRequest)
	if !ok {
		return nil, &ErrInvalidRequest{Err: fmt.Errorf("invalid request type: %T", req)}
	}

	if err := taxonomiesRequest.Validate(); err != nil {
		return nil, err
	}

	// Parse and validate the Hugo site URL
	siteURL, err := url.Parse(taxonomiesRequest.HugoSitePath)
	if err != nil {
		t.log.Error("Invalid Hugo site URL", "url", taxonomiesRequest.HugoSitePath, "error", err)
		return nil, &ErrInvalidRequest{Err: fmt.Errorf("invalid Hugo site URL: %w", err)}
	}

	// Ensure URL has scheme
	if siteURL.Scheme == "" {
		siteURL.Scheme = "https"
	}

	// Try common Hugo taxonomy endpoints with caching
	taxonomyEndpoints := []EndpointConfig{
		{path: "/taxonomies/index.json", validator: validateTaxonomyStructure},
		{path: "/index.json", validator: validateHugoIndex},
		{path: "/api/taxonomies.json", validator: validateTaxonomyStructure},
	}
	
	// Try individual taxonomy endpoints to discover what's available
	individualTaxonomyEndpoints := []string{
		"/categories/index.json",
		"/tags/index.json", 
		"/themes/index.json",
		"/methods/index.json",
		"/authors/index.json",
		"/series/index.json",
		"/topics/index.json",
	}

	var taxonomiesData []byte
	var found bool
	var usedEndpoint string

	for _, endpointConfig := range taxonomyEndpoints {
		taxonomyURL := siteURL.ResolveReference(&url.URL{Path: endpointConfig.path})
		cacheKey := t.cache.BuildKey(siteURL.String(), endpointConfig.path, nil)
		
		t.log.Debug("Trying taxonomy endpoint", "url", taxonomyURL.String(), "cache_key", cacheKey)

		// Check cache first
		if cachedData, hit := t.cache.Get(cacheKey); hit {
			t.log.Debug("Cache hit for endpoint", "url", taxonomyURL.String())
			if endpointConfig.validator(cachedData) {
				taxonomiesData = cachedData
				found = true
				usedEndpoint = taxonomyURL.String()
				break
			} else {
				t.log.Debug("Cached data failed validation, invalidating", "url", taxonomyURL.String())
				t.cache.Delete(cacheKey)
			}
		}

		// Fetch from network
		resp, err := t.httpClient.Get(taxonomyURL.String())
		if err != nil {
			t.log.Debug("Failed to fetch endpoint", "url", taxonomyURL.String(), "error", err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.log.Debug("Failed to read response body", "url", taxonomyURL.String(), "error", err)
				continue
			}

			// Validate response contains taxonomy data
			if endpointConfig.validator(body) {
				// Cache the validated response
				etag := resp.Header.Get("ETag")
				lastModified := resp.Header.Get("Last-Modified")
				t.cache.Set(cacheKey, body, etag, lastModified)
				
				taxonomiesData = body
				found = true
				usedEndpoint = taxonomyURL.String()
				t.log.Info("Found and cached taxonomies", "url", taxonomyURL.String())
				break
			} else {
				t.log.Debug("Response failed taxonomy validation", "url", taxonomyURL.String())
			}
		} else {
			t.log.Debug("HTTP error from endpoint", "url", taxonomyURL.String(), "status", resp.StatusCode)
		}
	}

	// If main endpoints failed, try individual taxonomy endpoints to discover what's available
	if !found {
		t.log.Debug("Main taxonomy endpoints failed, trying individual endpoints")
		discoveredTaxonomies := make(map[string]string)
		
		for _, endpoint := range individualTaxonomyEndpoints {
			taxonomyURL := siteURL.ResolveReference(&url.URL{Path: endpoint})
			cacheKey := t.cache.BuildKey(siteURL.String(), endpoint, nil)
			
			// Check cache first
			var responseData []byte
			if cachedData, hit := t.cache.Get(cacheKey); hit {
				responseData = cachedData
				t.log.Debug("Cache hit for individual taxonomy", "url", taxonomyURL.String())
			} else {
				// Try fetching from network
				resp, err := t.httpClient.Get(taxonomyURL.String())
				if err != nil {
					t.log.Debug("Failed to fetch individual taxonomy", "url", taxonomyURL.String(), "error", err)
					continue
				}
				defer resp.Body.Close()
				
				if resp.StatusCode == http.StatusOK {
					body, err := io.ReadAll(resp.Body)
					if err != nil {
						t.log.Debug("Failed to read individual taxonomy response", "url", taxonomyURL.String(), "error", err)
						continue
					}
					
					// Cache the response
					etag := resp.Header.Get("ETag")
					lastModified := resp.Header.Get("Last-Modified") 
					t.cache.Set(cacheKey, body, etag, lastModified)
					responseData = body
				} else {
					t.log.Debug("HTTP error from individual taxonomy", "url", taxonomyURL.String(), "status", resp.StatusCode)
					continue
				}
			}
			
			// Check if this looks like a valid taxonomy endpoint
			if gjson.ValidBytes(responseData) {
				parsed := gjson.ParseBytes(responseData)
				if taxonomies := parsed.Get("taxonomies"); taxonomies.Exists() && taxonomies.IsArray() {
					// Extract taxonomy name from endpoint path
					taxonomyName := strings.TrimSuffix(strings.TrimPrefix(endpoint, "/"), "/index.json")
					discoveredTaxonomies[taxonomyName] = taxonomyName
					t.log.Debug("Discovered taxonomy", "name", taxonomyName, "url", taxonomyURL.String())
				}
			}
		}
		
		// If we found any taxonomies, create a response
		if len(discoveredTaxonomies) > 0 {
			found = true
			usedEndpoint = "individual_discovery"
			// Create a simple taxonomies JSON from discovered ones
			taxonomiesData = []byte(fmt.Sprintf(`{"taxonomies": %s}`, formatTaxonomiesMap(discoveredTaxonomies)))
			t.log.Info("Successfully discovered taxonomies via individual endpoints", "count", len(discoveredTaxonomies))
		}
	}

	if !found {
		t.log.Error("No valid taxonomy data found", "site", taxonomiesRequest.HugoSitePath)
		return nil, &ErrInvalidRequest{Err: fmt.Errorf("no valid taxonomy data found at Hugo site: %s", taxonomiesRequest.HugoSitePath)}
	}

	// Parse taxonomies from validated JSON
	taxonomies := extractTaxonomies(taxonomiesData)

	// Format response with detailed error information
	responseData := fmt.Sprintf(`{
  "success": true,
  "taxonomies": %s,
  "metadata": {
    "source_endpoint": "%s",
    "taxonomy_count": %d,
    "cached": %s
  },
  "errors": []
}`, formatTaxonomies(taxonomies), usedEndpoint, len(taxonomies), "false")

	t.log.Info("Successfully retrieved taxonomies", "count", len(taxonomies), "site", taxonomiesRequest.HugoSitePath, "endpoint", usedEndpoint)
	return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(responseData)), nil
}

// EndpointConfig represents an endpoint with its validation function
type EndpointConfig struct {
	path      string
	validator func([]byte) bool
}

// validateTaxonomyStructure checks if the JSON contains taxonomy-like data
func validateTaxonomyStructure(data []byte) bool {
	if !gjson.ValidBytes(data) {
		return false
	}

	parsed := gjson.ParseBytes(data)
	
	// Check for direct taxonomies object
	if taxonomies := parsed.Get("taxonomies"); taxonomies.Exists() && taxonomies.IsObject() {
		return taxonomies.Map() != nil && len(taxonomies.Map()) > 0
	}
	
	// Check for common taxonomy keys
	commonTaxonomies := []string{"categories", "tags", "series", "authors", "topics"}
	foundTaxonomies := 0
	
	for _, tax := range commonTaxonomies {
		if parsed.Get(tax).Exists() {
			foundTaxonomies++
		}
	}
	
	// Consider it valid if we found at least one common taxonomy
	return foundTaxonomies > 0
}

// validateHugoIndex checks if the JSON is a Hugo index with potential taxonomy references
func validateHugoIndex(data []byte) bool {
	if !gjson.ValidBytes(data) {
		return false
	}

	parsed := gjson.ParseBytes(data)
	
	// Look for Hugo-specific structures that might contain taxonomies
	// Check for pages array with taxonomy metadata
	if pages := parsed.Get("pages"); pages.Exists() && pages.IsArray() {
		// If we have pages, check if any contain taxonomy information
		hasTaxonomyData := false
		pages.ForEach(func(key, page gjson.Result) bool {
			if page.Get("taxonomies").Exists() || 
			   page.Get("categories").Exists() || 
			   page.Get("tags").Exists() {
				hasTaxonomyData = true
				return false // Stop iteration
			}
			return true
		})
		return hasTaxonomyData
	}
	
	// Fall back to basic taxonomy validation
	return validateTaxonomyStructure(data)
}

// extractTaxonomies parses taxonomies from validated JSON data
func extractTaxonomies(data []byte) map[string]string {
	taxonomies := make(map[string]string)
	parsed := gjson.ParseBytes(data)

	// Try different JSON structures that Hugo might use
	if result := parsed.Get("taxonomies"); result.Exists() && result.IsObject() {
		// Direct taxonomies object
		result.ForEach(func(key, value gjson.Result) bool {
			taxonomies[key.String()] = value.String()
			return true
		})
	} else {
		// Look for common taxonomy keys in the root
		commonTaxonomies := []string{"categories", "tags", "series", "authors", "topics", "types"}
		for _, tax := range commonTaxonomies {
			if result := parsed.Get(tax); result.Exists() {
				taxonomies[tax] = tax
			}
		}
		
		// Also check for taxonomy keys that end with common patterns
		parsed.ForEach(func(key, value gjson.Result) bool {
			keyStr := key.String()
			if strings.HasSuffix(keyStr, "_taxonomy") || strings.HasSuffix(keyStr, "_tax") {
				baseName := strings.TrimSuffix(strings.TrimSuffix(keyStr, "_taxonomy"), "_tax")
				taxonomies[baseName] = baseName
			}
			return true
		})
	}

	return taxonomies
}

// formatTaxonomies formats the taxonomies map as a JSON string
func formatTaxonomies(taxonomies map[string]string) string {
	if len(taxonomies) == 0 {
		return "{}"
	}

	var parts []string
	for key, value := range taxonomies {
		parts = append(parts, fmt.Sprintf(`"%s": "%s"`, key, value))
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

// formatTaxonomiesMap formats the discovered taxonomies map as a JSON string
func formatTaxonomiesMap(taxonomies map[string]string) string {
	if len(taxonomies) == 0 {
		return "{}"
	}

	var parts []string
	for key, value := range taxonomies {
		parts = append(parts, fmt.Sprintf(`"%s": "%s"`, key, value))
	}

	return "{" + strings.Join(parts, ", ") + "}"
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
