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
// connectionDetails is optional metadata (e.g., host, port, db) to include in logs
func InitAsync(name string, maxRetries int, connectionDetails map[string]any, connectFn func() error) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		logger := logging.NewLogger(name)
		var lastErr error

		// Merge connection details with attempt info
		connectingLog := map[string]any{
			logging.ATTEMPT: 1,
		}
		for k, v := range connectionDetails {
			connectingLog[k] = v
		}

		for i := 0; i < maxRetries; i++ {
			connectingLog[logging.ATTEMPT] = i + 1
			logger.Info(fmt.Sprintf("%s_CONNECTING", name), connectingLog)

			lastErr = connectFn()
			if lastErr == nil {
				connectedLog := map[string]any{
					logging.ATTEMPT: i + 1,
				}
				logger.Info(fmt.Sprintf("%s_CONNECTED", name), connectedLog)
				return
			}

			logger.Warn(fmt.Sprintf("%s_CONNECTION_FAILED", name), lastErr, map[string]any{
				logging.ATTEMPT: i + 1,
			})

			// Exponential backoff
			time.Sleep(time.Duration(i+1) * time.Second)
		}

		logger.Fatal(fmt.Sprintf("%s_CONNECTION_FAILED", name), lastErr, map[string]any{
			logging.RETRIES: maxRetries,
		})
	}()

	return done
}
