package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"raidhub/lib/env"
	"raidhub/lib/utils/logging"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

var logger = logging.NewLogger("CRON_MANAGER")

var (
	crontabFile = "/var/spool/cron/crontabs/root" // Used for file watcher path only
	logDir      = "/var/log"
	htmlFile    = "/usr/local/share/cron-manager/index.html"
	port        = env.CronManagerPort
)

func reloadCrontab() error {
	// Parse crontab using crontab -l (ignores crontabFile param, uses crontab -l)
	newJobs, newEnvVars, err := parseCrontab()
	if err != nil {
		return err
	}

	setJobs(newJobs)
	setEnvVars(newEnvVars)

	logger.Info("CRONTAB_RELOADED", map[string]any{
		"job_count": len(newJobs),
	})
	return nil
}

func setupFileWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
					logger.Info("CRONTAB_FILE_CHANGED", map[string]any{
						"event": event.String(),
					})
					time.Sleep(100 * time.Millisecond) // Small delay to ensure file is fully written
					if err := reloadCrontab(); err != nil {
						logger.Error("CRONTAB_RELOAD_ERROR", err, nil)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				logger.Error("FILE_WATCHER_ERROR", err, nil)
			}
		}
	}()

	// Watch the directory containing the crontab file
	dir := filepath.Dir(crontabFile)
	if err := watcher.Add(dir); err != nil {
		return fmt.Errorf("failed to watch directory: %w", err)
	}

	return nil
}

func main() {
	// Initial parse
	if err := reloadCrontab(); err != nil {
		logger.Fatal("CRONTAB_PARSE_FAILED", err, nil)
	}

	// Setup file watcher
	if err := setupFileWatcher(); err != nil {
		logger.Warn("FILE_WATCHER_SETUP_FAILED", err, nil)
	}

	if port == "" {
		logger.Fatal("CRON_MANAGER_PORT_NOT_SET", nil, nil)
	}

	// Setup HTTP routes
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/api/jobs", handleJobs)
	http.HandleFunc("/api/jobs/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/trigger") {
			handleTrigger(w, r)
		} else if strings.HasSuffix(r.URL.Path, "/kill") {
			handleKill(w, r)
		} else if strings.HasSuffix(r.URL.Path, "/logs") {
			handleLogs(w, r)
		} else {
			http.NotFound(w, r)
		}
	})

	logger.Info("CRON_MANAGER_STARTING", map[string]any{
		"port": port,
	})
	httpErr := http.ListenAndServe(":"+port, nil)
	logger.Fatal("HTTP_SERVER_ERROR", httpErr, nil)
}
