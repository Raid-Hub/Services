package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"raidhub/lib/utils/logging"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	jobs             []Job
	jobsMutex        sync.RWMutex
	runningJobs      map[string]*exec.Cmd
	runningJobsMutex sync.RWMutex
)

func handleJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobsMutex.RLock()
	defer jobsMutex.RUnlock()

	response := map[string]interface{}{
		"jobs": jobs,
		"env":  getEnvVars(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleTrigger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobID := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
	jobID = strings.TrimSuffix(jobID, "/trigger")

	jobsMutex.RLock()
	var job *Job
	for i := range jobs {
		if jobs[i].ID == jobID {
			job = &jobs[i]
			break
		}
	}
	jobsMutex.RUnlock()

	if job == nil {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	// Check if client wants streaming (Accept: text/event-stream)
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "text/event-stream") {
		// Stream logs in real-time
		streamJobLogs(w, job)
		return
	}

	// Execute job asynchronously (original behavior)
	go executeJob(job)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "triggered", "job_id": job.ID})
}

func handleKill(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobID := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
	jobID = strings.TrimSuffix(jobID, "/kill")

	runningJobsMutex.Lock()
	cmd, exists := runningJobs[jobID]
	runningJobsMutex.Unlock()

	if !exists {
		http.Error(w, "Job not running", http.StatusNotFound)
		return
	}

	// Kill the process
	if err := cmd.Process.Kill(); err != nil {
		logger.Error("JOB_KILL_ERROR", err, map[string]any{
			logging.JOB_ID: jobID,
		})
		http.Error(w, fmt.Sprintf("Error killing job: %v", err), http.StatusInternalServerError)
		return
	}

	logger.Info("JOB_KILLED", map[string]any{
		logging.JOB_ID: jobID,
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "killed", "job_id": jobID})
}

func streamJobLogs(w http.ResponseWriter, job *Job) {
	// Set up SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get environment variables
	env := getEnvVars()

	// Get output writers (job log files + STDOUT/STDERR env files)
	baseStdoutWriter, baseStderrWriter, stdoutFile, stderrFile, err := getOutputWriters(job.ID, env)
	if err != nil {
		logger.Error("OUTPUT_WRITERS_ERROR", err, map[string]any{
			logging.JOB_ID: job.ID,
		})
		sendSSE(w, "error", fmt.Sprintf("Error setting up output writers: %v", err))
		return
	}
	defer stdoutFile.Close()
	defer stderrFile.Close()

	// Write separator with timestamp
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(stdoutFile, "\n--- [%s] Execution started ---\n", timestamp)
	sendSSE(w, "start", fmt.Sprintf("Job %s started at %s", job.ID, timestamp))

	// Create pipes for real-time streaming
	stdoutReader, stdoutPipeWriter := io.Pipe()
	stderrReader, stderrPipeWriter := io.Pipe()

	// Build command
	cmd := exec.Command("bash", "-c", job.Command)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: false,
	}

	// Set environment variables - inherit from container, override with crontab
	cmdEnv := os.Environ() // Start with container environment

	// Override with crontab environment variables
	for k, v := range env {
		cmdEnv = append(cmdEnv, fmt.Sprintf("%s=%s", k, v))
	}

	cmd.Env = cmdEnv

	// Set up command output: base writers (job log + STDOUT/STDERR env files + console) + pipe for SSE
	cmd.Stdout = io.MultiWriter(baseStdoutWriter, stdoutPipeWriter)
	cmd.Stderr = io.MultiWriter(baseStderrWriter, stderrPipeWriter)

	// Start command
	startTime := time.Now()
	sendSSE(w, "command", fmt.Sprintf("Executing: %s", job.Command))

	// Register running job
	runningJobsMutex.Lock()
	if runningJobs == nil {
		runningJobs = make(map[string]*exec.Cmd)
	}
	runningJobs[job.ID] = cmd
	runningJobsMutex.Unlock()

	// Clean up when done
	defer func() {
		runningJobsMutex.Lock()
		delete(runningJobs, job.ID)
		runningJobsMutex.Unlock()
	}()

	if err := cmd.Start(); err != nil {
		logger.Error("JOB_START_ERROR", err, map[string]any{
			logging.JOB_ID:  job.ID,
			logging.COMMAND: job.Command,
		})
		sendSSE(w, "error", fmt.Sprintf("Error starting job: %v", err))
		return
	}

	// Stream stdout and stderr in real-time
	var wg sync.WaitGroup
	wg.Add(2)

	// Stream stdout
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdoutReader)
		for scanner.Scan() {
			line := scanner.Text()
			sendSSE(w, "stdout", line)
		}
	}()

	// Stream stderr
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderrReader)
		for scanner.Scan() {
			line := scanner.Text()
			sendSSE(w, "stderr", line)
		}
	}()

	// Wait for command to complete
	err = cmd.Wait()

	// Close the pipes to signal EOF to the scanners
	// This will cause the scanners to finish reading
	stdoutPipeWriter.Close()
	stderrPipeWriter.Close()

	// Wait for streaming goroutines to finish reading all remaining data
	wg.Wait()

	// Close the readers
	stdoutReader.Close()
	stderrReader.Close()

	if err != nil {
		duration := time.Since(startTime)
		logger.Error("JOB_COMPLETED_WITH_ERROR", err, map[string]any{
			logging.JOB_ID:   job.ID,
			logging.COMMAND:  job.Command,
			logging.DURATION: duration.String(),
		})
		sendSSE(w, "error", fmt.Sprintf("Job completed with error: %v", err))
		fmt.Fprintf(stderrFile, "\n--- Job failed with error: %v ---\n", err)
	} else {
		duration := time.Since(startTime)
		logger.Info("JOB_COMPLETED_SUCCESS", map[string]any{
			logging.JOB_ID:   job.ID,
			logging.COMMAND:  job.Command,
			logging.DURATION: duration.String(),
		})
		sendSSE(w, "complete", fmt.Sprintf("Job completed successfully in %v", duration))
	}

	sendSSE(w, "end", "")

	// Ensure the response is flushed and closed
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Close the connection by setting Connection: close header
	// The client will detect this and close the stream
	w.Header().Set("Connection", "close")
}

