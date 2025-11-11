package publishing

import (
	"raidhub/lib/messaging/rabbit"
	"raidhub/lib/utils/singleton"
)

var (
	initDone <-chan struct{}
)

func init() {
	initDone = singleton.InitAsync("RABBITMQ_PUBLISHER", 3, map[string]any{}, func() error {
		// Wait for RabbitMQ connection to be ready before creating publisher channel
		rabbit.Wait()

		ch, err := rabbit.Conn.Channel()
		if err != nil {
			return err
		}
		channel = ch
		return nil
	})
}

// Wait blocks until publishing initialization is complete
func Wait() {
	<-initDone
}
