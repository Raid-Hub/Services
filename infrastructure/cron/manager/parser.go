package main

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
)

var (
	envVarsGlobal map[string]string
	envVarsMutex  sync.RWMutex
)

type Job struct {
	ID        string   `json:"id"`
	Schedules []string `json:"schedules"`
	Command   string   `json:"command"`
	Comment   string   `json:"comment,omitempty"`
}

func parseCrontab() ([]Job, map[string]string, error) {
	// Use crontab -l to get readable format (dcron stores in binary format)
	cmd := exec.Command("crontab", "-l")
	output, err := cmd.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read crontab: %w", err)
	}

	envVars := make(map[string]string)
	var currentComment string
	var currentJobID string
	jobIndex := 0

	// Map to track jobs by ID for combining
	jobMap := make(map[string]*Job)

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	cronPattern := regexp.MustCompile(`^(\S+\s+\S+\s+\S+\s+\S+\s+\S+)\s+(.+)$`)
	envPattern := regexp.MustCompile(`^([A-Z_][A-Z0-9_]*)=(.*)$`)
	idPattern := regexp.MustCompile(`(?i)^id\s*:\s*(.+)$`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines (but preserve comment for next job)
		if line == "" {
			continue
		}

		// Check for ENV variable
		if matches := envPattern.FindStringSubmatch(line); matches != nil {
			envVars[matches[1]] = matches[2]
			currentComment = "" // Reset comment after ENV vars
			currentJobID = ""   // Reset job ID after ENV vars
			continue
		}

		// Check for comment (starts with #)
		if strings.HasPrefix(line, "#") {
			commentText := strings.TrimSpace(strings.TrimPrefix(line, "#"))
			// Check if this is an ID comment
			if matches := idPattern.FindStringSubmatch(commentText); matches != nil {
				currentJobID = strings.TrimSpace(matches[1])
			} else if commentText != "" && !strings.HasPrefix(commentText, "=") {
				// Accumulate regular comment text (last comment before job is used)
				currentComment = commentText
			}
			continue
		}

		// Check for cron job
		if matches := cronPattern.FindStringSubmatch(line); matches != nil {
			schedule := matches[1]
			command := matches[2]

			// Use stored job ID if available, otherwise extract from comment
			jobID := currentJobID
			if jobID == "" {
				jobID = extractJobID(currentComment, jobIndex)
			}
			if jobID == "" {
				jobID = fmt.Sprintf("job-%d", jobIndex)
			}

			// Check if job with this ID already exists
			if existingJob, exists := jobMap[jobID]; exists {
				// Add schedule to existing job
				existingJob.Schedules = append(existingJob.Schedules, schedule)
			} else {
				// Create new job
				job := &Job{
					ID:        jobID,
					Schedules: []string{schedule},
					Command:   command,
					Comment:   currentComment,
				}
				jobMap[jobID] = job
			}

			jobIndex++
			currentComment = "" // Reset comment after processing job
			// Don't reset currentJobID - keep it for multiple entries with same ID
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("error reading crontab output: %w", err)
	}

	// Convert map back to slice to ensure all updates are reflected
	result := make([]Job, 0, len(jobMap))
	for _, job := range jobMap {
		result = append(result, *job)
	}

	// Sort jobs by ID alphabetically
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	return result, envVars, nil
}

func extractJobID(comment string, index int) string {
	if comment == "" {
		return ""
	}

	// Try patterns: "name: <id>", "<id>", "id: <id>"
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:name|id)\s*:\s*(\S+)`),
		regexp.MustCompile(`^(\S+)`),
	}

	for _, pattern := range patterns {
		if matches := pattern.FindStringSubmatch(comment); matches != nil {
			return strings.TrimSpace(matches[1])
		}
	}

	return ""
}

func getEnvVars() map[string]string {
	envVarsMutex.RLock()
	defer envVarsMutex.RUnlock()

	result := make(map[string]string)
	for k, v := range envVarsGlobal {
		result[k] = v
	}
	return result
}

func setEnvVars(env map[string]string) {
	envVarsMutex.Lock()
	defer envVarsMutex.Unlock()
	envVarsGlobal = env
}
