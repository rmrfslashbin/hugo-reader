package discovery

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tool, err := New()
	require.NoError(t, err)
	assert.NotNil(t, tool)
	assert.Equal(t, "hugo_reader_discover_site", tool.Name())
	assert.Contains(t, tool.Description(), "Discover available content and structure")
	assert.NotNil(t, tool.httpClient)
}

func TestDiscoveryRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     *DiscoveryRequest
		wantErr bool
	}{
		{
			name: "valid request with overview",
			req: &DiscoveryRequest{
				HugoSitePath: "https://example.com",
				DiscoveryType: "overview",
			},
			wantErr: false,
		},
		{
			name: "valid request with default type",
			req: &DiscoveryRequest{
				HugoSitePath: "https://example.com",
			},
			wantErr: false,
		},
		{
			name: "valid request with sections",
			req: &DiscoveryRequest{
				HugoSitePath: "https://example.com",
				DiscoveryType: "sections",
				Limit: 10,
			},
			wantErr: false,
		},
		{
			name: "missing hugo_site_path",
			req: &DiscoveryRequest{
				DiscoveryType: "overview",
			},
			wantErr: true,
		},
		{
			name: "invalid discovery type",
			req: &DiscoveryRequest{
				HugoSitePath: "https://example.com",
				DiscoveryType: "invalid",
			},
			wantErr: true,
		},
		{
			name: "invalid limit too high",
			req: &DiscoveryRequest{
				HugoSitePath: "https://example.com",
				DiscoveryType: "pages",
				Limit: 300,
			},
			wantErr: true,
		},
		{
			name: "invalid limit too low",
			req: &DiscoveryRequest{
				HugoSitePath: "https://example.com",
				DiscoveryType: "pages",
				Limit: 0,
			},
			wantErr: false, // Should set default
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
				if tt.req.DiscoveryType == "" {
					assert.Equal(t, "overview", tt.req.DiscoveryType)
				}
				if tt.req.Limit == 0 {
					assert.Equal(t, 50, tt.req.Limit)
				}
			}
		})
	}
}

func TestFormatResults(t *testing.T) {
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
				{"path": "/test/", "title": "Test"},
			},
			expected: `[
    {"path": "/test/", "title": "Test"}
  ]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatResults(tt.results)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]interface{}
		expected string
	}{
		{
			name: "basic metadata",
			metadata: map[string]interface{}{
				"method": "overview",
				"count": 5,
				"limited": true,
			},
			expected: `{
    "method": "overview",
    "count": 5,
    "limited": true
  }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatMetadata(tt.metadata)
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