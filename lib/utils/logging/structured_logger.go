package logging

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"raidhub/lib/utils/sentry"
)

// StructuredLogger is a simple logger implementation
type StructuredLogger struct {
	prefix string
}

// NewLogger creates a new logger with the given prefix
func NewLogger(prefix string) *StructuredLogger {
	return &StructuredLogger{prefix: prefix}
}

// formatLogfmtKey removes leading $
func formatLogfmtKey(k string) string {
	if after, ok := strings.CutPrefix(k, "$"); ok {
		k = after
	}
	return k
}

// formatLogfmtValue formats a value according to logfmt spec
// Values need quoting if they contain spaces, quotes, or other special chars
func formatLogfmtValue(v any) string {
	var s string
	switch val := v.(type) {
	case string:
		s = val
	case int:
		s = strconv.Itoa(val)
	case int8:
		s = strconv.FormatInt(int64(val), 10)
	case int16:
		s = strconv.FormatInt(int64(val), 10)
	case int32:
		s = strconv.FormatInt(int64(val), 10)
	case int64:
		s = strconv.FormatInt(val, 10)
	case uint:
		s = strconv.FormatUint(uint64(val), 10)
	case uint8:
		s = strconv.FormatUint(uint64(val), 10)
	case uint16:
		s = strconv.FormatUint(uint64(val), 10)
	case uint32:
		s = strconv.FormatUint(uint64(val), 10)
	case uint64:
		s = strconv.FormatUint(val, 10)
	case float32:
		s = strconv.FormatFloat(float64(val), 'f', -1, 32)
	case float64:
		s = strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		s = strconv.FormatBool(val)
	default:
		s = fmt.Sprintf("%v", v)
	}

	// Check if value needs quoting (contains spaces, quotes, =, or special chars)
	needsQuoting := false
	for _, r := range s {
		if r <= ' ' || r == '=' || r == '"' || r == '\\' {
			needsQuoting = true
			break
		}
	}

	if needsQuoting {
		// Escape quotes and backslashes, then quote
		s = strings.ReplaceAll(s, "\\", "\\\\")
		s = strings.ReplaceAll(s, "\"", "\\\"")
		return `"` + s + `"`
	}
	return s
}

func (l *StructuredLogger) log(level string, key string, fields map[string]any) {
	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	prefix := fmt.Sprintf("%s [%s][%s] -- ", timestamp, level, l.prefix)

	// Build structured log entry
	var output = key
	if len(fields) != 0 {
		output = fmt.Sprintf("%s >>", output)
		var logfmtParts []string
		// Sort keys to ensure consistent field ordering
		keys := make([]string, 0, len(fields))
		for k := range fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			logfmtParts = append(logfmtParts, fmt.Sprintf("%s=%s", formatLogfmtKey(k), formatLogfmtValue(fields[k])))
		}
		if len(logfmtParts) > 0 {
			output += " " + strings.Join(logfmtParts, " ")
		}
	}

	switch level {
	case INFO, DEBUG:
		fmt.Fprintf(stdoutWriter, "%s%s\n", prefix, output)
	case WARN, ERROR, FATAL:
		fmt.Fprintf(stderrWriter, "%s%s\n", prefix, output)
	}
}

func (l *StructuredLogger) Debug(key string, fields map[string]any) {
	if ShouldLog(Debug) {
		l.log(DEBUG, key, fields)
	}
}

func (l *StructuredLogger) Info(key string, fields map[string]any) {
	if ShouldLog(Info) {
		l.log(INFO, key, fields)
	}
}

func (l *StructuredLogger) Warn(key string, err error, fields map[string]any) {
	if ShouldLog(Warn) {
		if fields == nil {
			fields = make(map[string]any)
		}
		if err != nil {
			fields["error"] = err.Error()
		} else {
			fields["error"] = "<nil>"
		}
		l.log(WARN, key, fields)
	}
}

func (l *StructuredLogger) Error(key string, err error, fields map[string]any) {
	if fields == nil {
		fields = make(map[string]any)
	}
	if err != nil {
		fields["error"] = err.Error()
	} else {
		fields["error"] = "<nil>"
	}
	if ShouldLog(Error) {
		l.log(ERROR, key, fields)
	}

	if err != nil {
		sentry.CaptureError(Error, key, err, fields)
	}
}

func (l *StructuredLogger) Fatal(key string, err error, fields map[string]any) {
	if fields == nil {
		fields = make(map[string]any)
	}
	if err != nil {
		fields["error"] = err.Error()
	} else {
		fields["error"] = "<nil>"
	}
	if ShouldLog(Error) {
		l.log(FATAL, key, fields)
	}
	if err != nil {
		sentry.CaptureError(Fatal, key, err, fields)
	}

	sentry.Flush()
	os.Exit(1)
}

// InitSentry initializes Sentry error tracking using the logger's prefix as the app name.
// Sentry will only be initialized if SENTRY_DSN environment variable is set.
//
// Returns two functions that should be deferred in main():
//   - flushFunc: Flushes pending Sentry events before program exit (defer this first)
//   - recoverFunc: Captures panics and sends them to Sentry (defer this second)
//
// Example usage:
//
//	func main() {
//		logger := logging.NewLogger("my-service")
//		flushSentry, recoverSentry := logger.InitSentry()
//		defer flushSentry()    // Runs second - flushes all events
//		defer recoverSentry()  // Runs first - catches panics
//
//		// Your application code here
//	}
func (l *StructuredLogger) InitSentry() (flushFunc func(), recoverFunc func()) {
	fields := map[string]any{
		"app": l.prefix,
	}
	sentryInitialized := sentry.Init(l.prefix, IsVerbose())
	if !sentryInitialized {
		l.Debug("SENTRY_NOT_INITIALIZED", fields)
	} else {
		l.Debug("SENTRY_INITIALIZED", fields)
	}

	flushFunc = func() {
		if sentryInitialized {
			l.Debug("FLUSHING_SENTRY", fields)
			sentry.Flush()
		}
	}

	recoverFunc = sentry.Recover
	return
}
