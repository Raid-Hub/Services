package utils

import "log"

// Logger defines the interface for logging
type Logger interface {
	Info(msg string, fields ...any)
	Warn(msg string, fields ...any)
	Error(msg string, fields ...any)
	Debug(msg string, fields ...any)
}

// DefaultLogger is a simple logger implementation
type DefaultLogger struct {
	prefix string
}

// NewLogger creates a new logger with the given prefix
func NewLogger(prefix string) Logger {
	return &DefaultLogger{prefix: prefix}
}

func (l *DefaultLogger) Info(msg string, fields ...any) {
	log.Printf("[%s] INFO: %s %v", l.prefix, msg, fields)
}

func (l *DefaultLogger) Warn(msg string, fields ...any) {
	log.Printf("[%s] WARN: %s %v", l.prefix, msg, fields)
}

func (l *DefaultLogger) Error(msg string, fields ...any) {
	log.Printf("[%s] ERROR: %s %v", l.prefix, msg, fields)
}

func (l *DefaultLogger) Debug(msg string, fields ...any) {
	log.Printf("[%s] DEBUG: %s %v", l.prefix, msg, fields)
}
