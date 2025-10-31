package migrations

import (
	"os"
	"path/filepath"
	"raidhub/lib/utils/logging"
	"sort"
)

var logger = logging.NewLogger("Migrations")

func Info(message string, fields map[string]any) {
	logger.Info(message, fields)
}

func Debug(message string, fields map[string]any) {
	logger.Debug(message, fields)
}

func Warn(message string, fields map[string]any) {
	logger.Warn(message, fields)
}

func Error(message string, fields map[string]any) {
	logger.Error(message, fields)
}

// GetMigrationFiles reads all SQL files from a directory and returns them sorted
func GetMigrationFiles(directory string) ([]string, error) {
	var migrationFiles []string
	files, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if filepath.Ext(file.Name()) == ".sql" {
			migrationFiles = append(migrationFiles, file.Name())
		}
	}

	// Sort files by name
	sort.Strings(migrationFiles)
	return migrationFiles, nil
}

// ReadMigrationFile reads a migration file and returns its contents
func ReadMigrationFile(directory, filename string) (string, error) {
	filePath := filepath.Join(directory, filename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// RunMigrations is a generic migration runner that handles the common logic
func RunMigrations(config MigrationConfig) error {
	Info("FOUND_MIGRATION_FILES", map[string]any{
		"count":     len(config.MigrationFiles),
		"directory": config.Directory,
	})

	// Query applied migrations
	appliedMigrations, err := config.GetAppliedMigrations()
	if err != nil {
		Error("FAILED_TO_GET_APPLIED_MIGRATIONS", map[string]any{
			"error": err.Error(),
		})
		return err
	}

	// Apply unapplied migrations
	appliedCount := 0
	for _, filename := range config.MigrationFiles {
		if appliedMigrations[filename] {
			Debug("MIGRATION_ALREADY_APPLIED", map[string]any{
				"filename": filename,
			})
			continue
		}

		Info("APPLYING_MIGRATION", map[string]any{
			"filename": filename,
		})

		migrationSQL, err := ReadMigrationFile(config.Directory, filename)
		if err != nil {
			Error("FAILED_TO_READ_MIGRATION_FILE", map[string]any{
				"filename": filename,
				"error":    err.Error(),
			})
			return err
		}

		err = config.ApplyMigration(filename, migrationSQL)
		if err != nil {
			Error("FAILED_TO_APPLY_MIGRATION", map[string]any{
				"filename": filename,
				"error":    err.Error(),
			})
			return err
		}

		Info("MIGRATION_APPLIED", map[string]any{
			"filename": filename,
		})
		appliedCount++
	}

	if appliedCount == 0 {
		Info("DATABASE_UP_TO_DATE", nil)
	} else {
		Info("MIGRATIONS_COMPLETE", map[string]any{
			"applied_count": appliedCount,
		})
	}

	return nil
}

// MigrationConfig holds the configuration for running migrations
type MigrationConfig struct {
	Directory            string
	MigrationFiles       []string
	GetAppliedMigrations func() (map[string]bool, error)
	ApplyMigration       func(filename, sql string) error
}
