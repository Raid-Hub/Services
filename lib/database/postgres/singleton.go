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
	// Set search_path to include all schemas in priority order
	searchPath := "public,core,definitions,clan,flagging,leaderboard,extended,raw"
	connStr := fmt.Sprintf("user=%s dbname=%s password=%s sslmode=disable search_path=%s",
		env.PostgresUser, env.PostgresDB, env.PostgresPassword, searchPath)
	return sql.Open("postgres", connStr)
}
