package clickhouse

import (
	"log"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	ch "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

var DB ch.Conn

func init() {
	// Retry connection with backoff
	maxRetries := 10
	var err error
	for i := 0; i < maxRetries; i++ {
		DB, err = connect()
		if err == nil {
			log.Printf("ClickHouse connected")
			return
		}
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	log.Fatalf("Failed to connect to ClickHouse after %d retries: %v", maxRetries, err)
}

func connect() (ch.Conn, error) {
	return clickhouse.Open(&clickhouse.Options{
		Addr: []string{"localhost:9000"},
		Auth: clickhouse.Auth{
			Database: "default",
			Username: "default",
			Password: "",
		},
	})
}
