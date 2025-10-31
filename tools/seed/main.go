package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"

	"raidhub/lib/database/postgres"
	"raidhub/lib/utils/logging"

	_ "github.com/lib/pq"
)

var logger = logging.NewLogger("SEED")

func getSeedFiles(seedsDir string) ([]string, error) {
	var seedFiles []string
	files, err := ioutil.ReadDir(seedsDir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			seedFiles = append(seedFiles, file.Name())
		}
	}

	sort.Strings(seedFiles)
	return seedFiles, nil
}

type StepData struct {
	Step   int                         `json:"step"`
	Tables map[string][]map[string]any `json:"tables"`
}

type SeedFile struct {
	Steps []StepData `json:"steps"`
}

func seedFile(db *sql.DB, filePath string) error {
	logger.Info("SEEDING_FILE", map[string]any{
		logging.FILENAME: filepath.Base(filePath),
	})

	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read seed file: %v", err)
	}

	var seedFile SeedFile
	if err := json.Unmarshal(data, &seedFile); err != nil {
		return fmt.Errorf("failed to parse JSON: %v", err)
	}

	totalInserted := 0
	for _, step := range seedFile.Steps {
		logger.Info("SEEDING_STEP", map[string]any{
			"step": step.Step,
		})

		for tableName, records := range step.Tables {
			if len(records) == 0 {
				logger.Info("SKIPPING_EMPTY_TABLE", map[string]any{
					logging.TABLE: tableName,
				})
				continue
			}

			logger.Info("SEEDING_TABLE", map[string]any{
				logging.TABLE: tableName,
				logging.TOTAL: len(records),
			})

			// Get column names from first record
			var columns []string
			for col := range records[0] {
				columns = append(columns, col)
			}

			// Build INSERT statement
			placeholders := make([]string, len(columns))
			for i := range placeholders {
				placeholders[i] = fmt.Sprintf("$%d", i+1)
			}

			qualifiedTableName := fmt.Sprintf("\"%s\".\"%s\"", "definitions", tableName)

			query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT DO NOTHING",
				qualifiedTableName,
				strings.Join(columns, ", "),
				strings.Join(placeholders, ", "))

			stmt, err := db.Prepare(query)
			if err != nil {
				return fmt.Errorf("failed to prepare statement for %s: %v", tableName, err)
			}

			inserted := 0
			for _, record := range records {
				values := make([]any, len(columns))
				for i, col := range columns {
					values[i] = record[col]
				}

				result, err := stmt.Exec(values...)
				if err != nil {
					logger.Warn("FAILED_TO_INSERT_RECORD", map[string]any{
						logging.TABLE: tableName,
						logging.ERROR: err.Error(),
					})
					continue
				}

				rowsAffected, _ := result.RowsAffected()
				if rowsAffected > 0 {
					inserted++
				}
			}

			stmt.Close()
			logger.Info("RECORDS_INSERTED", map[string]any{
				logging.TABLE: tableName,
				logging.COUNT: inserted,
				logging.TOTAL: len(records),
			})
			totalInserted += inserted
		}
	}

	logger.Info("STEP_COMPLETE", map[string]any{
		logging.COUNT: totalInserted,
	})
	return nil
}

func main() {
	logger.Info("STARTING_SEED_TOOL", nil)

	// Wait for PostgreSQL connection to be ready
	postgres.Wait()

	db := postgres.DB

	seedsDir := "infrastructure/postgres/seeds"
	seedFiles, err := getSeedFiles(seedsDir)
	if err != nil {
		logger.Fatal("FAILED_TO_GET_SEED_FILES", map[string]any{
			logging.DIRECTORY: seedsDir,
			logging.ERROR:     err.Error(),
		})
		return
	}

	if len(seedFiles) == 0 {
		logger.Info("NO_SEED_FILES_FOUND", map[string]any{
			logging.DIRECTORY: seedsDir,
		})
		return
	}

	logger.Info("FOUND_SEED_FILES", map[string]any{
		logging.COUNT:    len(seedFiles),
		logging.DIRECTORY: seedsDir,
	})

	for _, seedFileName := range seedFiles {
		filePath := filepath.Join(seedsDir, seedFileName)

		if err := seedFile(db, filePath); err != nil {
			logger.Fatal("FAILED_TO_SEED_FILE", map[string]any{
				logging.FILENAME: seedFileName,
				logging.ERROR:    err.Error(),
			})
			return
		}
	}

	logger.Info("SEEDING_COMPLETED", nil)
}

