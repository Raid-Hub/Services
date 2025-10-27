package clickhouse

import (
	"log"
	"time"

	"raidhub/lib/env"

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
	host := env.ClickHouseHost
	if host == "" {
		host = "localhost"
	}

	// First connect to default database to create our custom database
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{host + ":" + env.ClickHousePort},
		Auth: clickhouse.Auth{
			Database: env.ClickHouseDB,
			Username: env.ClickHouseUser,
			Password: env.ClickHousePassword,
		},
	})
	if err != nil {
		return nil, err
	}

	return conn, nil
}
