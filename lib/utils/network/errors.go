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
	ErrorTypeTimeout       NetworkErrorType = "timeout"
	ErrorTypeConnection    NetworkErrorType = "connection"
	ErrorTypeUnknown       NetworkErrorType = "unknown"
)

// CategorizeNetworkError analyzes an error and returns a NetworkError with appropriate category
func CategorizeNetworkError(err error) *NetworkError {
	if err == nil {
		return nil
	}

	errStr := strings.ToLower(err.Error())
	
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
		strings.Contains(errStr, "network is unreachable") {
		return &NetworkError{
			Type:    ErrorTypeConnection,
			Message: "Connection error",
			Err:     err,
		}
	}

	// Unknown error
	return &NetworkError{
		Type:    ErrorTypeUnknown,
		Message: "Network error",
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

// ShouldRetry determines if an error is retryable
// Timeout and connection errors are typically retryable
func ShouldRetry(err error) bool {
	netErr := CategorizeNetworkError(err)
	return netErr != nil && (netErr.Type == ErrorTypeTimeout || netErr.Type == ErrorTypeConnection)
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

