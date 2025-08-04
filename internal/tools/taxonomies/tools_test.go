package taxonomies

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tool, err := New()
	require.NoError(t, err)
	assert.NotNil(t, tool)
	assert.Equal(t, "hugo_reader_get_taxonomies", tool.Name())
	assert.Equal(t, "Get all taxonomies defined in a Hugo site (e.g., categories, tags, authors). Returns the taxonomy names and their configuration. Use this first to understand the site's content organization.", tool.Description())
	assert.NotNil(t, tool.httpClient)
}

func TestTaxonomiesRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     *TaxonomiesRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: &TaxonomiesRequest{
				HugoSitePath: "https://example.com",
			},
			wantErr: false,
		},
		{
			name: "missing hugo_site_path",
			req: &TaxonomiesRequest{
				HugoSitePath: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.IsType(t, &ErrHugoSitePathRequired{}, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFormatTaxonomies(t *testing.T) {
	tests := []struct {
		name       string
		taxonomies map[string]string
		want       string
	}{
		{
			name:       "empty taxonomies",
			taxonomies: map[string]string{},
			want:       "{}",
		},
		{
			name: "single taxonomy",
			taxonomies: map[string]string{
				"categories": "categories",
			},
			want: "{\n    \"categories\": \"categories\"\n  }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTaxonomies(tt.taxonomies)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTool_SetLogger(t *testing.T) {
	tool, err := New()
	require.NoError(t, err)

	// Test with nil logger
	tool.SetLogger(nil)
	assert.NotNil(t, tool.log)

	// Test with valid logger
	// Note: We can't easily test the logger without more complex setup
	// This test just ensures the method doesn't panic
}

func TestValidateTaxonomyStructure(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		expected bool
	}{
		{
			name:     "valid direct taxonomies object",
			data:     `{"taxonomies": {"categories": "categories", "tags": "tags"}}`,
			expected: true,
		},
		{
			name:     "valid common taxonomy keys",
			data:     `{"categories": ["tech", "go"], "tags": ["programming"]}`,
			expected: true,
		},
		{
			name:     "invalid JSON",
			data:     `{invalid json}`,
			expected: false,
		},
		{
			name:     "empty taxonomies object",
			data:     `{"taxonomies": {}}`,
			expected: false,
		},
		{
			name:     "no taxonomy data",
			data:     `{"title": "Some Page", "content": "Page content"}`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateTaxonomyStructure([]byte(tt.data))
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateHugoIndex(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		expected bool
	}{
		{
			name:     "hugo index with pages containing taxonomies",
			data:     `{"pages": [{"title": "Post 1", "categories": ["tech"]}, {"title": "Post 2", "tags": ["go"]}]}`,
			expected: true,
		},
		{
			name:     "hugo index without taxonomy data in pages",
			data:     `{"pages": [{"title": "Post 1", "content": "Some content"}]}`,
			expected: false,
		},
		{
			name:     "fallback to direct taxonomies",
			data:     `{"categories": ["tech", "go"], "tags": ["programming"]}`,
			expected: true,
		},
		{
			name:     "invalid JSON",
			data:     `{invalid}`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateHugoIndex([]byte(tt.data))
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractTaxonomies(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		expected map[string]string
	}{
		{
			name: "direct taxonomies object",
			data: `{"taxonomies": {"categories": "categories", "tags": "tags"}}`,
			expected: map[string]string{
				"categories": "categories",
				"tags":       "tags",
			},
		},
		{
			name: "common taxonomy keys",
			data: `{"categories": ["tech"], "tags": ["go"], "series": ["tutorials"]}`,
			expected: map[string]string{
				"categories": "categories",
				"tags":       "tags",
				"series":     "series",
			},
		},
		{
			name: "taxonomy patterns",
			data: `{"post_taxonomy": ["news"], "content_tax": ["articles"]}`,
			expected: map[string]string{
				"post":    "post",
				"content": "content",
			},
		},
		{
			name:     "empty data",
			data:     `{}`,
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTaxonomies([]byte(tt.data))
			assert.Equal(t, tt.expected, result)
		})
	}
}