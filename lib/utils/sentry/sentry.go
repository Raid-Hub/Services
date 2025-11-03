package sentry

import (
	"fmt"
	"time"

	"raidhub/lib/env"

	"github.com/getsentry/sentry-go"
)

// Init initializes Sentry with configuration from environment variables
func Init(appName string, debug bool) bool {
	dsn := env.SentryDSN
	if dsn == "" {
		return false
	}

	if err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      env.Environment,
		Release:          env.Release,
		AttachStacktrace: true,
		Debug:            debug,
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			if event.Tags == nil {
				event.Tags = make(map[string]string)
			}
			event.Tags["app"] = appName
			return event
		},
	}); err != nil {
		panic(err)
	}

	return true
}

// Recover recovers from panics and sends them to Sentry
func Recover() {
	if err := recover(); err != nil {
		if hub := sentry.CurrentHub(); hub != nil {
			hub.WithScope(func(scope *sentry.Scope) {
				scope.SetLevel(sentry.LevelFatal)
				scope.SetTag("panic", "true")
				if e, ok := err.(error); ok {
					hub.CaptureException(e)
				} else {
					hub.CaptureMessage(fmt.Sprintf("panic: %v", err))
				}
			})
		}
		panic(err)
	}
}

// Flush ensures all pending events are sent before program exits
func Flush() {
	sentry.Flush(2 * time.Second)
}

func isInitialized() bool {
	return sentry.CurrentHub() != nil
}

// CaptureError captures an error to Sentry
func CaptureError(level sentry.Level, logKey string, err error, fields map[string]any) {
	if !isInitialized() {
		return
	}

	sentry.CurrentHub().WithScope(func(scope *sentry.Scope) {
		scope.SetLevel(sentry.LevelError)
		scope.SetTag("log_key", logKey)
		scope.SetTag("panic", "false")

		if user, ok := extractUser(fields); ok {
			scope.SetUser(*user)
		}

		for k, v := range fields {
			scope.SetExtra(k, v)
		}

		sentry.CurrentHub().CaptureException(err)
	})
}

// extractUser extracts the user from the fields map
func extractUser(fields map[string]any) (*sentry.User, bool) {
	if val, ok := fields["membership_id"]; ok {
		id, ok := convertToInt64(val)
		if !ok {
			return nil, false
		}
		return &sentry.User{ID: id}, true
	}

	return nil, false
}

// convertToInt64 converts various numeric types to int64
func convertToInt64(v any) (string, bool) {
	switch val := v.(type) {
	case int, int64:
		return fmt.Sprintf("%d", val), true
	case string:
		return val, true
	default:
		return "", false
	}
}
