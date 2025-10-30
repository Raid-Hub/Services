package postgres

import (
	"database/sql"
	"fmt"
	"raidhub/lib/env"
	"raidhub/lib/utils/logging"
	"raidhub/lib/utils/singleton"

	_ "github.com/lib/pq"
)

var (
	DB       *sql.DB
	logger   = logging.NewLogger("POSTGRES")
	initDone <-chan struct{}
)

func init() {
	initDone = singleton.InitAsync("POSTGRES", 10, func() error {
		searchPath := "public,core,definitions,clan,flagging,leaderboard,extended,raw"
		connStr := fmt.Sprintf("user=%s dbname=%s password=%s sslmode=disable search_path=%s",
			env.PostgresUser, env.PostgresDB, env.PostgresPassword, searchPath)
		db, err := sql.Open("postgres", connStr)
		if err != nil {
			return err
		}
		DB = db
		return nil
	})
}

// Wait blocks until PostgreSQL initialization is complete
func Wait() {
	<-initDone
}
