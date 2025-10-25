package main

import "github.com/rabbitmq/amqp091-go"

type ConsumerConfig struct {
	LatestId       int64
	OffloadChannel chan int64
	RabbitChannel  *amqp091.Channel
}

type WorkerResult struct {
	Lag       []float64
	NotFounds int
}
