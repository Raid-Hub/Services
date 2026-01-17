package network

import (
	"strings"
)

// NetworkError represents categorized network errors
type NetworkError struct {
	Type    NetworkErrorType
	Message string
	Err     error
}

type NetworkErrorType string

const (
	ErrorTypeTimeout     NetworkErrorType = "timeout"
	ErrorTypeCloudflare  NetworkErrorType = "cloudflare"
	ErrorTypeConnection  NetworkErrorType = "connection"
	ErrorTypeServerError NetworkErrorType = "server_error"
	ErrorTypeUnknown     NetworkErrorType = "unknown"
)

// CategorizeNetworkError analyzes an error and returns a NetworkError with appropriate category
func CategorizeNetworkError(err error) *NetworkError {
	if err == nil {
		return nil
	}

	errStr := strings.ToLower(err.Error())

	// Check for Cloudflare errors
	if strings.Contains(errStr, "cloudflare") {
		return &NetworkError{
			Type:    ErrorTypeCloudflare,
			Message: "Cloudflare error",
			Err:     err,
		}
	}

	// Check for timeout errors
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
		return &NetworkError{
			Type:    ErrorTypeTimeout,
			Message: "Request timed out",
			Err:     err,
		}
	}
	// Check for connection errors
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "connection closed") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "network is unreachable") ||
		strings.Contains(errStr, "unexpected eof") {
		return &NetworkError{
			Type:    ErrorTypeConnection,
			Message: "Connection error",
			Err:     err,
		}
	}

	// Check for retryable server errors (502, 504, 520) - but not 503 (system disabled)
	// 503 Service Unavailable typically means system disabled, which is not retryable
	if strings.Contains(errStr, "502") || strings.Contains(errStr, "504") || strings.Contains(errStr, "520") ||
		strings.Contains(errStr, "bad gateway") || strings.Contains(errStr, "gateway timeout") {
		return &NetworkError{
			Type:    ErrorTypeServerError,
			Message: "Server error (5xx)",
			Err:     err,
		}
	}

	// Unknown error
	return &NetworkError{
		Type:    ErrorTypeUnknown,
		Message: "",
		Err:     err,
	}
}

// IsTimeout checks if an error is a timeout error
func IsTimeout(err error) bool {
	netErr := CategorizeNetworkError(err)
	return netErr != nil && netErr.Type == ErrorTypeTimeout
}

// IsConnectionError checks if an error is a connection error
func IsConnectionError(err error) bool {
	netErr := CategorizeNetworkError(err)
	return netErr != nil && netErr.Type == ErrorTypeConnection
}

// IsCloudflareError checks if an error is a Cloudflare error
func IsCloudflareError(err error) bool {
	netErr := CategorizeNetworkError(err)
	return netErr != nil && netErr.Type == ErrorTypeCloudflare
}

// Unwrap implements the unwrap interface for error wrapping
func (e *NetworkError) Unwrap() error {
	return e.Err
}

// Error implements the error interface
func (e *NetworkError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Message
}

// ShouldLogAsError determines if an error should be logged as an ERROR level
// Network errors (timeout/connection) are typically warnings, not errors
func ShouldLogAsError(err error) bool {
	if err == nil {
		return false
	}
	netErr := CategorizeNetworkError(err)
	return netErr.Type == ErrorTypeUnknown
}
