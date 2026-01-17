package retry

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// MaxRetriesExceededError indicates that the maximum number of retry attempts has been exceeded
type MaxRetriesExceededError struct {
	MaxAttempts int
	LastError   error
}

func (e *MaxRetriesExceededError) Error() string {
	if e.LastError != nil {
		// LastError is already wrapped with context, so just return it
		return e.LastError.Error()
	}
	return fmt.Sprintf("max retry attempts exceeded (%d attempts)", e.MaxAttempts)
}

func (e *MaxRetriesExceededError) Unwrap() error {
	return e.LastError
}

// ContextCancelledError indicates that the context was cancelled during a retry
type ContextCancelledError struct {
	CtxErr    error
	LastError error
}

func (e *ContextCancelledError) Error() string {
	msg := "context cancelled during retry: " + e.CtxErr.Error()
	if e.LastError != nil {
		msg += ", last error: " + e.LastError.Error()
	}
	return msg
}

func (e *ContextCancelledError) Unwrap() error {
	if e.LastError != nil {
		return e.LastError
	}
	return e.CtxErr
}

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
	// Jitter adds random variation to delays to prevent thundering herd (default: 0.1 = 10%)
	// Applied as ±Jitter percentage of the calculated delay
	Jitter float64
	// OnRetry is an optional callback called before each retry attempt
	OnRetry func(attempt int, err error)
	// ShouldRetry determines if an error should be retried
	ShouldRetry func(err error) bool
}

// WithRetry executes a function with automatic retry logic
// The function should return an error, and if that error is retryable according to
// the config's ShouldRetry function, it will automatically retry according to the provided config.
func WithRetry(ctx context.Context, config RetryConfig, fn func(attempt int) error) error {
	return withRetryAttempts(ctx, config, fn, 0)
}

func withRetryAttempts(ctx context.Context, config RetryConfig, fn func(attempt int) error, currentAttempt int) error {
	err := fn(currentAttempt)

	// Success or not retryable
	if err == nil || !config.ShouldRetry(err) {
		return err
	}

	// Check if we've exceeded max attempts
	if currentAttempt >= config.MaxAttempts {
		// Wrap the error with fmt.Errorf to preserve the original stack trace
		wrappedErr := fmt.Errorf("max retry attempts exceeded (%d attempts): %w", config.MaxAttempts, err)
		return &MaxRetriesExceededError{
			MaxAttempts: config.MaxAttempts,
			LastError:   wrappedErr,
		}
	}

	// Check context cancellation before attempting retry
	select {
	case <-ctx.Done():
		// Wrap the error with fmt.Errorf to preserve the original stack trace
		wrappedErr := fmt.Errorf("context cancelled during retry: %w", err)
		return &ContextCancelledError{
			CtxErr:    ctx.Err(),
			LastError: wrappedErr,
		}
	default:
	}

	// Call OnRetry callback if provided
	if config.OnRetry != nil {
		config.OnRetry(currentAttempt+1, err)
	}

	// Calculate delay with exponential backoff and jitter
	delay := calculateDelayWithJitter(config.InitialDelay, config.Multiplier, config.Jitter, currentAttempt)
	if delay > config.MaxDelay {
		delay = config.MaxDelay
	}

	// Wait with context cancellation support
	timer := time.NewTimer(delay)
	select {
	case <-ctx.Done():
		timer.Stop()
		// Wrap the error with fmt.Errorf to preserve the original stack trace
		wrappedErr := fmt.Errorf("context cancelled during retry: %w", err)
		return &ContextCancelledError{
			CtxErr:    ctx.Err(),
			LastError: wrappedErr,
		}
	case <-timer.C:
	}

	// Retry the function
	return withRetryAttempts(ctx, config, fn, currentAttempt+1)
}

// WithRetryForResult executes a function that returns a result and error,
// automatically retrying on errors that match the config's ShouldRetry function
func WithRetryForResult[T any](ctx context.Context, config RetryConfig, fn func(attempt int) (T, error)) (T, error) {
	return withRetryAttemptsForResult(ctx, config, fn, 0)
}

func withRetryAttemptsForResult[T any](ctx context.Context, config RetryConfig, fn func(attempt int) (T, error), currentAttempt int) (T, error) {
	result, err := fn(currentAttempt)

	// Success or not retryable
	if err == nil {
		return result, err
	}

	shouldRetry := config.ShouldRetry != nil && config.ShouldRetry(err)
	if !shouldRetry {
		return result, err
	}

	// Check if we've exceeded max attempts
	var zero T
	if currentAttempt >= config.MaxAttempts {
		// Wrap the error with fmt.Errorf to preserve the original stack trace
		wrappedErr := fmt.Errorf("max retry attempts exceeded (%d attempts): %w", config.MaxAttempts, err)
		return zero, &MaxRetriesExceededError{
			MaxAttempts: config.MaxAttempts,
			LastError:   wrappedErr,
		}
	}

	// Call OnRetry callback if provided
	if config.OnRetry != nil {
		config.OnRetry(currentAttempt+1, err)
	}

	// Calculate delay with exponential backoff and jitter
	delay := calculateDelayWithJitter(config.InitialDelay, config.Multiplier, config.Jitter, currentAttempt)
	if delay > config.MaxDelay {
		delay = config.MaxDelay
	}

	// Wait with context cancellation support
	timer := time.NewTimer(delay)
	select {
	case <-ctx.Done():
		timer.Stop()
		// Wrap the error with fmt.Errorf to preserve the original stack trace
		wrappedErr := fmt.Errorf("context cancelled during retry: %w", err)
		return zero, &ContextCancelledError{
			CtxErr:    ctx.Err(),
			LastError: wrappedErr,
		}
	case <-timer.C:
	}

	// Retry the function
	return withRetryAttemptsForResult(ctx, config, fn, currentAttempt+1)
}

// calculateDelayWithJitter computes exponential backoff delay with jitter
// For attempt=0, returns initial delay. For attempt=n, returns initial * multiplier^n
// Jitter adds random variation: ±jitter percentage of the calculated delay
func calculateDelayWithJitter(initial time.Duration, multiplier float64, jitter float64, attempt int) time.Duration {
	baseDelay := float64(initial) * math.Pow(multiplier, float64(attempt))

	// Apply jitter if specified (jitter > 0)
	if jitter > 0 {
		// Generate random value between -jitter and +jitter
		jitterAmount := (rand.Float64()*2 - 1) * jitter // Range: [-jitter, +jitter]
		baseDelay = baseDelay * (1 + jitterAmount)
	}

	return time.Duration(baseDelay)
}
