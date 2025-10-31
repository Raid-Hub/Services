package migrations

import (
	"os"
	"path/filepath"
	"raidhub/lib/utils/logging"
	"sort"
)

// Migrations logging constants
const (
	MIGRATIONS_UP_TO_DATE = "MIGRATIONS_UP_TO_DATE"
	MIGRATIONS_FILES_READ = "MIGRATIONS_FILES_READ"
)

var logger = logging.NewLogger("MIGRATIONS")

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
	// Query applied migrations
	appliedMigrations, err := config.GetAppliedMigrations()
	if err != nil {
		return err
	}

	// Apply unapplied migrations
	appliedCount := 0
	for _, filename := range config.MigrationFiles {
		if appliedMigrations[filename] {
			continue
		}

		migrationSQL, err := ReadMigrationFile(config.Directory, filename)
		if err != nil {
			return err
		}

		err = config.ApplyMigration(filename, migrationSQL)
		if err != nil {
			return err
		}

		appliedCount++
	}

	if appliedCount == 0 {
		logger.Info(MIGRATIONS_UP_TO_DATE, nil)
	} else {
		logger.Info("MIGRATIONS_COMPLETED", map[string]any{
			logging.COUNT: appliedCount,
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
