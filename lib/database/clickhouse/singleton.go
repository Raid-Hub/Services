package clickhouse

import (
	"raidhub/lib/env"
	"raidhub/lib/utils/singleton"

	"github.com/ClickHouse/clickhouse-go/v2"
	ch "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

var (
	DB       ch.Conn
	initDone <-chan struct{}
)

func init() {
	initDone = singleton.InitAsync("CLICKHOUSE", 10, map[string]any{
		"host": env.ClickHouseHost,
		"port": env.ClickHousePort,
		"db":   env.ClickHouseDB,
	}, func() error {
		conn, err := clickhouse.Open(&clickhouse.Options{
			Addr: []string{env.ClickHouseHost + ":" + env.ClickHousePort},
			Auth: clickhouse.Auth{
				Database: env.ClickHouseDB,
				Username: env.ClickHouseUser,
				Password: env.ClickHousePassword,
			},
		})
		if err != nil {
			return err
		}
		DB = conn
		return nil
	})
}

// Wait blocks until ClickHouse initialization is complete
func Wait() {
	<-initDone
}
