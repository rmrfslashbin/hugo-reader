package cache

import (
	"testing"

	"github.com/rmrfslashbin/mcp/hugo-reader/internal/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	cacheInstance := cache.New()
	tool, err := New(cacheInstance)
	require.NoError(t, err)
	assert.NotNil(t, tool)
	assert.Equal(t, "hugo_reader_cache_manager", tool.Name())
	assert.Equal(t, "Manage Hugo reader cache with smart HTTP validation. Actions: 'clear' (remove all/specific entries), 'stats' (cache statistics), 'clean' (remove expired entries). Use 'clear' if getting stale data.", tool.Description())
}

func TestClearCacheRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     *ClearCacheRequest
		wantErr bool
	}{
		{
			name:    "valid clear action",
			req:     &ClearCacheRequest{Action: "clear"},
			wantErr: false,
		},
		{
			name:    "valid stats action",
			req:     &ClearCacheRequest{Action: "stats"},
			wantErr: false,
		},
		{
			name:    "valid clean action",
			req:     &ClearCacheRequest{Action: "clean"},
			wantErr: false,
		},
		{
			name:    "invalid action",
			req:     &ClearCacheRequest{Action: "invalid"},
			wantErr: true,
		},
		{
			name:    "empty action",
			req:     &ClearCacheRequest{Action: ""},
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

func TestTool_Execute_Stats(t *testing.T) {
	cacheInstance := cache.New()
	tool, err := New(cacheInstance)
	require.NoError(t, err)

	// Add some test data to cache
	cacheInstance.Set("test-key", []byte("test data"), "", "")

	req := &ClearCacheRequest{Action: "stats"}
	resp, err := tool.Execute(req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	
	// Response should be a ToolResponse with text content
	assert.NotNil(t, resp.Content)
	assert.Len(t, resp.Content, 1)
}

func TestTool_Execute_Clear(t *testing.T) {
	cacheInstance := cache.New()
	tool, err := New(cacheInstance)
	require.NoError(t, err)

	// Add some test data to cache
	cacheInstance.Set("test-key", []byte("test data"), "", "")

	// Verify data exists
	_, found := cacheInstance.Get("test-key")
	assert.True(t, found)

	req := &ClearCacheRequest{Action: "clear"}
	resp, err := tool.Execute(req)
	require.NoError(t, err)
	assert.NotNil(t, resp)

	// Verify cache is cleared
	_, found = cacheInstance.Get("test-key")
	assert.False(t, found)
	
	// Response should indicate success
	assert.NotNil(t, resp.Content)
	assert.Len(t, resp.Content, 1)
}

func TestTool_Execute_Clean(t *testing.T) {
	cacheInstance := cache.New()
	tool, err := New(cacheInstance)
	require.NoError(t, err)

	req := &ClearCacheRequest{Action: "clean"}
	resp, err := tool.Execute(req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	
	// Response should contain clean results
	assert.NotNil(t, resp.Content)
	assert.Len(t, resp.Content, 1)
}

type invalidRequest struct {
	Invalid string
}

func (r *invalidRequest) Validate() error {
	return nil
}

func TestTool_Execute_InvalidRequest(t *testing.T) {
	cacheInstance := cache.New()
	tool, err := New(cacheInstance)
	require.NoError(t, err)

	// Test with invalid request type
	req := &invalidRequest{Invalid: "test"}
	_, err = tool.Execute(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid request type")
}

func TestTool_SetLogger(t *testing.T) {
	cacheInstance := cache.New()
	tool, err := New(cacheInstance)
	require.NoError(t, err)

	// Test with nil logger
	tool.SetLogger(nil)
	assert.NotNil(t, tool.log)

	// Test that it doesn't panic with valid logger
	// We can't easily test the logger content without more setup
}