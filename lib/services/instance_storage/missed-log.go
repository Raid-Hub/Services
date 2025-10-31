package instance_storage

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"raidhub/lib/env"
	"raidhub/lib/utils/logging"
)

var logFilePath string

func expandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			panic(fmt.Sprintf("Failed to get user home directory: %s", err))
		}
		if path == "~" {
			return homeDir
		}
		// Handle ~/path and ~/path formats
		if len(path) > 1 && path[1] == '/' {
			return filepath.Join(homeDir, path[2:])
		}
		// Handle ~user format (if needed in future)
		return filepath.Join(homeDir, path[1:])
	}
	return path
}

func init() {
	// Get log file path from environment variable (supports ~ expansion if needed)
	logFilePath = expandPath(env.MissedPGCRLogFilePath)
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
		logger.Fatal("FAILED_TO_OPEN_MISSED_LOG_FILE", map[string]any{
			logging.ERROR: err.Error(),
		})
		return
	}
	defer file.Close()

	// Create a writer to append to the file
	writer := bufio.NewWriter(file)

	_, err = writer.WriteString(fmt.Sprint(instanceId) + "\n")
	if err != nil {
		logger.Fatal("FAILED_TO_WRITE_TO_MISSED_LOG_FILE", map[string]any{
			logging.ERROR:       err.Error(),
			logging.INSTANCE_ID: instanceId,
		})
		return
	}

	// Flush the writer to ensure the data is written to the file
	err = writer.Flush()
	if err != nil {
		logger.Fatal("FAILED_TO_FLUSH_MISSED_LOG_FILE", map[string]any{
			logging.ERROR:       err.Error(),
			logging.INSTANCE_ID: instanceId,
		})
		return
	}
}
