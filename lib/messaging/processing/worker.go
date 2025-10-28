package processing

import (
	"context"

	"raidhub/lib/utils"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Worker represents a message processing worker with structured logging
type Worker struct {
	// Public fields (accessed by processors)
	ID        int
	QueueName string
	Config    TopicManagerConfig
	logger    utils.Logger

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
				// NACK with requeue=false to prevent infinite retries on permanent failures
				// The message will be sent to the dead letter queue if configured
				if err := msg.Nack(false, false); err != nil {
					panic(err)
				}
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

func (w *Worker) Info(v ...any) {
	w.logger.Info(v...)
}

func (w *Worker) Warn(v ...any) {
	w.logger.Warn(v...)
}

func (w *Worker) Error(v ...any) {
	w.logger.Error(v...)
}

func (w *Worker) Debug(v ...any) {
	w.logger.Debug(v...)
}

func (w *Worker) InfoF(format string, args ...any) {
	w.logger.InfoF(format, args...)
}

func (w *Worker) WarnF(format string, args ...any) {
	w.logger.WarnF(format, args...)
}

func (w *Worker) ErrorF(format string, args ...any) {
	w.logger.ErrorF(format, args...)
}

func (w *Worker) DebugF(format string, args ...any) {
	w.logger.DebugF(format, args...)
}
