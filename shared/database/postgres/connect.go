package postgres

import (
	"database/sql"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func Connect() (*sql.DB, error) {
	if err := godotenv.Load(); err != nil {
		return nil, err
	}

	POSTGRES_USER := os.Getenv("POSTGRES_USER")
	POSTGRES_PASSWORD := os.Getenv("POSTGRES_PASSWORD")

	connStr := "user=" + POSTGRES_USER + " dbname=raidhub password=" + POSTGRES_PASSWORD + " sslmode=disable"

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	return db, nil
}
