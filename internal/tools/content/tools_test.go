package content

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tool, err := New()
	require.NoError(t, err)
	assert.NotNil(t, tool)
	assert.Equal(t, "hugo_reader_get_content", tool.Name())
	assert.Equal(t, "Get content from Hugo sites by path. Supports bulk retrieval and flexible response options (metadata, body, or both). Tries multiple endpoint patterns automatically. Example paths: '/posts/my-post/', '/recipes/cookies/', '/about/'. Use with or without trailing slashes.", tool.Description())
	assert.NotNil(t, tool.httpClient)
}

func TestContentRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     *ContentRequest
		wantErr bool
	}{
		{
			name: "valid request with defaults",
			req: &ContentRequest{
				HugoSitePath: "https://example.com",
				Paths:        []string{"posts/article1", "about"},
			},
			wantErr: false,
		},
		{
			name: "valid request with all fields",
			req: &ContentRequest{
				HugoSitePath: "https://example.com",
				Paths:        []string{"posts/article1"},
				Include:      []string{"metadata"},
				Limit:        10,
			},
			wantErr: false,
		},
		{
			name: "missing hugo_site_path",
			req: &ContentRequest{
				HugoSitePath: "",
				Paths:        []string{"posts/article1"},
			},
			wantErr: true,
		},
		{
			name: "empty paths",
			req: &ContentRequest{
				HugoSitePath: "https://example.com",
				Paths:        []string{},
			},
			wantErr: true,
		},
		{
			name: "invalid include value",
			req: &ContentRequest{
				HugoSitePath: "https://example.com",
				Paths:        []string{"posts/article1"},
				Include:      []string{"invalid"},
			},
			wantErr: true,
		},
		{
			name: "limit too high",
			req: &ContentRequest{
				HugoSitePath: "https://example.com",
				Paths:        []string{"posts/article1"},
				Limit:        150,
			},
			wantErr: true,
		},
		{
			name: "limit too low",
			req: &ContentRequest{
				HugoSitePath: "https://example.com",
				Paths:        []string{"posts/article1"},
				Limit:        0,
			},
			wantErr: false, // 0 gets set to default (50)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Check defaults are set
				if len(tt.req.Include) == 0 {
					assert.Equal(t, []string{"both"}, tt.req.Include)
				}
				if tt.req.Limit == 0 {
					assert.Equal(t, 50, tt.req.Limit)
				}
			}
		})
	}
}

func TestValidateContentStructure(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		expected bool
	}{
		{
			name:     "valid content with title and content",
			data:     `{"title": "My Post", "content": "Post content here", "date": "2023-01-01"}`,
			expected: true,
		},
		{
			name:     "valid content with multiple fields",
			data:     `{"title": "My Post", "body": "Post body", "summary": "Post summary", "url": "/posts/my-post"}`,
			expected: true,
		},
		{
			name:     "invalid content with only one field",
			data:     `{"title": "My Post"}`,
			expected: false,
		},
		{
			name:     "invalid JSON",
			data:     `{invalid json}`,
			expected: false,
		},
		{
			name:     "no content fields",
			data:     `{"random": "data", "other": "field"}`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateContentStructure([]byte(tt.data))
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateHugoIndexForContent(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		expected bool
	}{
		{
			name:     "hugo index with pages containing content",
			data:     `{"pages": [{"title": "Post 1", "content": "Content here"}, {"title": "Post 2", "summary": "Summary"}]}`,
			expected: true,
		},
		{
			name:     "hugo index without content in pages",
			data:     `{"pages": [{"title": "Post 1", "random": "data"}]}`,
			expected: false,
		},
		{
			name:     "fallback to direct content validation",
			data:     `{"title": "Page", "content": "Content", "date": "2023-01-01"}`,
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
			result := validateHugoIndexForContent([]byte(tt.data))
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractContent(t *testing.T) {
	tests := []struct {
		name           string
		data           string
		requestedPath  string
		include        []string
		expectedFields []string
	}{
		{
			name:           "metadata only",
			data:           `{"title": "My Post", "date": "2023-01-01", "content": "Post content"}`,
			requestedPath:  "posts/my-post",
			include:        []string{"metadata"},
			expectedFields: []string{"path", "source_endpoint", "metadata"},
		},
		{
			name:           "body only",
			data:           `{"title": "My Post", "content": "Post content", "body": "Post body"}`,
			requestedPath:  "posts/my-post",
			include:        []string{"body"},
			expectedFields: []string{"path", "source_endpoint", "body"},
		},
		{
			name:           "both metadata and body",
			data:           `{"title": "My Post", "date": "2023-01-01", "content": "Post content"}`,
			requestedPath:  "posts/my-post",
			include:        []string{"both"},
			expectedFields: []string{"path", "source_endpoint", "metadata", "body"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractContent([]byte(tt.data), tt.requestedPath, tt.include, "http://example.com/test.json")
			
			// Check that all expected fields are present
			for _, field := range tt.expectedFields {
				assert.Contains(t, result, field, "Expected field %s not found", field)
			}
			
			// Check that path is set correctly
			assert.Equal(t, tt.requestedPath, result["path"])
			assert.Equal(t, "http://example.com/test.json", result["source_endpoint"])
		})
	}
}

func TestContains(t *testing.T) {
	slice := []string{"metadata", "body", "both"}
	
	assert.True(t, contains(slice, "metadata"))
	assert.True(t, contains(slice, "body"))
	assert.True(t, contains(slice, "both"))
	assert.False(t, contains(slice, "nonexistent"))
}

func TestFormatStringArray(t *testing.T) {
	tests := []struct {
		name     string
		arr      []string
		expected string
	}{
		{
			name:     "empty array",
			arr:      []string{},
			expected: "[]",
		},
		{
			name:     "single item",
			arr:      []string{"metadata"},
			expected: `["metadata"]`,
		},
		{
			name:     "multiple items",
			arr:      []string{"metadata", "body"},
			expected: `["metadata", "body"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatStringArray(tt.arr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatErrors(t *testing.T) {
	tests := []struct {
		name     string
		errors   []string
		expected string
	}{
		{
			name:     "no errors",
			errors:   []string{},
			expected: "[]",
		},
		{
			name:     "single error",
			errors:   []string{"Path not found"},
			expected: "[\n    \"Path not found\"\n  ]",
		},
		{
			name:     "multiple errors",
			errors:   []string{"Path not found", "Invalid format"},
			expected: "[\n    \"Path not found\",\n    \"Invalid format\"\n  ]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatErrors(tt.errors)
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