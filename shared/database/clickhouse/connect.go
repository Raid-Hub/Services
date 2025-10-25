package clickhouse

import (
	"log"
	"os"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/joho/godotenv"
)

func Connect(debug bool) (driver.Conn, error) {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	user := os.Getenv("CLICKHOUSE_USER")
	if user == "" {
		user = "default"
	}

	password := os.Getenv("CLICKHOUSE_PASSWORD")

	return ch.Open(&ch.Options{
		Settings: ch.Settings{
			"flatten_nested": 0,
		},
		MaxOpenConns: 50,
		MaxIdleConns: 40,
		Debug:        debug,
		Auth: ch.Auth{
			Username: user,
			Password: password,
		},
	})
}
