package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"raidhub/lib/env"
)

var (
	stdoutWriter io.Writer = os.Stdout
	stderrWriter io.Writer = os.Stderr
)

func init() {
	// Check for STDOUT environment variable
	if env.StdoutPath != "" {
		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(env.StdoutPath), 0755); err != nil {
			panic(fmt.Errorf("failed to create stdout directory: %v", err))
		}
		
		file, err := os.OpenFile(env.StdoutPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			// Use MultiWriter to write to both file and stdout
			stdoutWriter = io.MultiWriter(os.Stdout, file)
		} else {
			panic(fmt.Errorf("failed to open stdout file: %v", err))
		}
	}

	// Check for STDERR environment variable
	if env.StderrPath != "" {
		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(env.StderrPath), 0755); err != nil {
			panic(fmt.Errorf("failed to create stderr directory: %v", err))
		}
		
		file, err := os.OpenFile(env.StderrPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			// Use MultiWriter to write to both file and stderr
			stderrWriter = io.MultiWriter(os.Stderr, file)
		} else {
			panic(fmt.Errorf("failed to open stderr file: %v", err))
		}
	}
}