func sendSSE(w http.ResponseWriter, event, data string) {
	fmt.Fprintf(w, "event: %s\n", event)
	fmt.Fprintf(w, "data: %s\n\n", data)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// getOutputWriters creates writers that write to both job log files and STDOUT/STDERR env-configured destinations
func getOutputWriters(jobID string, envVars map[string]string) (stdoutWriter io.Writer, stderrWriter io.Writer, stdoutFile *os.File, stderrFile *os.File, err error) {
	// Ensure log directory exists
	if err = os.MkdirAll(logDir, 0755); err != nil {
		return
	}

	stdoutPath := filepath.Join(logDir, fmt.Sprintf("%s.log", jobID))
	stderrPath := filepath.Join(logDir, fmt.Sprintf("%s.err", jobID))

	// Open job log files
	stdoutFile, err = os.OpenFile(stdoutPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}

	stderrFile, err = os.OpenFile(stderrPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		stdoutFile.Close()
		return
	}

	// Set up stdout writer: job log + STDOUT env var file (if set) + console
	stdoutWriters := []io.Writer{stdoutFile, os.Stdout}
	if stdoutEnvPath, exists := envVars["STDOUT"]; exists && stdoutEnvPath != "" {
		if stdoutEnvFile, openErr := os.OpenFile(stdoutEnvPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); openErr == nil {
			stdoutWriters = append(stdoutWriters, stdoutEnvFile)
		}
	}
	stdoutWriter = io.MultiWriter(stdoutWriters...)

	// Set up stderr writer: job log + STDERR env var file (if set) + console
	stderrWriters := []io.Writer{stderrFile, os.Stderr}
	if stderrEnvPath, exists := envVars["STDERR"]; exists && stderrEnvPath != "" {
		if stderrEnvFile, openErr := os.OpenFile(stderrEnvPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); openErr == nil {
			stderrWriters = append(stderrWriters, stderrEnvFile)
		}
	}
	stderrWriter = io.MultiWriter(stderrWriters...)

	return
}

func executeJob(job *Job) {
	// Get environment variables
	env := getEnvVars()

	// Get output writers (job log files + STDOUT/STDERR env files)
	stdoutWriter, stderrWriter, stdoutFile, stderrFile, err := getOutputWriters(job.ID, env)
	if err != nil {
		logger.Error("OUTPUT_WRITERS_ERROR", err, map[string]any{
			logging.JOB_ID: job.ID,
		})
		return
	}
	defer stdoutFile.Close()
	defer stderrFile.Close()

	// Write separator with timestamp before each execution
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(stdoutFile, "\n--- [%s] Execution started ---\n", timestamp)
	fmt.Fprintf(stderrFile, "\n--- [%s] Execution started ---\n", timestamp)

	// Build command with shell
	// Use bash instead of sh and set SysProcAttr to avoid setpgid errors
	cmd := exec.Command("bash", "-c", job.Command)

	// Prevent setpgid errors in Docker by setting SysProcAttr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: false,
	}

	// Set environment variables - inherit from container, override with crontab
	cmdEnv := os.Environ() // Start with container environment

	// Override with crontab environment variables
	for k, v := range env {
		cmdEnv = append(cmdEnv, fmt.Sprintf("%s=%s", k, v))
	}

	cmd.Env = cmdEnv

	// Use configured writers (job log + STDOUT/STDERR env files + console)
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter

	startTime := time.Now()
	logger.Info("JOB_STARTING", map[string]any{
		logging.JOB_ID:  job.ID,
		logging.COMMAND: job.Command,
	})

	if err := cmd.Run(); err != nil {
		duration := time.Since(startTime)
		logger.Error("JOB_COMPLETED_WITH_ERROR", err, map[string]any{
			logging.JOB_ID:   job.ID,
			logging.COMMAND:  job.Command,
			logging.DURATION: duration.String(),
		})
		fmt.Fprintf(stderrFile, "\n--- Job failed with error: %v ---\n", err)
	} else {
		duration := time.Since(startTime)
		logger.Info("JOB_COMPLETED_SUCCESS", map[string]any{
			logging.JOB_ID:   job.ID,
			logging.COMMAND:  job.Command,
			logging.DURATION: duration.String(),
		})
	}
}

func handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobID := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
	jobID = strings.TrimSuffix(jobID, "/logs")

	logType := r.URL.Query().Get("type")
	if logType == "" {
		logType = "stdout"
	}

	logger.Debug("LOG_REQUEST", map[string]any{
		logging.JOB_ID:  jobID,
		logging.LOG_TYPE: logType,
	})

	stdoutPath := filepath.Join(logDir, fmt.Sprintf("%s.log", jobID))
	stderrPath := filepath.Join(logDir, fmt.Sprintf("%s.err", jobID))

	// Handle "both" option - combine stdout and stderr chronologically
	if logType == "both" {
		// Read both files and merge chronologically
		mergedLines, err := mergeLogFiles(stdoutPath, stderrPath)
		if err != nil {
			if os.IsNotExist(err) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{
					"error":  "No log files found for this job.",
					"stdout": "",
					"stderr": "",
				})
				return
			}
			logger.Error("LOG_MERGE_ERROR", err, map[string]any{
				logging.JOB_ID: jobID,
			})
			http.Error(w, "Error reading log files", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Format merged lines for JSON response
		json.NewEncoder(w).Encode(map[string]interface{}{
			logging.MERGED: mergedLines,
		})
		return
	}

	// Handle single file (stdout or stderr)
	var logPath string
	if logType == "stderr" || logType == "err" {
		logPath = stderrPath
	} else {
		logPath = stdoutPath
	}

	// Check if file exists
	fileInfo, err := os.Stat(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Warn("LOG_FILE_NOT_FOUND", nil, map[string]any{
				logging.JOB_ID: jobID,
				logging.PATH:   logPath,
			})
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("No log file found for this job."))
			return
		}
		logger.Error("LOG_FILE_STAT_ERROR", err, map[string]any{
			logging.JOB_ID: jobID,
			logging.PATH:   logPath,
		})
		http.Error(w, "Error reading log file", http.StatusInternalServerError)
		return
	}

	logger.Debug("LOG_FILE_FOUND", map[string]any{
		logging.JOB_ID: jobID,
		logging.PATH:   logPath,
		logging.SIZE:   fileInfo.Size(),
	})

	// If file is empty, return appropriate message
	if fileInfo.Size() == 0 {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Log file is empty."))
		return
	}

	file, err := os.Open(logPath)
	if err != nil {
		logger.Error("LOG_FILE_OPEN_ERROR", err, map[string]any{
			logging.JOB_ID: jobID,
			logging.PATH:   logPath,
		})
		http.Error(w, "Error reading log file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", "text/plain")
	bytesCopied, err := io.Copy(w, file)
	fields := map[string]any{
		logging.JOB_ID: jobID,	
		logging.PATH:   logPath,
		"bytes_copied": bytesCopied,
	}
	if err != nil {
		logger.Error("LOG_FILE_COPY_ERROR", err, fields)
	} else {
		logger.Debug("LOG_FILE_COPIED", fields)
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		file, err := os.Open(htmlFile)
		if err != nil {
			logger.Error("HTML_FILE_OPEN_ERROR", err, map[string]any{
				logging.PATH: htmlFile,
			})
			http.Error(w, "Failed to load HTML file", http.StatusInternalServerError)
			return
		}
		defer file.Close()

		w.Header().Set("Content-Type", "text/html")
		io.Copy(w, file)
		return
	}

	// Handle static files (CSS and JS)
	htmlDir := filepath.Dir(htmlFile)
	if r.URL.Path == "/styles.css" {
		cssFile := filepath.Join(htmlDir, "styles.css")
		file, err := os.Open(cssFile)
		if err != nil {
			logger.Error("CSS_FILE_OPEN_ERROR", err, map[string]any{
				logging.PATH: cssFile,
			})
			http.Error(w, "Failed to load CSS file", http.StatusInternalServerError)
			return
		}
		defer file.Close()
		w.Header().Set("Content-Type", "text/css")
		io.Copy(w, file)
		return
	}

	if r.URL.Path == "/app.js" {
		jsFile := filepath.Join(htmlDir, "app.js")
		file, err := os.Open(jsFile)
		if err != nil {
			logger.Error("JS_FILE_OPEN_ERROR", err, map[string]any{
				logging.PATH: jsFile,
			})
			http.Error(w, "Failed to load JS file", http.StatusInternalServerError)
			return
		}
		defer file.Close()
		w.Header().Set("Content-Type", "application/javascript")
		io.Copy(w, file)
		return
	}

	http.NotFound(w, r)
}

