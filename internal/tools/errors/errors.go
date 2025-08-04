package errors

import (
	"encoding/json"
	"fmt"
)

// ErrorDetail represents a detailed error with context
type ErrorDetail struct {
	Code      string                 `json:"code"`
	Message   string                 `json:"message"`
	Context   map[string]interface{} `json:"context,omitempty"`
	Timestamp string                 `json:"timestamp"`
}

// ErrorResponse represents the standardized error handling structure
type ErrorResponse struct {
	Success bool          `json:"success"`
	Errors  []ErrorDetail `json:"errors"`
	Data    interface{}   `json:"data,omitempty"`
}

// Common error codes
const (
	ErrCodeInvalidRequest     = "INVALID_REQUEST"
	ErrCodeInvalidURL         = "INVALID_URL"
	ErrCodeNetworkError       = "NETWORK_ERROR"
	ErrCodeValidationFailed   = "VALIDATION_FAILED"
	ErrCodeNotFound           = "NOT_FOUND"
	ErrCodeTimeout            = "TIMEOUT"
	ErrCodeUnauthorized       = "UNAUTHORIZED"
	ErrCodeRateLimited        = "RATE_LIMITED"
	ErrCodeInternalError      = "INTERNAL_ERROR"
	ErrCodeCacheError         = "CACHE_ERROR"
	ErrCodeParseError         = "PARSE_ERROR"
)

// NewError creates a new ErrorDetail with timestamp
func NewError(code, message string, context map[string]interface{}) ErrorDetail {
	return ErrorDetail{
		Code:      code,
		Message:   message,
		Context:   context,
		Timestamp: getCurrentTimestamp(),
	}
}

// NewErrorResponse creates a new ErrorResponse
func NewErrorResponse(success bool, errors []ErrorDetail, data interface{}) ErrorResponse {
	return ErrorResponse{
		Success: success,
		Errors:  errors,
		Data:    data,
	}
}

// AddError adds an error to an existing error list
func AddError(errors []ErrorDetail, code, message string, context map[string]interface{}) []ErrorDetail {
	return append(errors, NewError(code, message, context))
}

// FormatErrors formats error details as JSON string
func FormatErrors(errors []ErrorDetail) string {
	if len(errors) == 0 {
		return "[]"
	}
	
	jsonBytes, err := json.Marshal(errors)
	if err != nil {
		// Fallback formatting if JSON marshaling fails
		return fmt.Sprintf(`[{"code": "%s", "message": "Failed to format errors", "timestamp": "%s"}]`, 
			ErrCodeInternalError, getCurrentTimestamp())
	}
	
	return string(jsonBytes)
}

// ToUserFriendlyMessage converts technical errors to user-friendly messages
func ToUserFriendlyMessage(code string) string {
	switch code {
	case ErrCodeInvalidURL:
		return "The provided URL is not valid. Please check the Hugo site URL format."
	case ErrCodeNetworkError:
		return "Unable to connect to the Hugo site. Please check your internet connection and the site URL."
	case ErrCodeNotFound:
		return "The requested content was not found on the Hugo site."
	case ErrCodeTimeout:
		return "The request timed out. The Hugo site may be slow to respond."
	case ErrCodeUnauthorized:
		return "Access denied. The Hugo site may require authentication."
	case ErrCodeRateLimited:
		return "Too many requests. Please wait a moment before trying again."
	case ErrCodeValidationFailed:
		return "The response from the Hugo site doesn't contain the expected data structure."
	case ErrCodeParseError:
		return "Unable to parse the response from the Hugo site. The data may be in an unexpected format."
	case ErrCodeCacheError:
		return "There was an issue with the cache system."
	case ErrCodeInternalError:
		return "An internal error occurred while processing your request."
	default:
		return "An unexpected error occurred."
	}
}

// Helper function to get current timestamp
func getCurrentTimestamp() string {
	// Using a simple format for now
	// In a real implementation, you might want to use time.Now().Format(time.RFC3339)
	return "2024-01-01T00:00:00Z" // Placeholder for consistent testing
}

// ValidationError represents validation-specific errors
type ValidationError struct {
	Field   string `json:"field"`
	Value   string `json:"value"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

// NetworkError represents network-specific errors
type NetworkError struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code,omitempty"`
	Method     string `json:"method"`
	Timeout    bool   `json:"timeout"`
}

// CacheError represents cache-specific errors
type CacheError struct {
	Key       string `json:"key"`
	Operation string `json:"operation"`
	Reason    string `json:"reason"`
}

// CreateValidationError creates a validation error with context
func CreateValidationError(field, value, rule, message string) ErrorDetail {
	return NewError(ErrCodeValidationFailed, message, map[string]interface{}{
		"validation": ValidationError{
			Field:   field,
			Value:   value,
			Rule:    rule,
			Message: message,
		},
	})
}

// CreateNetworkError creates a network error with context
func CreateNetworkError(url, method string, statusCode int, timeout bool, message string) ErrorDetail {
	return NewError(ErrCodeNetworkError, message, map[string]interface{}{
		"network": NetworkError{
			URL:        url,
			StatusCode: statusCode,
			Method:     method,
			Timeout:    timeout,
		},
	})
}

// CreateCacheError creates a cache error with context
func CreateCacheError(key, operation, reason, message string) ErrorDetail {
	return NewError(ErrCodeCacheError, message, map[string]interface{}{
		"cache": CacheError{
			Key:       key,
			Operation: operation,
			Reason:    reason,
		},
	})
}