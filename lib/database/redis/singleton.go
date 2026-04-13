package redis

import (
	"context"
	"raidhub/lib/env"
	"raidhub/lib/utils/singleton"

	"github.com/redis/go-redis/v9"
)

var (
	Client   *redis.Client
	initDone <-chan struct{}
)

func init() {
	initDone = singleton.InitAsync("REDIS", 10, map[string]any{
		"host": env.RedisHost,
		"port": env.RedisPort,
	}, func() error {
		opts := &redis.Options{
			Addr: env.RedisHost + ":" + env.RedisPort,
		}
		if env.RedisPassword != "" {
			opts.Password = env.RedisPassword
		}
		client := redis.NewClient(opts)
		if err := client.Ping(context.Background()).Err(); err != nil {
			return err
		}
		Client = client
		return nil
	})
}

// Wait blocks until Redis initialization is complete.
func Wait() {
	<-initDone
}
