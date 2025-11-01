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
	searchPath := "public,core,definitions,clan,flagging,leaderboard,extended,raw,cache"
	initDone = singleton.InitAsync("POSTGRES", 5, map[string]any{
		"host":        env.PostgresHost,
		"port":        env.PostgresPort,
		"db":          env.PostgresDB,
		"search_path": searchPath,
	}, func() error {
		connStr := fmt.Sprintf("host=%s port=%s user=%s dbname=%s password=%s sslmode=disable search_path=%s",
			env.PostgresHost, env.PostgresPort, env.PostgresUser, env.PostgresDB, env.PostgresPassword, searchPath)
		db, err := sql.Open("postgres", connStr)
		if err != nil {
			return err
		}
		// Validate the connection with a ping
		if err := db.Ping(); err != nil {
			db.Close()
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
