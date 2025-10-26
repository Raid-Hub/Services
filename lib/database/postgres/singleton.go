package postgres

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"raidhub/lib/env"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func init() {
	// Retry connection with backoff
	maxRetries := 10
	var err error
	for i := 0; i < maxRetries; i++ {
		DB, err = connect()
		if err == nil {
			log.Printf("PostgreSQL connected")
			return
		}
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	panic(fmt.Sprintf("Failed to connect to PostgreSQL after %d retries: %v", maxRetries, err))
}

func connect() (*sql.DB, error) {
	connStr := fmt.Sprintf("user=%s dbname=%s password=%s sslmode=disable", env.PostgresUser, env.PostgresDB, env.PostgresPassword)
	return sql.Open("postgres", connStr)
}
