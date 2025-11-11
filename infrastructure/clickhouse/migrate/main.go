package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"raidhub/lib/database/clickhouse"
	"raidhub/lib/env"
	"raidhub/lib/migrations"
)

func applyMigration(filename, migrationSQL string) error {
	ctx := context.Background()

	clickhouse.Wait()

	// Split SQL by semicolons and execute each statement
	statements := splitSQL(migrationSQL)
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		if err := clickhouse.DB.Exec(ctx, stmt); err != nil {
			// Check if it's an "already exists" error
			if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "Duplicate") {
				log.Printf("  Skipping (already exists): %s", err.Error())
				continue
			}
			return fmt.Errorf("error executing migration: %w", err)
		}
	}

	// Record migration with explicit database
	migrationsTable := fmt.Sprintf("%s._migrations", env.ClickHouseDB)
	if err := clickhouse.DB.Exec(ctx, fmt.Sprintf("INSERT INTO %s (name) VALUES (?)", migrationsTable), filename); err != nil {
		return fmt.Errorf("error recording migration: %w", err)
	}

	return nil
}

func splitSQL(sql string) []string {
	// Simple split by semicolon, ignoring those inside strings
	var statements []string
	var current strings.Builder
	inString := false
	escapeNext := false

	for _, c := range sql {
		if escapeNext {
			current.WriteRune(c)
			escapeNext = false
			continue
		}

		if c == '\\' {
			escapeNext = true
			current.WriteRune(c)
			continue
		}

		if c == '\'' {
			inString = !inString
			current.WriteRune(c)
			continue
		}

		if c == ';' && !inString {
			statements = append(statements, current.String())
			current.Reset()
			continue
		}

		current.WriteRune(c)
	}

	// Add the last statement if any
	if current.Len() > 0 {
		statements = append(statements, current.String())
	}

	return statements
}

func main() {
	log.Println("ClickHouse Migration Tool")
	log.Println("=========================")

	// Wait for ClickHouse initialization to complete
	clickhouse.Wait()

	// Use the existing ClickHouse connection from singleton
	conn := clickhouse.DB

	ctx := context.Background()

	// Ensure _migrations table exists
	createMigrationsTableSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s._migrations
		(
			name String,
			applied_at DateTime DEFAULT now()
		)
		ENGINE = ReplacingMergeTree(applied_at)
		ORDER BY name
	`, env.ClickHouseDB)
	if err := conn.Exec(ctx, createMigrationsTableSQL); err != nil {
		log.Fatalf("Error creating _migrations table: %v", err)
	}

	migrationDirectory := "infrastructure/clickhouse/migrations"

	// Check if directory exists
	if _, err := os.Stat(migrationDirectory); os.IsNotExist(err) {
		log.Printf("Migration directory '%s' does not exist, creating it...", migrationDirectory)
		if err := os.MkdirAll(migrationDirectory, 0755); err != nil {
			log.Fatalf("Error creating migration directory: %v", err)
		}
		log.Println("No migration files found")
		return
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
		migrationsQuery := fmt.Sprintf("SELECT name FROM %s._migrations", env.ClickHouseDB)
		rows, err := conn.Query(ctx, migrationsQuery)
		if err != nil {
			log.Printf("Warning: Could not query applied migrations (table may be empty): %v", err)
			return appliedMigrations, nil
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
