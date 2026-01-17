package network

import (
	"maps"
	"time"

	"raidhub/lib/utils/logging"
	"raidhub/lib/utils/retry"
)

// TransientNetworkErrorRetryConfig retries transient network errors
// such as timeout, connection errors, and server errors (5xx)
func TransientNetworkErrorRetryConfig() retry.RetryConfig {
	return retry.RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 50 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.1, // 10% jitter
		OnRetry:      nil,
		ShouldRetry: func(err error) bool {
			netErr := CategorizeNetworkError(err)
			switch netErr.Type {
			case ErrorTypeTimeout, ErrorTypeConnection, ErrorTypeServerError:
				return true
			default:
				return false
			}
		},
	}
}

// Uses more attempts and longer delays to handle Cloudflare's rate limiting and blocking pages
// Only retries if the error is a Cloudflare error
// Params: logger: logger to use for logging, loggingFields: fields to add to the logging
func CloudflareRetryConfig(logger logging.Logger, loggingFields map[string]any) retry.RetryConfig {
	return retry.RetryConfig{
		MaxAttempts:  8,
		InitialDelay: 3 * time.Second,
		MaxDelay:     120 * time.Second,
		Multiplier:   3,   // Back off fast
		Jitter:       0.2, // 20% jitter for better distribution of retries
		OnRetry: func(attempt int, err error) {
			fields := map[string]any{
				logging.ATTEMPTS: attempt,
			}
			maps.Copy(fields, loggingFields)
			logger.Warn("CLOUDFLARE_NETWORK_ERROR", err, fields)
		},
		ShouldRetry: IsCloudflareError, // Only retry Cloudflare errors
	}
}
