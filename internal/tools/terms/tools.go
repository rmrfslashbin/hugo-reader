package terms

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

// Tool retrieves terms for a specific taxonomy from Hugo sites.
type Tool struct {
	log        *slog.Logger
	name       string
	description string
	httpClient *http.Client
	cache      *cache.Cache
}

// TaxonomyTermsRequest represents the request parameters for the taxonomy terms tool.
type TaxonomyTermsRequest struct {
	HugoSitePath string `json:"hugo_site_path" jsonschema:"title=Hugo Site Path"`
	Taxonomy     string `json:"taxonomy" jsonschema:"title=Taxonomy Name"`
}

// EndpointConfig represents an endpoint with its validation function
type EndpointConfig struct {
	path      string
	validator func([]byte, string) bool
}

// New creates a new Tool.
func New(opts ...ToolOption) (*Tool, error) {
	tool := &Tool{
		name:        "hugo_reader_get_taxonomy_terms",
		description: "Get all terms (values) for a specific taxonomy from a Hugo site. For example, get all 'categories' or 'tags' used on the site. Use after getting taxonomies to explore available terms.",
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
func (r *TaxonomyTermsRequest) Validate() error {
	if r.HugoSitePath == "" {
		return fmt.Errorf("hugo_site_path is required")
	}
	if r.Taxonomy == "" {
		return fmt.Errorf("taxonomy is required")
	}
	return nil
}

// Execute retrieves terms for a specific taxonomy from a Hugo site.
func (t *Tool) Execute(req tools.Request) (*mcp_golang.ToolResponse, error) {
	// Check if logger is initialized
	if t.log == nil {
		t.log = slog.Default().With("tool", t.name)
	}

	termsRequest, ok := req.(*TaxonomyTermsRequest)
	if !ok {
		return nil, fmt.Errorf("invalid request type: %T", req)
	}

	if err := termsRequest.Validate(); err != nil {
		return nil, err
	}

	// Parse and validate the Hugo site URL
	siteURL, err := url.Parse(termsRequest.HugoSitePath)
	if err != nil {
		t.log.Error("Invalid Hugo site URL", "url", termsRequest.HugoSitePath, "error", err)
		return nil, fmt.Errorf("invalid Hugo site URL: %w", err)
	}

	// Ensure URL has scheme
	if siteURL.Scheme == "" {
		siteURL.Scheme = "https"
	}

	// Try common Hugo taxonomy terms endpoints
	taxonomyEndpoints := []EndpointConfig{
		{path: fmt.Sprintf("/taxonomies/%s/index.json", termsRequest.Taxonomy), validator: validateTermsStructure},
		{path: fmt.Sprintf("/%s/index.json", termsRequest.Taxonomy), validator: validateTermsStructure},
		{path: fmt.Sprintf("/api/taxonomies/%s.json", termsRequest.Taxonomy), validator: validateTermsStructure},
		{path: "/index.json", validator: validateHugoIndexForTerms},
	}

	var termsData []byte
	var found bool
	var usedEndpoint string

	for _, endpointConfig := range taxonomyEndpoints {
		taxonomyURL := siteURL.ResolveReference(&url.URL{Path: endpointConfig.path})
		cacheKey := t.cache.BuildKey(siteURL.String(), endpointConfig.path, map[string]string{"taxonomy": termsRequest.Taxonomy})
		
		t.log.Debug("Trying taxonomy terms endpoint", "url", taxonomyURL.String(), "cache_key", cacheKey)

		// Check cache first
		if cachedData, hit := t.cache.Get(cacheKey); hit {
			t.log.Debug("Cache hit for terms endpoint", "url", taxonomyURL.String())
			if endpointConfig.validator(cachedData, termsRequest.Taxonomy) {
				termsData = cachedData
				found = true
				usedEndpoint = taxonomyURL.String()
				break
			} else {
				t.log.Debug("Cached terms data failed validation, invalidating", "url", taxonomyURL.String())
				t.cache.Delete(cacheKey)
			}
		}

		// Fetch from network
		resp, err := t.httpClient.Get(taxonomyURL.String())
		if err != nil {
			t.log.Debug("Failed to fetch terms endpoint", "url", taxonomyURL.String(), "error", err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.log.Debug("Failed to read terms response body", "url", taxonomyURL.String(), "error", err)
				continue
			}

			// Validate response contains taxonomy terms data
			if endpointConfig.validator(body, termsRequest.Taxonomy) {
				// Cache the validated response
				etag := resp.Header.Get("ETag")
				lastModified := resp.Header.Get("Last-Modified")
				t.cache.Set(cacheKey, body, etag, lastModified)
				
				termsData = body
				found = true
				usedEndpoint = taxonomyURL.String()
				t.log.Info("Found and cached taxonomy terms", "url", taxonomyURL.String(), "taxonomy", termsRequest.Taxonomy)
				break
			} else {
				t.log.Debug("Response failed terms validation", "url", taxonomyURL.String(), "taxonomy", termsRequest.Taxonomy)
			}
		} else {
			t.log.Debug("HTTP error from terms endpoint", "url", taxonomyURL.String(), "status", resp.StatusCode)
		}
	}

	if !found {
		t.log.Error("No valid taxonomy terms data found", "site", termsRequest.HugoSitePath, "taxonomy", termsRequest.Taxonomy)
		return nil, fmt.Errorf("no valid taxonomy terms data found for taxonomy '%s' at Hugo site: %s", termsRequest.Taxonomy, termsRequest.HugoSitePath)
	}

	// Extract terms from validated JSON
	terms := extractTerms(termsData, termsRequest.Taxonomy)

	// Format response with detailed metadata
	responseData := fmt.Sprintf(`{
  "success": true,
  "taxonomy": "%s",
  "terms": %s,
  "metadata": {
    "source_endpoint": "%s",
    "term_count": %d,
    "cached": %s
  },
  "errors": []
}`, termsRequest.Taxonomy, formatTerms(terms), usedEndpoint, len(terms), "false")

	t.log.Info("Successfully retrieved taxonomy terms", "count", len(terms), "site", termsRequest.HugoSitePath, "taxonomy", termsRequest.Taxonomy, "endpoint", usedEndpoint)
	return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(responseData)), nil
}

// validateTermsStructure checks if the JSON contains valid taxonomy terms data
func validateTermsStructure(data []byte, taxonomy string) bool {
	if !gjson.ValidBytes(data) {
		return false
	}

	parsed := gjson.ParseBytes(data)
	
	// Check for direct terms array or object
	if terms := parsed.Get("terms"); terms.Exists() {
		return terms.IsArray() || terms.IsObject()
	}
	
	// Check for taxonomy-specific structure
	if taxTerms := parsed.Get(taxonomy); taxTerms.Exists() {
		return taxTerms.IsArray() || taxTerms.IsObject()
	}
	
	// Check for Hugo-style taxonomies array (common format)
	if taxonomies := parsed.Get("taxonomies"); taxonomies.Exists() && taxonomies.IsArray() {
		// This format has objects with name, count, url fields
		if len(taxonomies.Array()) > 0 {
			firstItem := taxonomies.Array()[0]
			return firstItem.Get("name").Exists() && (firstItem.Get("count").Exists() || firstItem.Get("url").Exists())
		}
	}
	
	// Check for pages with the taxonomy
	if pages := parsed.Get("pages"); pages.Exists() && pages.IsArray() {
		hasTermsData := false
		pages.ForEach(func(key, page gjson.Result) bool {
			if page.Get(taxonomy).Exists() {
				hasTermsData = true
				return false // Stop iteration
			}
			return true
		})
		return hasTermsData
	}
	
	return false
}

// validateHugoIndexForTerms checks if the Hugo index contains terms for the specified taxonomy
func validateHugoIndexForTerms(data []byte, taxonomy string) bool {
	if !gjson.ValidBytes(data) {
		return false
	}

	parsed := gjson.ParseBytes(data)
	
	// Look for pages with the specific taxonomy
	if pages := parsed.Get("pages"); pages.Exists() && pages.IsArray() {
		hasTermsData := false
		pages.ForEach(func(key, page gjson.Result) bool {
			if page.Get(taxonomy).Exists() {
				hasTermsData = true
				return false // Stop iteration
			}
			return true
		})
		return hasTermsData
	}
	
	// Check if taxonomy exists in root
	return parsed.Get(taxonomy).Exists()
}

// extractTerms parses terms from validated JSON data for a specific taxonomy
func extractTerms(data []byte, taxonomy string) []string {
	var terms []string
	parsed := gjson.ParseBytes(data)

	// Try different JSON structures that Hugo might use
	if result := parsed.Get("terms"); result.Exists() {
		if result.IsArray() {
			result.ForEach(func(key, value gjson.Result) bool {
				terms = append(terms, value.String())
				return true
			})
		} else if result.IsObject() {
			result.ForEach(func(key, value gjson.Result) bool {
				terms = append(terms, key.String())
				return true
			})
		}
	} else if result := parsed.Get(taxonomy); result.Exists() {
		if result.IsArray() {
			result.ForEach(func(key, value gjson.Result) bool {
				if value.Type == gjson.String {
					terms = append(terms, value.String())
				} else if value.Type == gjson.JSON {
					// Extract term name from object
					if name := value.Get("name"); name.Exists() {
						terms = append(terms, name.String())
					} else if title := value.Get("title"); title.Exists() {
						terms = append(terms, title.String())
					}
				}
				return true
			})
		} else if result.IsObject() {
			result.ForEach(func(key, value gjson.Result) bool {
				terms = append(terms, key.String())
				return true
			})
		}
	} else if taxonomies := parsed.Get("taxonomies"); taxonomies.Exists() && taxonomies.IsArray() {
		// Extract terms from Hugo-style taxonomies array
		taxonomies.ForEach(func(key, taxonomyItem gjson.Result) bool {
			if name := taxonomyItem.Get("name"); name.Exists() {
				terms = append(terms, name.String())
			}
			return true
		})
	} else if pages := parsed.Get("pages"); pages.Exists() && pages.IsArray() {
		// Extract terms from pages
		termMap := make(map[string]bool)
		pages.ForEach(func(key, page gjson.Result) bool {
			if pageTaxonomy := page.Get(taxonomy); pageTaxonomy.Exists() {
				if pageTaxonomy.IsArray() {
					pageTaxonomy.ForEach(func(k, term gjson.Result) bool {
						termMap[term.String()] = true
						return true
					})
				} else if pageTaxonomy.Type == gjson.String {
					termMap[pageTaxonomy.String()] = true
				}
			}
			return true
		})
		
		for term := range termMap {
			terms = append(terms, term)
		}
	}

	return terms
}

// formatTerms formats the terms slice as a JSON array string
func formatTerms(terms []string) string {
	if len(terms) == 0 {
		return "[]"
	}

	var quotedTerms []string
	for _, term := range terms {
		quotedTerms = append(quotedTerms, fmt.Sprintf(`"%s"`, strings.ReplaceAll(term, `"`, `\"`)))
	}

	return "[\n    " + strings.Join(quotedTerms, ",\n    ") + "\n  ]"
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