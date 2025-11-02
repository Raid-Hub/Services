package sentry

import (
	"fmt"
	"time"

	"raidhub/lib/env"

	"github.com/getsentry/sentry-go"
)

// Init initializes Sentry with configuration from environment variables
func Init(appName string) bool {
	dsn := env.SentryDSN
	if dsn == "" {
		return false
	}

	if err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      env.Environment,
		Release:          env.Release,
		AttachStacktrace: true,
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

func isInitialized() bool {
	return sentry.CurrentHub() != nil
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
		panic(err) // Re-panic after capturing
	}
}

// CaptureError captures an error to Sentry
func CaptureError(level sentry.Level, logKey string, err error, fields map[string]any) {
	if !isInitialized() {
		return
	}

	sentry.CurrentHub().WithScope(func(scope *sentry.Scope) {
		scope.SetLevel(sentry.LevelError)
		scope.SetTag("log_key", logKey)
		for k, v := range fields {
			scope.SetExtra(k, v)
		}

		sentry.CurrentHub().CaptureException(err)
	})
}

// Flush ensures all pending events are sent before program exits
func Flush() {
	sentry.Flush(2 * time.Second)
}
