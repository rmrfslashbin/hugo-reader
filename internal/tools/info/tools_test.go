package info

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tool, err := New("abc123")
	require.NoError(t, err)
	assert.NotNil(t, tool)
	assert.Equal(t, "hugo_reader_info", tool.Name())
	assert.Contains(t, tool.Description(), "version, build, and runtime information")
	assert.Equal(t, "abc123", tool.gitCommit)
	assert.Equal(t, "1.0.0", tool.version)
}

func TestNewWithOptions(t *testing.T) {
	tool, err := New("abc123", 
		WithVersion("2.0.0"),
		WithBuildTime("2023-01-01T12:00:00Z"),
	)
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", tool.version)
	assert.Equal(t, "2023-01-01T12:00:00Z", tool.buildTime)
}

func TestInfoRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     *InfoRequest
		wantErr bool
	}{
		{
			name: "valid request - basic",
			req: &InfoRequest{
				IncludeRuntime: false,
				IncludeTools:   false,
			},
			wantErr: false,
		},
		{
			name: "valid request - with runtime",
			req: &InfoRequest{
				IncludeRuntime: true,
				IncludeTools:   false,
			},
			wantErr: false,
		},
		{
			name: "valid request - with tools",
			req: &InfoRequest{
				IncludeRuntime: false,
				IncludeTools:   true,
			},
			wantErr: false,
		},
		{
			name: "valid request - with both",
			req: &InfoRequest{
				IncludeRuntime: true,
				IncludeTools:   true,
			},
			wantErr: false,
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

func TestFormatInfoSimple(t *testing.T) {
	info := map[string]interface{}{
		"name":        "Test Server",
		"version":     "1.0.0",
		"git_commit":  "abc123",
		"build_time":  "2023-01-01T12:00:00Z",
		"description": "Test description",
		"repository":  "https://github.com/test/repo",
	}

	result := formatInfoSimple(info)
	
	// Check that all basic fields are present
	assert.Contains(t, result, `"name": "Test Server"`)
	assert.Contains(t, result, `"version": "1.0.0"`)
	assert.Contains(t, result, `"git_commit": "abc123"`)
	assert.Contains(t, result, `"build_time": "2023-01-01T12:00:00Z"`)
	assert.Contains(t, result, `"description": "Test description"`)
	assert.Contains(t, result, `"repository": "https://github.com/test/repo"`)
	
	// Check JSON structure
	assert.True(t, strings.HasPrefix(result, "{\n"))
	assert.True(t, strings.HasSuffix(result, "\n  }"))
}

func TestFormatInfoSimpleWithRuntime(t *testing.T) {
	info := map[string]interface{}{
		"name":        "Test Server",
		"version":     "1.0.0",
		"git_commit":  "abc123",
		"build_time":  "2023-01-01T12:00:00Z",
		"description": "Test description",
		"repository":  "https://github.com/test/repo",
		"runtime": map[string]interface{}{
			"go_version":    "go1.21.0",
			"go_os":         "darwin",
			"go_arch":       "amd64",
			"num_cpu":       8,
			"num_goroutine": 5,
		},
	}

	result := formatInfoSimple(info)
	
	// Check runtime fields
	assert.Contains(t, result, `"runtime": {`)
	assert.Contains(t, result, `"go_version": "go1.21.0"`)
	assert.Contains(t, result, `"go_os": "darwin"`)
	assert.Contains(t, result, `"go_arch": "amd64"`)
	assert.Contains(t, result, `"num_cpu": 8`)
	assert.Contains(t, result, `"num_goroutine": 5`)
}

func TestFormatInfoSimpleWithTools(t *testing.T) {
	info := map[string]interface{}{
		"name":        "Test Server",
		"version":     "1.0.0",
		"git_commit":  "abc123",
		"build_time":  "2023-01-01T12:00:00Z",
		"description": "Test description",
		"repository":  "https://github.com/test/repo",
		"tools": []map[string]interface{}{
			{
				"name":        "test_tool",
				"description": "Test tool description",
				"purpose":     "Testing",
			},
		},
	}

	result := formatInfoSimple(info)
	
	// Check tools section
	assert.Contains(t, result, `"tools": [`)
	assert.Contains(t, result, `"name": "test_tool"`)
	assert.Contains(t, result, `"description": "Test tool description"`)
	assert.Contains(t, result, `"purpose": "Testing"`)
}

func TestTool_SetLogger(t *testing.T) {
	tool, err := New("abc123")
	require.NoError(t, err)

	// Test with nil logger
	tool.SetLogger(nil)
	assert.NotNil(t, tool.log)

	// Test that it doesn't panic with valid logger
	// We can't easily test the logger content without more setup
}