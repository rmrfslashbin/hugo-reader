package search

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tool, err := New()
	require.NoError(t, err)
	assert.NotNil(t, tool)
	assert.Equal(t, "hugo_reader_search", tool.Name())
	assert.Equal(t, "Search content across a Hugo site with support for filters and Hugo-specific search indices", tool.Description())
	assert.NotNil(t, tool.httpClient)
}

func TestSearchRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     *SearchRequest
		wantErr bool
	}{
		{
			name: "valid request with defaults",
			req: &SearchRequest{
				HugoSitePath: "https://example.com",
				Query:        "golang",
			},
			wantErr: false,
		},
		{
			name: "valid request with all fields",
			req: &SearchRequest{
				HugoSitePath: "https://example.com",
				Query:        "golang",
				ContentType:  "post",
				Taxonomy:     "categories",
				Term:         "tech",
				Limit:        10,
			},
			wantErr: false,
		},
		{
			name: "missing hugo_site_path",
			req: &SearchRequest{
				HugoSitePath: "",
				Query:        "golang",
			},
			wantErr: true,
		},
		{
			name: "missing query",
			req: &SearchRequest{
				HugoSitePath: "https://example.com",
				Query:        "",
			},
			wantErr: true,
		},
		{
			name: "limit too high",
			req: &SearchRequest{
				HugoSitePath: "https://example.com",
				Query:        "golang",
				Limit:        150,
			},
			wantErr: true,
		},
		{
			name: "limit too low",
			req: &SearchRequest{
				HugoSitePath: "https://example.com",
				Query:        "golang",
				Limit:        0,
			},
			wantErr: false, // 0 gets set to default (20)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Check default limit is set
				if tt.req.Limit == 0 {
					assert.Equal(t, 20, tt.req.Limit)
				}
			}
		})
	}
}

func TestValidateSearchResults(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		expected bool
	}{
		{
			name:     "valid search results with results array",
			data:     `{"results": [{"title": "Post 1", "content": "Content here"}]}`,
			expected: true,
		},
		{
			name:     "valid search results with hits array",
			data:     `{"hits": [{"title": "Post 1", "content": "Content here"}]}`,
			expected: true,
		},
		{
			name:     "valid direct array",
			data:     `[{"title": "Post 1", "content": "Content here"}]`,
			expected: true,
		},
		{
			name:     "invalid JSON",
			data:     `{invalid json}`,
			expected: false,
		},
		{
			name:     "no search result structure",
			data:     `{"title": "Some Page", "content": "Page content"}`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateSearchResults([]byte(tt.data))
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateHugoIndexForSearch(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		expected bool
	}{
		{
			name:     "hugo index with pages",
			data:     `{"pages": [{"title": "Post 1", "content": "Content here"}]}`,
			expected: true,
		},
		{
			name:     "direct array of content",
			data:     `[{"title": "Post 1", "content": "Content here"}]`,
			expected: true,
		},
		{
			name:     "empty pages array",
			data:     `{"pages": []}`,
			expected: false,
		},
		{
			name:     "invalid JSON",
			data:     `{invalid}`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateHugoIndexForSearch([]byte(tt.data))
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractSearchResults(t *testing.T) {
	tests := []struct {
		name           string
		data           string
		expectedCount  int
		expectedFields []string
	}{
		{
			name:           "results array",
			data:           `{"results": [{"title": "Post 1", "url": "/post1", "content": "Content here"}]}`,
			expectedCount:  1,
			expectedFields: []string{"title", "url", "content"},
		},
		{
			name:           "hits array",
			data:           `{"hits": [{"title": "Post 1", "summary": "Summary here", "score": 0.95}]}`,
			expectedCount:  1,
			expectedFields: []string{"title", "summary", "score"},
		},
		{
			name:           "direct array",
			data:           `[{"title": "Post 1", "date": "2023-01-01", "tags": ["golang", "tech"]}]`,
			expectedCount:  1,
			expectedFields: []string{"title", "date", "tags"},
		},
		{
			name:          "empty results",
			data:          `{"results": []}`,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &SearchRequest{Query: "test"}
			results := extractSearchResults([]byte(tt.data), req)
			
			assert.Equal(t, tt.expectedCount, len(results))
			
			if len(results) > 0 {
				result := results[0]
				for _, field := range tt.expectedFields {
					assert.Contains(t, result, field, "Expected field %s not found", field)
				}
			}
		})
	}
}

func TestPerformClientSideSearch(t *testing.T) {
	data := `{
		"pages": [
			{
				"title": "Golang Tutorial",
				"content": "Learn golang programming with this comprehensive tutorial",
				"url": "/posts/golang-tutorial",
				"categories": ["programming", "golang"],
				"tags": ["tutorial", "beginner"]
			},
			{
				"title": "Python Guide",
				"content": "Python programming guide for beginners",
				"url": "/posts/python-guide",
				"categories": ["programming", "python"],
				"tags": ["guide", "beginner"]
			}
		]
	}`

	tests := []struct {
		name          string
		query         string
		contentType   string
		taxonomy      string
		term          string
		expectedCount int
	}{
		{
			name:          "simple query match",
			query:         "golang",
			expectedCount: 1,
		},
		{
			name:          "query in content",
			query:         "programming",
			expectedCount: 2,
		},
		{
			name:          "no matches",
			query:         "nonexistent",
			expectedCount: 0,
		},
		{
			name:          "taxonomy filter match",
			query:         "programming",
			taxonomy:      "categories",
			term:          "golang",
			expectedCount: 1,
		},
		{
			name:          "taxonomy filter no match",
			query:         "programming",
			taxonomy:      "categories",
			term:          "java",
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &SearchRequest{
				Query:       tt.query,
				ContentType: tt.contentType,
				Taxonomy:    tt.taxonomy,
				Term:        tt.term,
			}
			
			results := performClientSideSearch([]byte(data), req)
			assert.Equal(t, tt.expectedCount, len(results))
			
			// Check that results have relevance scores
			for _, result := range results {
				assert.Contains(t, result, "score")
				assert.Greater(t, result["score"], 0.0)
			}
		})
	}
}

func TestFormatSearchResults(t *testing.T) {
	tests := []struct {
		name     string
		results  []map[string]interface{}
		expected string
	}{
		{
			name:     "empty results",
			results:  []map[string]interface{}{},
			expected: "[]",
		},
		{
			name: "single result",
			results: []map[string]interface{}{
				{"title": "Test Post", "score": 1.5},
			},
			expected: "[\n    {\"title\": \"Test Post\", \"score\": 1.50}\n  ]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSearchResults(tt.results)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatMetadata(t *testing.T) {
	metadata := map[string]interface{}{
		"search_method":   "hugo_native",
		"result_count":    5,
		"cached":         true,
		"fallback_used":  false,
	}

	result := formatMetadata(metadata)
	
	// Check that all keys are present in the formatted result
	assert.Contains(t, result, "search_method")
	assert.Contains(t, result, "result_count")
	assert.Contains(t, result, "cached")
	assert.Contains(t, result, "fallback_used")
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