package processing

import (
	"context"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Worker represents a message processing worker with structured logging
type Worker struct {
	// Public fields (accessed by processors)
	ID        int
	QueueName string
	Config    TopicManagerConfig

	// Private fields (internal worker state)
	ctx       context.Context
	channel   <-chan amqp.Delivery
	processor ProcessorFunc
}

// ProcessorFunc defines the function signature for processing messages
type ProcessorFunc func(worker *Worker, message amqp.Delivery) error

// Run starts the worker loop and processes messages until the context is cancelled
func (w *Worker) Run() {
	for {
		select {
		case msg, ok := <-w.channel:
			if !ok {
				return
			}

			// Wait for API availability if needed
			if w.Config.Wg != nil {
				w.Config.Wg.Wait()
			}

			err := w.ProcessMessage(msg)
			if err != nil {
				w.Error("Error processing message", "error", err)
				// Don't ack on error - let message retry
				continue
			}

			// Ack successful processing
			if err := msg.Ack(false); err != nil {
				w.Error("Failed to acknowledge message", "error", err)
			}

		case <-w.ctx.Done():
			w.Info("Worker stopping (context cancelled)")
			return
		}
	}
}

func (w *Worker) ProcessMessage(message amqp.Delivery) error {
	return w.processor(w, message)
}

// Info logs an informational message
func (w *Worker) Info(msg string, v ...any) {
	log.Printf("[%s][%d] INFO: %s %v", w.QueueName, w.ID, msg, v)
}

// Warn logs a warning message
func (w *Worker) Warn(msg string, v ...any) {
	log.Printf("[%s][%d] WARN: %s %v", w.QueueName, w.ID, msg, v)
}

// Error logs an error message
func (w *Worker) Error(msg string, v ...any) {
	log.Printf("[%s][%d] ERROR: %s %v", w.QueueName, w.ID, msg, v)
}
