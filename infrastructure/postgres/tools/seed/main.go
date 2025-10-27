package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"sort"
	"strings"

	"raidhub/lib/env"

	_ "github.com/lib/pq"
)

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

func loadConfig() (*DatabaseConfig, error) {
	config := &DatabaseConfig{
		Host:     "localhost",
		Port:     env.PostgresPort,
		User:     env.PostgresUser,
		Password: env.PostgresPassword,
		DBName:   env.PostgresDB,
	}

	return config, nil
}

func connectDB(config *DatabaseConfig) (*sql.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.Host, config.Port, config.User, config.Password, config.DBName)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	return db, nil
}

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

func seedFile(db *sql.DB, filePath string) error {
	log.Printf("  → Seeding from %s", filepath.Base(filePath))

	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read seed file: %v", err)
	}

	var seedData map[string][]map[string]any
	if err := json.Unmarshal(data, &seedData); err != nil {
		return fmt.Errorf("failed to parse JSON: %v", err)
	}

	totalInserted := 0
	for tableName, records := range seedData {
		if len(records) == 0 {
			log.Printf("    %s: No records to insert", tableName)
			continue
		}

		log.Printf("    %s: %d records", tableName, len(records))

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
				log.Printf("      Warning: failed to insert record: %v", err)
				continue
			}

			rowsAffected, _ := result.RowsAffected()
			if rowsAffected > 0 {
				inserted++
			}
		}

		stmt.Close()
		log.Printf("      Inserted %d/%d records", inserted, len(records))
		totalInserted += inserted
	}

	log.Printf("    Total inserted: %d records", totalInserted)
	return nil
}

func main() {
	log.Println("PostgreSQL Seeding Tool")
	log.Println("======================")

	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := connectDB(config)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	seedsDir := "infrastructure/postgres/seeds"
	seedFiles, err := getSeedFiles(seedsDir)
	if err != nil {
		log.Fatalf("Failed to get seed files: %v", err)
	}

	if len(seedFiles) == 0 {
		log.Println("No seed files found")
		return
	}

	log.Printf("Found %d seed files in %s", len(seedFiles), seedsDir)

	for _, seedFileName := range seedFiles {
		filePath := filepath.Join(seedsDir, seedFileName)

		if err := seedFile(db, filePath); err != nil {
			log.Fatalf("Failed to seed %s: %v", seedFileName, err)
		}
	}

	log.Println("\n✅ Seeding completed successfully!")
}