func setJobs(newJobs []Job) {
	jobsMutex.Lock()
	defer jobsMutex.Unlock()
	jobs = newJobs
}

type LogLine struct {
	Source  string `json:"source"` // "stdout" or "stderr"
	Content string `json:"content"`
}

// mergeLogFiles reads both stdout and stderr files and merges them chronologically
// by interleaving lines from both files. Since logs are written simultaneously,
// we merge by interleaving lines in a round-robin fashion.
func mergeLogFiles(stdoutPath, stderrPath string) ([]LogLine, error) {
	var stdoutLines []string
	var stderrLines []string

	// Read stdout file
	if stdoutFile, err := os.Open(stdoutPath); err == nil {
		scanner := bufio.NewScanner(stdoutFile)
		for scanner.Scan() {
			stdoutLines = append(stdoutLines, scanner.Text())
		}
		stdoutFile.Close()
		if err := scanner.Err(); err != nil {
			logger.Error("STDOUT_READ_ERROR", err, map[string]any{
				logging.PATH: stdoutPath,
			})
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	// Read stderr file
	if stderrFile, err := os.Open(stderrPath); err == nil {
		scanner := bufio.NewScanner(stderrFile)
		for scanner.Scan() {
			stderrLines = append(stderrLines, scanner.Text())
		}
		stderrFile.Close()
		if err := scanner.Err(); err != nil {
			logger.Error("STDERR_READ_ERROR", err, map[string]any{
				logging.PATH: stderrPath,
			})
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	// Check if both files are empty
	if len(stdoutLines) == 0 && len(stderrLines) == 0 {
		return nil, os.ErrNotExist
	}

	// Merge chronologically by interleaving lines
	// Strategy: interleave lines from both files, taking one from stdout, then one from stderr
	// This approximates chronological order since both files are written simultaneously
	merged := make([]LogLine, 0, len(stdoutLines)+len(stderrLines))

	stdoutIdx := 0
	stderrIdx := 0

	// Simple interleaving: alternate between stdout and stderr
	for stdoutIdx < len(stdoutLines) || stderrIdx < len(stderrLines) {
		// Always prefer stdout if both have lines (stdout is more common)
		if stdoutIdx < len(stdoutLines) && stderrIdx < len(stderrLines) {
			// Alternate: take from stdout first, then stderr
			merged = append(merged, LogLine{Source: "stdout", Content: stdoutLines[stdoutIdx]})
			stdoutIdx++
			merged = append(merged, LogLine{Source: "stderr", Content: stderrLines[stderrIdx]})
			stderrIdx++
		} else if stdoutIdx < len(stdoutLines) {
			merged = append(merged, LogLine{Source: "stdout", Content: stdoutLines[stdoutIdx]})
			stdoutIdx++
		} else if stderrIdx < len(stderrLines) {
			merged = append(merged, LogLine{Source: "stderr", Content: stderrLines[stderrIdx]})
			stderrIdx++
		}
	}

	return merged, nil
}
