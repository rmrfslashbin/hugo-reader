package discovery

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

// Tool discovers available content and structure in Hugo sites.
type Tool struct {
	log        *slog.Logger
	name       string
	description string
	httpClient *http.Client
	cache      *cache.Cache
}

// DiscoveryRequest represents the request parameters for site discovery.
type DiscoveryRequest struct {
	HugoSitePath string `json:"hugo_site_path" jsonschema:"title=Hugo Site Path"`
	DiscoveryType string `json:"discovery_type,omitempty" jsonschema:"enum=overview,enum=sections,enum=pages,enum=sitemap,title=Discovery Type"`
	Limit        int    `json:"limit,omitempty" jsonschema:"title=Result Limit,minimum=1,maximum=200"`
}

// New creates a new Tool.
func New(opts ...ToolOption) (*Tool, error) {
	tool := &Tool{
		name:        "hugo_reader_discover_site",
		description: "Discover available content and structure in Hugo sites. Types: 'overview' (site structure), 'sections' (content sections), 'pages' (all pages), 'sitemap' (from sitemap.xml). Use this to explore what content is available.",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		cache: cache.New(cache.WithTTL(10 * time.Minute)), // Longer TTL for discovery
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
			t.cache = cache.New(cache.WithLogger(logger), cache.WithTTL(10*time.Minute))
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
func (r *DiscoveryRequest) Validate() error {
	if r.HugoSitePath == "" {
		return fmt.Errorf("hugo_site_path is required")
	}
	
	// Set default discovery type if not specified
	if r.DiscoveryType == "" {
		r.DiscoveryType = "overview"
	}
	
	// Validate discovery type
	validTypes := map[string]bool{"overview": true, "sections": true, "pages": true, "sitemap": true}
	if !validTypes[r.DiscoveryType] {
		return fmt.Errorf("invalid discovery_type: %s (must be: overview, sections, pages, or sitemap)", r.DiscoveryType)
	}
	
	// Set default limit if not specified or validate
	if r.Limit == 0 {
		r.Limit = 50 // Default limit
	} else if r.Limit < 1 || r.Limit > 200 {
		return fmt.Errorf("limit must be between 1 and 200")
	}
	
	return nil
}

// Execute discovers site content and structure.
func (t *Tool) Execute(req tools.Request) (*mcp_golang.ToolResponse, error) {
	// Check if logger is initialized
	if t.log == nil {
		t.log = slog.Default().With("tool", t.name)
	}

	discoveryRequest, ok := req.(*DiscoveryRequest)
	if !ok {
		return nil, fmt.Errorf("invalid request type: %T", req)
	}

	if err := discoveryRequest.Validate(); err != nil {
		return nil, err
	}

	// Parse and validate the Hugo site URL
	siteURL, err := url.Parse(discoveryRequest.HugoSitePath)
	if err != nil {
		t.log.Error("Invalid Hugo site URL", "url", discoveryRequest.HugoSitePath, "error", err)
		return nil, fmt.Errorf("invalid Hugo site URL: %w", err)
	}

	// Ensure URL has scheme
	if siteURL.Scheme == "" {
		siteURL.Scheme = "https"
	}

	var results []map[string]interface{}
	var metadata map[string]interface{}

	switch discoveryRequest.DiscoveryType {
	case "overview":
		results, metadata, err = t.discoverOverview(siteURL, discoveryRequest.Limit)
	case "sections":
		results, metadata, err = t.discoverSections(siteURL, discoveryRequest.Limit)
	case "pages":
		results, metadata, err = t.discoverPages(siteURL, discoveryRequest.Limit)
	case "sitemap":
		results, metadata, err = t.discoverSitemap(siteURL, discoveryRequest.Limit)
	default:
		return nil, fmt.Errorf("unsupported discovery type: %s", discoveryRequest.DiscoveryType)
	}

	if err != nil {
		t.log.Error("Discovery failed", "type", discoveryRequest.DiscoveryType, "error", err)
		return nil, fmt.Errorf("discovery failed: %w", err)
	}

	// Format response
	responseData := fmt.Sprintf(`{
  "success": true,
  "discovery_type": "%s",
  "results": %s,
  "metadata": %s,
  "errors": []
}`, discoveryRequest.DiscoveryType, formatResults(results), formatMetadata(metadata))

	t.log.Info("Discovery completed", "type", discoveryRequest.DiscoveryType, "results", len(results), "site", discoveryRequest.HugoSitePath)
	return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(responseData)), nil
}

// discoverOverview provides a general overview of site structure
func (t *Tool) discoverOverview(siteURL *url.URL, limit int) ([]map[string]interface{}, map[string]interface{}, error) {
	results := []map[string]interface{}{}
	
	// Try multiple discovery endpoints
	endpoints := []string{
		"/index.json",
		"/api/index.json",
		"/sitemap.xml",
		"/robots.txt",
	}
	
	foundEndpoints := []string{}
	
	for _, endpoint := range endpoints {
		endpointURL := siteURL.ResolveReference(&url.URL{Path: endpoint})
		resp, err := t.httpClient.Get(endpointURL.String())
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		
		if resp.StatusCode == http.StatusOK {
			foundEndpoints = append(foundEndpoints, endpoint)
			
			// Try to extract some basic info
			if strings.HasSuffix(endpoint, ".json") {
				body, err := io.ReadAll(resp.Body)
				if err == nil && gjson.ValidBytes(body) {
					parsed := gjson.ParseBytes(body)
					
					result := map[string]interface{}{
						"endpoint": endpoint,
						"type": "json",
						"url": endpointURL.String(),
					}
					
					// Extract basic structure info
					if pages := parsed.Get("pages"); pages.Exists() && pages.IsArray() {
						result["pages_count"] = len(pages.Array())
					}
					if sections := parsed.Get("sections"); sections.Exists() {
						result["sections"] = sections.Value()
					}
					if taxonomies := parsed.Get("taxonomies"); taxonomies.Exists() {
						result["taxonomies"] = taxonomies.Value()
					}
					
					results = append(results, result)
				}
			} else {
				results = append(results, map[string]interface{}{
					"endpoint": endpoint,
					"type": "other",
					"url": endpointURL.String(),
					"status": "available",
				})
			}
		}
	}
	
	metadata := map[string]interface{}{
		"discovery_method": "overview",
		"endpoints_found": len(foundEndpoints),
		"endpoints_checked": len(endpoints),
		"available_endpoints": foundEndpoints,
	}
	
	return results, metadata, nil
}

// discoverSections finds content sections
func (t *Tool) discoverSections(siteURL *url.URL, limit int) ([]map[string]interface{}, map[string]interface{}, error) {
	// Try to get sections from index
	indexURL := siteURL.ResolveReference(&url.URL{Path: "/index.json"})
	resp, err := t.httpClient.Get(indexURL.String())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch index: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("index not available (status: %d)", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read index: %w", err)
	}
	
	if !gjson.ValidBytes(body) {
		return nil, nil, fmt.Errorf("invalid JSON in index")
	}
	
	parsed := gjson.ParseBytes(body)
	results := []map[string]interface{}{}
	sections := make(map[string]int)
	
	// Extract sections from pages
	if pages := parsed.Get("pages"); pages.Exists() && pages.IsArray() {
		pages.ForEach(func(key, page gjson.Result) bool {
			if len(results) >= limit {
				return false
			}
			
			if section := page.Get("section"); section.Exists() {
				sectionName := section.String()
				sections[sectionName]++
			}
			
			if url := page.Get("url"); url.Exists() {
				urlStr := url.String()
				parts := strings.Split(strings.Trim(urlStr, "/"), "/")
				if len(parts) > 0 {
					sections[parts[0]]++
				}
			}
			
			return true
		})
	}
	
	// Convert sections map to results
	for section, count := range sections {
		if len(results) >= limit {
			break
		}
		results = append(results, map[string]interface{}{
			"section": section,
			"count": count,
			"example_path": fmt.Sprintf("/%s/", section),
		})
	}
	
	metadata := map[string]interface{}{
		"discovery_method": "sections",
		"total_sections": len(sections),
		"source": "index.json",
	}
	
	return results, metadata, nil
}

// discoverPages finds available pages
func (t *Tool) discoverPages(siteURL *url.URL, limit int) ([]map[string]interface{}, map[string]interface{}, error) {
	// Try to get pages from index
	indexURL := siteURL.ResolveReference(&url.URL{Path: "/index.json"})
	resp, err := t.httpClient.Get(indexURL.String())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch index: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("index not available (status: %d)", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read index: %w", err)
	}
	
	if !gjson.ValidBytes(body) {
		return nil, nil, fmt.Errorf("invalid JSON in index")
	}
	
	parsed := gjson.ParseBytes(body)
	results := []map[string]interface{}{}
	
	// Extract pages
	if pages := parsed.Get("pages"); pages.Exists() && pages.IsArray() {
		pages.ForEach(func(key, page gjson.Result) bool {
			if len(results) >= limit {
				return false
			}
			
			result := map[string]interface{}{}
			
			if title := page.Get("title"); title.Exists() {
				result["title"] = title.String()
			}
			if url := page.Get("url"); url.Exists() {
				result["url"] = url.String()
				result["path"] = url.String()
			}
			if date := page.Get("date"); date.Exists() {
				result["date"] = date.String()
			}
			if section := page.Get("section"); section.Exists() {
				result["section"] = section.String()
			}
			
			results = append(results, result)
			return true
		})
	}
	
	metadata := map[string]interface{}{
		"discovery_method": "pages",
		"total_found": len(results),
		"source": "index.json",
		"limited": len(results) >= limit,
	}
	
	return results, metadata, nil
}

// discoverSitemap extracts URLs from sitemap.xml
func (t *Tool) discoverSitemap(siteURL *url.URL, limit int) ([]map[string]interface{}, map[string]interface{}, error) {
	sitemapURL := siteURL.ResolveReference(&url.URL{Path: "/sitemap.xml"})
	resp, err := t.httpClient.Get(sitemapURL.String())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch sitemap: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("sitemap not available (status: %d)", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read sitemap: %w", err)
	}
	
	bodyStr := string(body)
	results := []map[string]interface{}{}
	
	// Simple XML parsing for URLs
	lines := strings.Split(bodyStr, "\n")
	for _, line := range lines {
		if len(results) >= limit {
			break
		}
		
		line = strings.TrimSpace(line)
		if strings.Contains(line, "<loc>") && strings.Contains(line, "</loc>") {
			start := strings.Index(line, "<loc>") + 5
			end := strings.Index(line, "</loc>")
			if start < end {
				urlStr := line[start:end]
				if strings.HasPrefix(urlStr, "http") {
					path := strings.TrimPrefix(urlStr, siteURL.String())
					results = append(results, map[string]interface{}{
						"url": urlStr,
						"path": path,
						"source": "sitemap.xml",
					})
				}
			}
		}
	}
	
	metadata := map[string]interface{}{
		"discovery_method": "sitemap",
		"total_found": len(results),
		"source": "sitemap.xml",
		"limited": len(results) >= limit,
	}
	
	return results, metadata, nil
}

// Formatting functions
func formatResults(results []map[string]interface{}) string {
	if len(results) == 0 {
		return "[]"
	}
	
	var parts []string
	for _, result := range results {
		parts = append(parts, fmt.Sprintf("    %s", formatResult(result)))
	}
	
	return "[\n" + strings.Join(parts, ",\n") + "\n  ]"
}

func formatResult(result map[string]interface{}) string {
	var parts []string
	
	for key, value := range result {
		switch v := value.(type) {
		case string:
			parts = append(parts, fmt.Sprintf(`"%s": "%s"`, key, strings.ReplaceAll(v, `"`, `\"`)))
		case int:
			parts = append(parts, fmt.Sprintf(`"%s": %d`, key, v))
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
		case int:
			parts = append(parts, fmt.Sprintf(`"%s": %d`, key, v))
		case []string:
			var items []string
			for _, item := range v {
				items = append(items, fmt.Sprintf(`"%s"`, item))
			}
			parts = append(parts, fmt.Sprintf(`"%s": [%s]`, key, strings.Join(items, ", ")))
		case bool:
			parts = append(parts, fmt.Sprintf(`"%s": %t`, key, v))
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