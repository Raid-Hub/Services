package sentry

import (
	"fmt"
	"strconv"
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

// errorWithLogKey wraps an error to include the log key in the error message
// This ensures the log key appears in Sentry error titles
type errorWithLogKey struct {
	logKey string
	err    error
}

func (e *errorWithLogKey) Error() string {
	if e.err != nil {
		return fmt.Sprintf("%s: %s", e.logKey, e.err.Error())
	}
	return fmt.Sprintf("%s: <nil>", e.logKey)
}

func (e *errorWithLogKey) Unwrap() error {
	return e.err
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
			scope.SetExtra(k, normalizeValueForSentry(v))
		}

		// Wrap error with log key to include it in Sentry error title
		wrappedErr := &errorWithLogKey{
			logKey: logKey,
			err:    err,
		}

		sentry.CurrentHub().CaptureException(wrappedErr)
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

// normalizeValueForSentry converts numeric types to strings to avoid JSON serialization issues
// Sentry's SetExtra can have issues with large int64 values when serialized as JSON numbers
func normalizeValueForSentry(v any) any {
	switch val := v.(type) {
	case int64:
		return strconv.FormatInt(val, 10)
	case int:
		return strconv.FormatInt(int64(val), 10)
	case int32:
		return strconv.FormatInt(int64(val), 10)
	case int16:
		return strconv.FormatInt(int64(val), 10)
	case int8:
		return strconv.FormatInt(int64(val), 10)
	case uint64:
		return strconv.FormatUint(val, 10)
	case uint:
		return strconv.FormatUint(uint64(val), 10)
	case uint32:
		return strconv.FormatUint(uint64(val), 10)
	case uint16:
		return strconv.FormatUint(uint64(val), 10)
	case uint8:
		return strconv.FormatUint(uint64(val), 10)
	default:
		return v
	}
}
