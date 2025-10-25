package pgcr

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

var onceCreateLog sync.Once
var logDir string

func createLogsDirIfNotExist() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting cwd", err)
		return
	}

	// Create the logs directory if it doesn't exist
	logDir = filepath.Join(cwd, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Fatalln(err)
	}

}

func WriteMissedLog(instanceId int64) {
	onceCreateLog.Do(createLogsDirIfNotExist)

	// Open the file in append mode with write permissions
	file, err := os.OpenFile(filepath.Join(logDir, "missed.log"), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()

	// Create a writer to append to the file
	writer := bufio.NewWriter(file)

	// Write the line you want to append
	_, err = writer.WriteString(fmt.Sprint(instanceId) + "\n")
	if err != nil {
		log.Fatalln(err)
	}

	// Flush the writer to ensure the data is written to the file
	err = writer.Flush()
	if err != nil {
		log.Fatalln(err)
	}
}
