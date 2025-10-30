package logging

import (
	"fmt"
	"os"
)

// StructuredLogger is a simple logger implementation
type StructuredLogger struct {
	prefix string
}

// NewLogger creates a new logger with the given prefix
func NewLogger(prefix string) Logger {
	return &StructuredLogger{prefix: prefix}
}

func (l *StructuredLogger) log(level string, key string, fields map[string]any) {
	prefix := fmt.Sprintf("[%s][%s] ", level, l.prefix)

	// Build structured log entry
	var output = fmt.Sprintf(":: %s", key)
	if len(fields) != 0 {
		output = fmt.Sprintf("%s ::", output)
		for key, value := range fields {
			output += fmt.Sprintf(" %s=%v", key, value)
		}
	}

	switch level {
	case info, debug:
		fmt.Fprintf(os.Stdout, "%s%s\n", prefix, output)
	case warn, error, fatal:
		fmt.Fprintf(os.Stderr, "%s%s\n", prefix, output)
	}
}

func (l *StructuredLogger) Info(key string, fields map[string]any) {
	l.log(info, key, fields)
}

func (l *StructuredLogger) Warn(key string, fields map[string]any) {
	l.log(warn, key, fields)
}

func (l *StructuredLogger) Error(key string, fields map[string]any) {
	l.log(error, key, fields)
}

func (l *StructuredLogger) Debug(key string, fields map[string]any) {
	if IsVerbose() {
		l.log(debug, key, fields)
	}
}

func (l *StructuredLogger) Fatal(key string, fields map[string]any) {
	l.log(fatal, key, fields)
	os.Exit(1)
}
