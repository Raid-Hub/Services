package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"raidhub/lib/database/postgres"
	"raidhub/lib/migrations"

	"github.com/lib/pq"
)

func applyMigration(filename, migrationSQL string) error {
	db := postgres.DB
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

	// Use the existing PostgreSQL connection from singleton
	db := postgres.DB

	// Use new unified migration directory
	migrationDirectory := "infrastructure/postgres/migrations"

	// Check if new structure exists, fallback to old structure
	if _, err := os.Stat(migrationDirectory); os.IsNotExist(err) {
		log.Println("New migration structure not found, using legacy structure...")
		migrationDirectory = "infrastructure/postgres/migrations.new"
	}

	migrationFiles, err := migrations.GetMigrationFiles(migrationDirectory)
	if err != nil {
		log.Fatalf("Error getting migration files: %v", err)
	}

	if len(migrationFiles) == 0 {
		log.Println("No migration files found")
		return
	}

	// Create migration config
	getAppliedMigrations := func() (map[string]bool, error) {
		appliedMigrations := make(map[string]bool)
		rows, err := db.Query("SELECT name FROM _migrations")
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return nil, err
			}
			appliedMigrations[name] = true
		}
		return appliedMigrations, nil
	}

	config := migrations.MigrationConfig{
		Directory:            migrationDirectory,
		MigrationFiles:       migrationFiles,
		GetAppliedMigrations: getAppliedMigrations,
		ApplyMigration: func(filename, sql string) error {
			return applyMigration(filename, sql)
		},
	}

	if err := migrations.RunMigrations(config); err != nil {
		log.Fatalf("Error running migrations: %v", err)
	}
}
