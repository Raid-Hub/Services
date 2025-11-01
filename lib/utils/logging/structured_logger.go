package logging

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// StructuredLogger is a simple logger implementation
type StructuredLogger struct {
	prefix string
}

// NewLogger creates a new logger with the given prefix
func NewLogger(prefix string) Logger {
	return &StructuredLogger{prefix: prefix}
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
	timestamp := time.Now().UTC().Format("2006-01-02 15:04:05.000")
	prefix := fmt.Sprintf("%s [%s][%s] -- ", timestamp, level, l.prefix)

	// Build structured log entry
	var output = key
	if len(fields) != 0 {
		output = fmt.Sprintf("%s >>", output)
		var logfmtParts []string
		for k, v := range fields {
			logfmtParts = append(logfmtParts, fmt.Sprintf("%s=%s", k, formatLogfmtValue(v)))
		}
		if len(logfmtParts) > 0 {
			output += " " + strings.Join(logfmtParts, " ")
		}
	}

	switch level {
	case info, debug:
		fmt.Fprintf(stdoutWriter, "%s%s\n", prefix, output)
	case warn, error, fatal:
		fmt.Fprintf(stderrWriter, "%s%s\n", prefix, output)
	}
}

func (l *StructuredLogger) Info(key string, fields map[string]any) {
	if ShouldLog(LevelInfo) {
		l.log(info, key, fields)
	}
}

func (l *StructuredLogger) Warn(key string, fields map[string]any) {
	if ShouldLog(LevelWarn) {
		l.log(warn, key, fields)
	}
}

func (l *StructuredLogger) Error(key string, fields map[string]any) {
	if ShouldLog(LevelError) {
		l.log(error, key, fields)
	}
}

func (l *StructuredLogger) Debug(key string, fields map[string]any) {
	if ShouldLog(LevelDebug) {
		l.log(debug, key, fields)
	}
}

func (l *StructuredLogger) Fatal(key string, fields map[string]any) {
	// Fatal logs are always shown when error level is enabled (fatal is not a separate configurable level)
	if ShouldLog(LevelError) {
		l.log(fatal, key, fields)
	}
	os.Exit(1)
}
