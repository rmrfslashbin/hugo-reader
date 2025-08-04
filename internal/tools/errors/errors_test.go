package errors

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewError(t *testing.T) {
	context := map[string]interface{}{
		"url":    "https://example.com",
		"method": "GET",
	}
	
	err := NewError(ErrCodeNetworkError, "Connection failed", context)
	
	assert.Equal(t, ErrCodeNetworkError, err.Code)
	assert.Equal(t, "Connection failed", err.Message)
	assert.Equal(t, context, err.Context)
	assert.NotEmpty(t, err.Timestamp)
}

func TestNewErrorResponse(t *testing.T) {
	errors := []ErrorDetail{
		NewError(ErrCodeNotFound, "Content not found", nil),
	}
	
	resp := NewErrorResponse(false, errors, map[string]string{"attempted": "search"})
	
	assert.False(t, resp.Success)
	assert.Len(t, resp.Errors, 1)
	assert.NotNil(t, resp.Data)
}

func TestAddError(t *testing.T) {
	var errors []ErrorDetail
	
	errors = AddError(errors, ErrCodeInvalidURL, "Bad URL", nil)
	errors = AddError(errors, ErrCodeNetworkError, "Connection failed", nil)
	
	assert.Len(t, errors, 2)
	assert.Equal(t, ErrCodeInvalidURL, errors[0].Code)
	assert.Equal(t, ErrCodeNetworkError, errors[1].Code)
}

func TestFormatErrors(t *testing.T) {
	tests := []struct {
		name     string
		errors   []ErrorDetail
		expected string
	}{
		{
			name:     "empty errors",
			errors:   []ErrorDetail{},
			expected: "[]",
		},
		{
			name: "single error",
			errors: []ErrorDetail{
				{Code: ErrCodeNotFound, Message: "Not found", Timestamp: "2024-01-01T00:00:00Z"},
			},
			expected: `[{"code":"NOT_FOUND","message":"Not found","timestamp":"2024-01-01T00:00:00Z"}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatErrors(tt.errors)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToUserFriendlyMessage(t *testing.T) {
	tests := []struct {
		code     string
		expected string
	}{
		{
			code:     ErrCodeInvalidURL,
			expected: "The provided URL is not valid. Please check the Hugo site URL format.",
		},
		{
			code:     ErrCodeNetworkError,
			expected: "Unable to connect to the Hugo site. Please check your internet connection and the site URL.",
		},
		{
			code:     ErrCodeNotFound,
			expected: "The requested content was not found on the Hugo site.",
		},
		{
			code:     "UNKNOWN_CODE",
			expected: "An unexpected error occurred.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			result := ToUserFriendlyMessage(tt.code)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCreateValidationError(t *testing.T) {
	err := CreateValidationError("hugo_site_path", "", "required", "hugo_site_path is required")
	
	assert.Equal(t, ErrCodeValidationFailed, err.Code)
	assert.Equal(t, "hugo_site_path is required", err.Message)
	assert.Contains(t, err.Context, "validation")
	
	validation := err.Context["validation"].(ValidationError)
	assert.Equal(t, "hugo_site_path", validation.Field)
	assert.Equal(t, "", validation.Value)
	assert.Equal(t, "required", validation.Rule)
}

func TestCreateNetworkError(t *testing.T) {
	err := CreateNetworkError("https://example.com", "GET", 404, false, "Not found")
	
	assert.Equal(t, ErrCodeNetworkError, err.Code)
	assert.Equal(t, "Not found", err.Message)
	assert.Contains(t, err.Context, "network")
	
	network := err.Context["network"].(NetworkError)
	assert.Equal(t, "https://example.com", network.URL)
	assert.Equal(t, "GET", network.Method)
	assert.Equal(t, 404, network.StatusCode)
	assert.False(t, network.Timeout)
}

func TestCreateCacheError(t *testing.T) {
	err := CreateCacheError("cache-key-123", "get", "expired", "Cache entry expired")
	
	assert.Equal(t, ErrCodeCacheError, err.Code)
	assert.Equal(t, "Cache entry expired", err.Message)
	assert.Contains(t, err.Context, "cache")
	
	cache := err.Context["cache"].(CacheError)
	assert.Equal(t, "cache-key-123", cache.Key)
	assert.Equal(t, "get", cache.Operation)
	assert.Equal(t, "expired", cache.Reason)
}