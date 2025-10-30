package singleton

import (
	"fmt"
	"raidhub/lib/utils/logging"
	"time"
)

// InitAsync provides a generic singleton initialization pattern with retry logic
// It handles connection retries, logging, and error handling for any singleton that needs
// to connect to an external service (database, message queue, etc.)
// It runs asynchronously and returns a channel that closes when initialization completes
func InitAsync(name string, maxRetries int, connectFn func() error) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		logger := logging.NewLogger(name)
		var lastErr error

		for i := 0; i < maxRetries; i++ {
			logger.Debug(fmt.Sprintf("%s_CONNECTING", name), map[string]any{
				logging.ATTEMPT: i + 1,
			})

			lastErr = connectFn()
			if lastErr == nil {
				logger.Debug(fmt.Sprintf("%s_CONNECTED", name), map[string]any{
					logging.STATUS:  "ready",
					logging.ATTEMPT: i + 1,
				})
				return
			}

			logger.Warn(fmt.Sprintf("%s_CONNECTION_FAILED", name), map[string]any{
				logging.ATTEMPT: i + 1,
				logging.ERROR:   lastErr.Error(),
			})

			// Exponential backoff
			time.Sleep(time.Duration(i+1) * time.Second)
		}

		logger.Fatal(fmt.Sprintf("%s_CONNECTION_FAILED", name), map[string]any{
			logging.RETRIES: maxRetries,
			logging.ERROR:   lastErr.Error(),
		})
	}()

	return done
}
