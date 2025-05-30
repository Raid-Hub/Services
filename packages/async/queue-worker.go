package async

import (
	"database/sql"
	"log"
	"raidhub/packages/util"

	amqp "github.com/rabbitmq/amqp091-go"
)

type QueueWorker struct {
	QueueName string
	Conn      *amqp.Connection
	Db        *sql.DB
	Processer func(qw *QueueWorker, msg amqp.Delivery)
	Wg        *util.ReadOnlyWaitGroup
}

func (qw *QueueWorker) Register(numWorkers int, keepInReady bool) {
	ch, err := qw.Conn.Channel()
	if err != nil {
		log.Fatalf("Failed to create channel: %s", err)
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		qw.QueueName,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Failed to create queue: %s", err)
	}

	if keepInReady {
		err = ch.Qos(1, 0, false)
		if err != nil {
			log.Fatalf("Failed to set QoS: %s", err)
		}
	}
	for i := 0; i < numWorkers; i++ {
		msgs, err := ch.Consume(
			q.Name,
			"",
			false,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			log.Fatalf("Failed to register a consumer: %s", err)
		}
		go func() {
			for msg := range msgs {
				qw.Processer(qw, msg)
			}
		}()
	}

	log.Printf("Waiting for messages on queue %s...", qw.QueueName)
	forever := make(chan bool)
	<-forever
}
