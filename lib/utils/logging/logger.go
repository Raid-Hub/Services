package logging

// Logger defines the interface for logging
type Logger interface {
	Debug(key string, fields map[string]any) // Only when verbose flag is passed
	Info(key string, fields map[string]any)
	Warn(key string, fields map[string]any)  // monitored, but not alerted
	Error(key string, fields map[string]any) // monitored/alerted
	Fatal(key string, fields map[string]any) // Crashes with exit code 1
}
