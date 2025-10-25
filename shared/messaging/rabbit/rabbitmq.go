package rabbit

import (
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	conn *amqp.Connection
	once sync.Once
)

func Init() (*amqp.Connection, error) {
	var err error = nil
	once.Do(func() {
		conn, err = initOnce()
	})
	return conn, err
}

func initOnce() (*amqp.Connection, error) {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	// Set up RabbitMQ connection
	username := os.Getenv("RABBITMQ_USER")
	password := os.Getenv("RABBITMQ_PASSWORD")
	port := os.Getenv("RABBITMQ_PORT")

	if username == "" || password == "" || port == "" {
		log.Fatalf("Environment variables RABBITMQ_USER, RABBITMQ_PASSWORD, or RABBITMQ_PORT are not set")
	}

	rabbitURL := fmt.Sprintf("amqp://%s:%s@localhost:%s/", username, password, port)

	return amqp.Dial(rabbitURL)
}

func Cleanup() {
	// Clean up the resources when the program exits
	if conn != nil {
		conn.Close()
	}
}
