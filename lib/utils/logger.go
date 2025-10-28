package utils

import (
	"fmt"
	"os"
)

const (
	INFO  = "INFO"
	WARN  = "WARN"
	ERROR = "ERROR"
	DEBUG = "DEBUG"
)

// Logger defines the interface for logging
type Logger interface {
	Info(v ...any)
	Warn(v ...any)
	Error(v ...any)
	Debug(v ...any)
	InfoF(format string, args ...any)
	WarnF(format string, args ...any)
	ErrorF(format string, args ...any)
	DebugF(format string, args ...any)
}

// DefaultLogger is a simple logger implementation
type DefaultLogger struct {
	prefix string
}

// NewLogger creates a new logger with the given prefix
func NewLogger(prefix string) Logger {
	return &DefaultLogger{prefix: prefix}
}

func (l *DefaultLogger) getPrefix(level string) string {
	return fmt.Sprintf("[%s][%s]: ", l.prefix, level)
}

func (l *DefaultLogger) log(level string, v ...any) {
	prefix := l.getPrefix(level)
	switch level {
	case INFO, WARN, DEBUG:
		fmt.Fprint(os.Stdout, prefix)
		fmt.Fprintln(os.Stdout, v...)
		fmt.Fprintln(os.Stdout)
	case ERROR:
		fmt.Fprint(os.Stderr, prefix)
		fmt.Fprintln(os.Stderr, v...)
		fmt.Fprintln(os.Stderr)
	}
}

func (l *DefaultLogger) logF(level string, format string, v ...any) {
	prefix := l.getPrefix(level)
	switch level {
	case INFO, WARN, DEBUG:
		fmt.Fprint(os.Stdout, prefix)
		fmt.Fprintf(os.Stdout, format, v...)
		fmt.Fprintln(os.Stdout)
	case ERROR:
		fmt.Fprint(os.Stderr, prefix)
		fmt.Fprintf(os.Stderr, format, v...)
		fmt.Fprintln(os.Stderr)
	}
}

func (l *DefaultLogger) Info(v ...any) {
	l.log(INFO, v...)
}

func (l *DefaultLogger) Warn(v ...any) {
	l.log(WARN, v...)
}

func (l *DefaultLogger) Error(v ...any) {
	l.log(ERROR, v...)
}

func (l *DefaultLogger) Debug(v ...any) {
	l.log(DEBUG, v...)
}

func (l *DefaultLogger) InfoF(format string, args ...any) {
	l.logF(INFO, format, args...)
}

func (l *DefaultLogger) WarnF(format string, args ...any) {
	l.logF(WARN, format, args...)
}

func (l *DefaultLogger) ErrorF(format string, args ...any) {
	l.logF(ERROR, format, args...)
}

func (l *DefaultLogger) DebugF(format string, args ...any) {
	l.logF(DEBUG, format, args...)
}
