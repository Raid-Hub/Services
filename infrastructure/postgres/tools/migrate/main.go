package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
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

	// Connect directly to the configured database
	// The init script should have already set up the user and database
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbName)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to database '%s' with user '%s': %v", dbName, user, err)
	}

	return db, nil
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

func applyMigration(db *sql.DB, filename, migrationSQL string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute the entire migration as one statement
	// This handles dollar-quoted strings and complex SQL properly
	_, err = tx.Exec(migrationSQL)
	if err != nil {
		// Check if it's a "already exists" error and skip
		pqErr, ok := err.(*pq.Error)
		if ok && pqErr.Code == "42P07" { // duplicate_table error
			log.Printf("  Skipping (already exists): %s", pqErr.Message)
			return nil
		} else {
			return fmt.Errorf("error executing migration: %w", err)
		}
	}

	// Record migration
	_, err = tx.Exec("INSERT INTO _migrations (name, applied_at) VALUES ($1, $2)", filename, time.Now())
	if err != nil {
		return fmt.Errorf("error recording migration: %w", err)
	}

	return tx.Commit()
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

	// Use new unified migration directory
	migrationDirectory := "infrastructure/postgres/migrations"

	// Check if new structure exists, fallback to old structure
	if _, err := os.Stat(migrationDirectory); os.IsNotExist(err) {
		log.Println("New migration structure not found, using legacy structure...")
		migrationDirectory = "infrastructure/postgres/migrations.new"
	}

	migrationFiles, err := getMigrationFiles(migrationDirectory)
	if err != nil {
		log.Fatalf("Error getting migration files: %v", err)
	}

	if len(migrationFiles) == 0 {
		log.Println("No migration files found")
		return
	}

	log.Printf("Found %d migration files in %s", len(migrationFiles), migrationDirectory)

	// Check which migrations have been applied
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

	// Apply unapplied migrations
	appliedCount := 0
	for _, filename := range migrationFiles {
		if appliedMigrations[filename] {
			log.Printf("✓ %s (already applied)", filename)
			continue
		}

		log.Printf("→ Applying migration: %s", filename)

		migrationSQL, err := readMigrationFile(migrationDirectory, filename)
		if err != nil {
			log.Fatalf("Error reading file '%s': %v", filename, err)
		}

		err = applyMigration(db, filename, migrationSQL)
		if err != nil {
			log.Fatalf("Error applying '%s': %v", filename, err)
		}

		log.Printf("✓ Applied %s", filename)
		appliedCount++
	}

	if appliedCount == 0 {
		log.Println("\n✓ Database is up to date")
	} else {
		log.Printf("\n✓ Applied %d new migration(s)", appliedCount)
	}
}
