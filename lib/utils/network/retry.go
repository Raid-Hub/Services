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

// CloudflareRetryConfig performs one quick retry after ~2s for Cloudflare challenge pages.
// Longer outages are handled by Hermes queue backoff.
func CloudflareRetryConfig(logger logging.Logger, loggingFields map[string]any) retry.RetryConfig {
	return retry.RetryConfig{
		MaxAttempts:  1,
		InitialDelay: 2 * time.Second,
		MaxDelay:     2 * time.Second,
		Multiplier:   1.0,
		Jitter:       0.2,
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
