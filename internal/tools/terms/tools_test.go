package terms

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tool, err := New()
	require.NoError(t, err)
	assert.NotNil(t, tool)
	assert.Equal(t, "hugo_reader_get_taxonomy_terms", tool.Name())
	assert.Equal(t, "Get all terms (values) for a specific taxonomy from a Hugo site. For example, get all 'categories' or 'tags' used on the site. Use after getting taxonomies to explore available terms.", tool.Description())
	assert.NotNil(t, tool.httpClient)
}

func TestTaxonomyTermsRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     *TaxonomyTermsRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: &TaxonomyTermsRequest{
				HugoSitePath: "https://example.com",
				Taxonomy:     "categories",
			},
			wantErr: false,
		},
		{
			name: "missing hugo_site_path",
			req: &TaxonomyTermsRequest{
				HugoSitePath: "",
				Taxonomy:     "categories",
			},
			wantErr: true,
		},
		{
			name: "missing taxonomy",
			req: &TaxonomyTermsRequest{
				HugoSitePath: "https://example.com",
				Taxonomy:     "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateTermsStructure(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		taxonomy string
		expected bool
	}{
		{
			name:     "valid terms array",
			data:     `{"terms": ["tech", "golang", "programming"]}`,
			taxonomy: "categories",
			expected: true,
		},
		{
			name:     "valid terms object",
			data:     `{"terms": {"tech": 5, "golang": 3, "programming": 8}}`,
			taxonomy: "categories",
			expected: true,
		},
		{
			name:     "valid taxonomy-specific array",
			data:     `{"categories": ["tech", "golang", "programming"]}`,
			taxonomy: "categories",
			expected: true,
		},
		{
			name:     "valid pages with taxonomy",
			data:     `{"pages": [{"title": "Post 1", "categories": ["tech"]}, {"title": "Post 2", "categories": ["golang"]}]}`,
			taxonomy: "categories",
			expected: true,
		},
		{
			name:     "valid taxonomies array format",
			data:     `{"taxonomies": [{"name": "tech", "count": 5, "url": "/categories/tech/"}, {"name": "golang", "count": 3, "url": "/categories/golang/"}]}`,
			taxonomy: "categories",
			expected: true,
		},
		{
			name:     "real site taxonomies format",
			data:     `{"taxonomies":[{"name":"Appetizer","count":2,"url":"/categories/appetizer/"},{"name":"Breakfast","count":1,"url":"/categories/breakfast/"}]}`,
			taxonomy: "categories",
			expected: true,
		},
		{
			name:     "invalid JSON",
			data:     `{invalid json}`,
			taxonomy: "categories",
			expected: false,
		},
		{
			name:     "no terms data",
			data:     `{"title": "Some Page", "content": "Page content"}`,
			taxonomy: "categories",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateTermsStructure([]byte(tt.data), tt.taxonomy)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateHugoIndexForTerms(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		taxonomy string
		expected bool
	}{
		{
			name:     "hugo index with pages containing taxonomy",
			data:     `{"pages": [{"title": "Post 1", "categories": ["tech"]}, {"title": "Post 2", "tags": ["go"]}]}`,
			taxonomy: "categories",
			expected: true,
		},
		{
			name:     "hugo index without taxonomy in pages",
			data:     `{"pages": [{"title": "Post 1", "content": "Some content"}]}`,
			taxonomy: "categories",
			expected: false,
		},
		{
			name:     "taxonomy in root",
			data:     `{"categories": ["tech", "go"], "tags": ["programming"]}`,
			taxonomy: "categories",
			expected: true,
		},
		{
			name:     "invalid JSON",
			data:     `{invalid}`,
			taxonomy: "categories",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateHugoIndexForTerms([]byte(tt.data), tt.taxonomy)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractTerms(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		taxonomy string
		expected []string
	}{
		{
			name:     "terms array",
			data:     `{"terms": ["tech", "golang", "programming"]}`,
			taxonomy: "categories",
			expected: []string{"tech", "golang", "programming"},
		},
		{
			name:     "terms object",
			data:     `{"terms": {"tech": 5, "golang": 3}}`,
			taxonomy: "categories",
			expected: []string{"tech", "golang"},
		},
		{
			name:     "taxonomy-specific array",
			data:     `{"categories": ["tech", "golang"]}`,
			taxonomy: "categories",
			expected: []string{"tech", "golang"},
		},
		{
			name:     "pages with taxonomy",
			data:     `{"pages": [{"categories": ["tech"]}, {"categories": ["golang", "tech"]}]}`,
			taxonomy: "categories",
			expected: []string{"tech", "golang"},
		},
		{
			name:     "taxonomies array format",
			data:     `{"taxonomies": [{"name": "tech", "count": 5}, {"name": "golang", "count": 3}]}`,
			taxonomy: "categories",
			expected: []string{"tech", "golang"},
		},
		{
			name:     "real site taxonomies format",
			data:     `{"taxonomies":[{"name":"Appetizer","count":2,"url":"/categories/appetizer/"},{"name":"Breakfast","count":1,"url":"/categories/breakfast/"}]}`,
			taxonomy: "categories",
			expected: []string{"Appetizer", "Breakfast"},
		},
		{
			name:     "empty data",
			data:     `{}`,
			taxonomy: "categories",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTerms([]byte(tt.data), tt.taxonomy)
			
			// For pages extraction, we use a map so order isn't guaranteed
			// Convert to sets for comparison
			if tt.name == "pages with taxonomy" {
				assert.ElementsMatch(t, tt.expected, result)
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestFormatTerms(t *testing.T) {
	tests := []struct {
		name     string
		terms    []string
		expected string
	}{
		{
			name:     "empty terms",
			terms:    []string{},
			expected: "[]",
		},
		{
			name:     "single term",
			terms:    []string{"tech"},
			expected: "[\n    \"tech\"\n  ]",
		},
		{
			name:     "multiple terms",
			terms:    []string{"tech", "golang"},
			expected: "[\n    \"tech\",\n    \"golang\"\n  ]",
		},
		{
			name:     "terms with quotes",
			terms:    []string{`term with "quotes"`},
			expected: "[\n    \"term with \\\"quotes\\\"\"\n  ]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTerms(tt.terms)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTool_SetLogger(t *testing.T) {
	tool, err := New()
	require.NoError(t, err)

	// Test with nil logger
	tool.SetLogger(nil)
	assert.NotNil(t, tool.log)

	// Test that it doesn't panic with valid logger
	// We can't easily test the logger content without more setup
}