package instance_storage

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"raidhub/lib/env"
)

var logFilePath string

func init() {
	// Get log file path from environment variable
	logFilePath = env.MissedPGCRLogFilePath
	// Ensure parent directories exist
	logDir := filepath.Dir(logFilePath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		panic(fmt.Sprintf("Failed to create missed pgcr log directory: %s", err))
	}

	// Create the file if it doesn't exist (empty file)
	if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
		file, err := os.Create(logFilePath)
		if err != nil {
			panic(fmt.Sprintf("Failed to create missed pgcr log file: %s", err))
		}
		file.Close()
	}
}

func WriteMissedLog(instanceId int64) {
	// Open the file in append mode, creating it if it doesn't exist
	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		panic(fmt.Sprintf("Failed to open missed pgcr log file: %s", err))
	}
	defer file.Close()

	// Create a writer to append to the file
	writer := bufio.NewWriter(file)

	_, err = writer.WriteString(fmt.Sprint(instanceId) + "\n")
	if err != nil {
		panic(fmt.Sprintf("Failed to write to missed pgcr log file: %s", err))
	}

	// Flush the writer to ensure the data is written to the file
	err = writer.Flush()
	if err != nil {
		panic(fmt.Sprintf("Failed to flush missed pgcr log file: %s", err))
	}
}
