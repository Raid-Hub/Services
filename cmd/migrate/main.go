package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"raidhub/packages/postgres"
	"strconv"
	"strings"

	_ "github.com/lib/pq"
)

func getMigrationFiles(directory string) ([]string, error) {
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
	return migrationFiles, nil
}

func readMigrationFile(directory, filename string) (string, error) {
	filePath := filepath.Join(directory, filename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func applyMigration(tx *sql.Tx, id int, migrationName string, migrationSQL string) error {
	for _, statement := range strings.Split(migrationSQL, ";") {
		log.Println(statement)
		_, err := tx.Exec(fmt.Sprintf("%s;", statement))
		if err != nil {
			return err
		}
	}

	_, err := tx.Exec("INSERT INTO _migrations (id, name) VALUES ($1, $2)", id, migrationName)
	return err
}

func main() {
	db, err := postgres.Connect()
	if err != nil {
		log.Println("Error connecting to the database:", err)
		return
	}
	defer db.Close()

	migrationDirectory := "docker/migrations"
	migrationFiles, err := getMigrationFiles(migrationDirectory)
	if err != nil {
		log.Println("Error getting migration files:", err)
		return
	}

	applyingMigrations := false
	for i, filename := range migrationFiles {

		tx, err := db.Begin()
		if err != nil {
			log.Fatalf("Error starting transaction: %s\n", err)
		}
		defer tx.Rollback()

		migrationSQL, err := readMigrationFile(migrationDirectory, filename)
		if err != nil {
			log.Fatalf("Error reading migration file '%s': %s\n", filename, err)
		}

		parts := strings.Split(filename, "_")

		// To get the id
		id, err := strconv.Atoi(parts[0])
		if err != nil {
			log.Fatalf("Error reading migration file '%s': %s\n", filename, err)
		}

		if id != i+1 {
			log.Fatalf("Error: migration file '%s' has an invalid id\n", filename)
		}

		// To get the name without extension
		nameWithExt := parts[1]
		migrationName := nameWithExt[:len(nameWithExt)-len(filepath.Ext(nameWithExt))] // "name"

		// Check if migration exists in _migrations table
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM _migrations WHERE id = $1", id).Scan(&count)
		if err != nil {
			log.Fatalf("Error checking migration '%s' in _migrations table: %s\n", migrationName, err)
		}

		if count == 0 {
			// Migration not found, apply it
			applyingMigrations = true
			err := applyMigration(tx, id, migrationName, migrationSQL)
			if err != nil {
				log.Fatalf("Error applying migration '%s': %s\n", migrationName, err)
			}
			err = tx.Commit()
			if err != nil {
				log.Fatalf("Error applying migration '%s': %s\n", migrationName, err)
			}
			log.Println("Applied migration:", migrationName)
		} else if applyingMigrations {
			log.Fatalf("Error applying migration '%s': previous migration did not apply\n", migrationName)
		}

	}

}
