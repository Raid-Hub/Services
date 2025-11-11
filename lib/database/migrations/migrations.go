package migrations

import (
	"os"
	"path/filepath"
	"raidhub/lib/utils/logging"
	"sort"
)

var MigrationsLogger = logging.NewLogger("Migrations")

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
	MigrationsLogger.Info("FOUND_MIGRATION_FILES", map[string]any{
		logging.COUNT:     len(config.MigrationFiles),
		logging.DIRECTORY: config.Directory,
	})

	// Query applied migrations
	appliedMigrations, err := config.GetAppliedMigrations()
	if err != nil {
		return err
	}

	// Apply unapplied migrations
	appliedCount := 0
	for _, filename := range config.MigrationFiles {
		if appliedMigrations[filename] {
			MigrationsLogger.Info("MIGRATION_ALREADY_APPLIED", map[string]any{
				logging.FILENAME: filename,
			})
			continue
		}

		MigrationsLogger.Info("APPLYING_MIGRATION", map[string]any{
			logging.FILENAME: filename,
		})

		migrationSQL, err := ReadMigrationFile(config.Directory, filename)
		if err != nil {
			MigrationsLogger.Error("FAILED_TO_READ_MIGRATION_FILE", err, map[string]any{
				logging.FILENAME: filename,
			})
			return err
		}

		err = config.ApplyMigration(filename, migrationSQL)
		if err != nil {
			MigrationsLogger.Error("FAILED_TO_APPLY_MIGRATION", err, map[string]any{
				logging.FILENAME: filename,
			})
			return err
		}

		MigrationsLogger.Info("MIGRATION_APPLIED", map[string]any{
			logging.FILENAME: filename,
		})
		appliedCount++
	}

	if appliedCount == 0 {
		MigrationsLogger.Info("DATABASE_UP_TO_DATE", nil)
	} else {
		MigrationsLogger.Info("MIGRATIONS_COMPLETE", map[string]any{
			logging.APPLIED_COUNT: appliedCount,
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
