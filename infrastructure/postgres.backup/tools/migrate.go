package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

func connectDB() (*sql.DB, error) {
	// Load environment variables
	godotenv.Load()

	user := os.Getenv("POSTGRES_USER")
	if user == "" {
		user = "username"
	}

	password := os.Getenv("POSTGRES_PASSWORD")
	if password == "" {
		password = "password"
	}

	host := os.Getenv("POSTGRES_HOST")
	if host == "" {
		host = "localhost"
	}

	port := os.Getenv("POSTGRES_PORT")
	if port == "" {
		port = "5432"
	}

	dbName := os.Getenv("POSTGRES_DB")
	if dbName == "" {
		dbName = "raidhub"
	}

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbName)

	return sql.Open("postgres", dsn)
}

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

	// Sort files by name (timestamps naturally sort chronologically)
	sort.Strings(migrationFiles)
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

func applyMigration(tx *sql.Tx, filename, migrationSQL string) error {
	// Split SQL by semicolons and execute each statement
	statements := strings.Split(migrationSQL, ";")
	
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		
		// Add semicolon back
		if !strings.HasSuffix(stmt, ";") {
			stmt += ";"
		}
		
		_, err := tx.Exec(stmt)
		if err != nil {
			// Check if it's a "already exists" error and skip
			pqErr, ok := err.(*pq.Error)
			if ok && pqErr.Code == "42P07" { // duplicate_table error
				log.Printf("  Skipping (already exists): %s", pqErr.Message)
				continue
			}
			return fmt.Errorf("error executing statement: %w\nStatement: %s", err, stmt)
		}
	}

	// Record migration
	_, err := tx.Exec("INSERT INTO _migrations (name, applied_at) VALUES ($1, $2)", filename, time.Now())
	return err
}

func main() {
	log.Println("PostgreSQL Migration Tool")
	log.Println("=========================")

	db, err := connectDB()
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	defer db.Close()

	// Ensure _migrations table exists
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS _migrations (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			applied_at TIMESTAMP DEFAULT NOW()
		)
	`)
	if err != nil {
		log.Fatalf("Error creating _migrations table: %v", err)
	}

	// Process both schemas and seeds
	schemaDirectory := "infrastructure/postgres/schemas"
	seedDirectory := "infrastructure/postgres/seeds"
	
	allFiles := make(map[string]string) // filename -> directory
	
	// Get schema files
	schemaFiles, err := getMigrationFiles(schemaDirectory)
	if err != nil {
		log.Fatalf("Error getting schema files: %v", err)
	}
	for _, file := range schemaFiles {
		allFiles[file] = schemaDirectory
	}
	
	// Get seed files
	seedFiles, err := getMigrationFiles(seedDirectory)
	if err != nil {
		log.Fatalf("Error getting seed files: %v", err)
	}
	for _, file := range seedFiles {
		allFiles[file] = seedDirectory
	}
	
	if len(allFiles) == 0 {
		log.Println("No migration or seed files found")
		return
	}
	
	log.Printf("Found %d total files (%d schemas, %d seeds)\n", len(allFiles), len(schemaFiles), len(seedFiles))

	// Check which migrations/seeds have been applied
	appliedMigrations := make(map[string]bool)
	rows, err := db.Query("SELECT name FROM _migrations")
	if err != nil {
		log.Fatalf("Error querying applied migrations: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			log.Fatalf("Error scanning migration name: %v", err)
		}
		appliedMigrations[name] = true
	}

	// Apply unapplied migrations and seeds
	appliedCount := 0
	for filename := range allFiles {
		if appliedMigrations[filename] {
			log.Printf("✓ %s (already applied)", filename)
			continue
		}

		directory := allFiles[filename]
		fileType := "migration"
		if directory == seedDirectory {
			fileType = "seed"
		}
		
		log.Printf("→ Applying %s (%s)...", filename, fileType)

		tx, err := db.Begin()
		if err != nil {
			log.Fatalf("Error starting transaction: %v", err)
		}

		migrationSQL, err := readMigrationFile(directory, filename)
		if err != nil {
			tx.Rollback()
			log.Fatalf("Error reading file '%s': %v", filename, err)
		}

		err = applyMigration(tx, filename, migrationSQL)
		if err != nil {
			tx.Rollback()
			log.Fatalf("Error applying '%s': %v", filename, err)
		}

		err = tx.Commit()
		if err != nil {
			log.Fatalf("Error committing '%s': %v", filename, err)
		}

		log.Printf("✓ Applied %s", filename)
		appliedCount++
	}

	if appliedCount == 0 {
		log.Println("\n✓ Database is up to date")
	} else {
		log.Printf("\n✓ Applied %d new file(s)", appliedCount)
	}
}

