package network

import (
	"context"
	"fmt"
	"time"
)

// RetryConfig configures retry behavior
type RetryConfig struct {
	// MaxAttempts is the maximum number of retry attempts (default: 3)
	MaxAttempts int
	// InitialDelay is the initial delay before the first retry (default: 100ms)
	InitialDelay time.Duration
	// MaxDelay caps the retry delay (default: 5s)
	MaxDelay time.Duration
	// Multiplier increases delay after each retry (default: 2.0)
	Multiplier float64
	// OnRetry is an optional callback called before each retry attempt
	OnRetry func(attempt int, err error)
}

// DefaultRetryConfig returns a sensible default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
		OnRetry:      nil,
	}
}

// WithRetry executes a function with automatic retry logic for network errors
// The function should return an error, and if that error is retryable according to
// ShouldRetry(), it will automatically retry according to the provided config.
func WithRetry(ctx context.Context, config RetryConfig, fn func() error) error {
	return withRetryAttempts(ctx, config, fn, 0)
}

func withRetryAttempts(ctx context.Context, config RetryConfig, fn func() error, currentAttempt int) error {
	err := fn()
	
	// Success or not retryable
	if err == nil || !ShouldRetry(err) {
		return err
	}

	// Check if we've exceeded max attempts
	if currentAttempt >= config.MaxAttempts {
		return fmt.Errorf("max retry attempts (%d) exceeded: %w", config.MaxAttempts, err)
	}

	// Call OnRetry callback if provided
	if config.OnRetry != nil {
		config.OnRetry(currentAttempt+1, err)
	}

	// Calculate delay with exponential backoff
	delay := calculateDelay(config.InitialDelay, config.Multiplier, currentAttempt)
	if delay > config.MaxDelay {
		delay = config.MaxDelay
	}

	// Wait with context cancellation support
	timer := time.NewTimer(delay)
	select {
	case <-ctx.Done():
		timer.Stop()
		return fmt.Errorf("context cancelled during retry: %w", ctx.Err())
	case <-timer.C:
	}

	// Retry the function
	return withRetryAttempts(ctx, config, fn, currentAttempt+1)
}

// WithRetryForResult executes a function that returns a result and error,
// automatically retrying on network errors
func WithRetryForResult[T any](ctx context.Context, config RetryConfig, fn func() (T, error)) (T, error) {
	return withRetryAttemptsForResult(ctx, config, fn, 0)
}

func withRetryAttemptsForResult[T any](ctx context.Context, config RetryConfig, fn func() (T, error), currentAttempt int) (T, error) {
	result, err := fn()
	
	// Success or not retryable
	if err == nil || !ShouldRetry(err) {
		return result, err
	}

	// Check if we've exceeded max attempts
	var zero T
	if currentAttempt >= config.MaxAttempts {
		return zero, fmt.Errorf("max retry attempts (%d) exceeded: %w", config.MaxAttempts, err)
	}

	// Call OnRetry callback if provided
	if config.OnRetry != nil {
		config.OnRetry(currentAttempt+1, err)
	}

	// Calculate delay with exponential backoff
	delay := calculateDelay(config.InitialDelay, config.Multiplier, currentAttempt)
	if delay > config.MaxDelay {
		delay = config.MaxDelay
	}

	// Wait with context cancellation support
	timer := time.NewTimer(delay)
	select {
	case <-ctx.Done():
		timer.Stop()
		return zero, fmt.Errorf("context cancelled during retry: %w", ctx.Err())
	case <-timer.C:
	}

	// Retry the function
	return withRetryAttemptsForResult(ctx, config, fn, currentAttempt+1)
}

// calculateDelay computes exponential backoff delay
func calculateDelay(initial time.Duration, multiplier float64, attempt int) time.Duration {
	delay := float64(initial) * multiplier
	for i := 0; i < attempt; i++ {
		delay *= multiplier
	}
	return time.Duration(delay)
}

// QuickRetry is a convenience function for quick retries with default configuration
// It returns just the error and doesn't support context cancellation
func QuickRetry(fn func() error) error {
	ctx := context.Background()
	config := DefaultRetryConfig()
	return WithRetry(ctx, config, fn)
}

// QuickRetryForResult is a convenience function for quick retries with default configuration
// It returns the result and error and doesn't support context cancellation
func QuickRetryForResult[T any](fn func() (T, error)) (T, error) {
	ctx := context.Background()
	config := DefaultRetryConfig()
	return WithRetryForResult(ctx, config, fn)
}

