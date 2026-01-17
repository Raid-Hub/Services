package sentry

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"raidhub/lib/env"
	"raidhub/lib/utils/retry"

	"github.com/getsentry/sentry-go"
)

const (
	// QueueTagKey is the key used for the queue Sentry tag
	// The field in logs uses "$queue" prefix to distinguish it as a tag
	QueueTagKey = "$queue"
)

// filterFrames removes frames containing internal paths from a stack trace
func filterFrames(frames []sentry.Frame) []sentry.Frame {
	filtered := make([]sentry.Frame, 0, len(frames))

	for _, frame := range frames {
		// Remove frames containing these path substrings
		if strings.Contains(frame.Filename, "lib/utils/sentry") ||
			strings.Contains(frame.Filename, "lib/utils/logging") {
			continue // skip this frame
		}

		filtered = append(filtered, frame)
	}

	return filtered
}

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

			// Filter stack traces in exceptions
			for i := range event.Exception {
				exception := &event.Exception[i]
				if exception.Stacktrace != nil {
					exception.Stacktrace.Frames = filterFrames(exception.Stacktrace.Frames)
				}
			}

			return event
		},
	}); err != nil {
		panic(err)
	}

	return true
}

// Recover recovers from panics and sends them to Sentry, then re-panics.
// Fields can contain context information that will be set as Sentry tags (for fields starting with "$")
// or extras (for other fields). Fields can also be a function that returns a map, which will be
// called when the panic occurs (useful for dynamic context that changes at runtime).
//
// Usage examples:
//   - defer sentry.Recover() - re-panic after capturing
//   - defer sentry.Recover(fields) - re-panic with context
//   - defer sentry.Recover(func() map[string]any { return getFields() }) - re-panic with dynamic context
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
			sentry.Flush(1 * time.Second)
		}
		panic(err)
	}
}

// Go starts a goroutine with panic recovery. The function will be wrapped with
// Recover() to ensure panics are captured to Sentry and then re-panicked.
//
// Usage:
//   - sentry.Go(func() { doWork() })
func Go(fn func()) {
	go func() {
		defer Recover()
		fn()
	}()
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

	// Skip sending ContextCancelledError to Sentry as it's an expected condition
	var contextCancelledErr *retry.ContextCancelledError
	if err != nil && errors.As(err, &contextCancelledErr) {
		return
	}

	sentry.CurrentHub().WithScope(func(scope *sentry.Scope) {
		scope.SetLevel(sentry.LevelError)

		// Extract any fields starting with "$" and set them as Sentry tags, then remove from fields
		// Tag names are the field key without the "$" prefix
		tagFields := make([]string, 0)
		for k, v := range fields {
			if after, ok := strings.CutPrefix(k, "$"); ok {
				if tagStr := convertToString(v); tagStr != "" {
					scope.SetTag(after, tagStr)
				}
				tagFields = append(tagFields, k)
			}
		}
		// Remove tag fields from extras
		for _, k := range tagFields {
			delete(fields, k)
		}

		scope.SetTag("log_key", logKey)
		scope.SetTag("panic", "false")

		if user, ok := extractUser(fields); ok {
			scope.SetUser(*user)
		}

		for k, v := range fields {
			scope.SetExtra(k, normalizeValueForSentry(v))
		}

		sentry.CurrentHub().CaptureException(err)
	})
}

// convertToString converts any value to string for Sentry tag
func convertToString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case int, int64, int32, int16, int8:
		return fmt.Sprintf("%d", val)
	case uint, uint64, uint32, uint16, uint8:
		return fmt.Sprintf("%d", val)
	default:
		return fmt.Sprintf("%v", val)
	}
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
