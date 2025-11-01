package main

import (
	"context"
	"fmt"
	"time"

	"raidhub/lib/messaging/processing"
	"raidhub/lib/monitoring/hermes_metrics"
	"raidhub/lib/utils"
	"raidhub/lib/utils/logging"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	WORKER_STOPPING = "WORKER_STOPPING"
	AUTOSCALE_IN    = "autoscaled_in"
)

// Worker represents a message processing worker with structured logging
type Worker struct {
	// Public fields (accessed by processors)
	ID        int
	QueueName string
	Topic     processing.Topic
	logger    logging.Logger

	// Private fields (internal worker state)
	ctx         context.Context          // Worker context that cancels on shutdown or autoscale
	cancel      context.CancelCauseFunc  // Cancel function for worker context (with cause)
	amqpChannel *amqp.Channel            // RabbitMQ channel for this worker
	wg          *utils.ReadOnlyWaitGroup // API availability wait group
	channel     <-chan amqp.Delivery
	processor   processing.ProcessorFunc
	done        chan struct{} // Channel that closes when worker is finished
}

// Run starts the worker and kicks off the polling
func (w *Worker) Run() {
	defer close(w.done)
	defer w.amqpChannel.Close() // Clean up the RabbitMQ channel when worker stops

	for {
		select {
		case <-w.ctx.Done():
			// Log the reason for stopping based on the cause
			causeStr := "unknown"
			if cause := context.Cause(w.ctx); cause != nil {
				causeStr = cause.Error()
			}
			w.Debug(WORKER_STOPPING, map[string]any{
				logging.REASON: causeStr,
			})
			return
		case msg, ok := <-w.channel:
			if !ok {
				w.Error(WORKER_STOPPING, map[string]any{
					logging.ERROR: "channel_closed",
				})
				return
			}

			// Wait for API availability if needed
			if w.wg != nil {
				w.wg.Wait()
			}

			err := w.ProcessMessage(msg)
			if err != nil {
				w.Warn("MESSAGE_PROCESSING_ERROR", map[string]any{
					logging.ERROR: err.Error(),
				})
				// Record metrics for failed processing
				hermes_metrics.QueueMessagesProcessed.WithLabelValues(w.QueueName, "error").Inc()
				// NACK with requeue=false to prevent infinite retries on permanent failures
				// The message will be sent to the dead letter queue if configured
				if err := msg.Nack(false, false); err != nil {
					w.Fatal("MESSAGE_NACK_ERROR", map[string]any{
						logging.ERROR: err.Error(),
					})
				}
				continue
			}

			// Ack successful processing
			if err := msg.Ack(false); err != nil {
				w.Warn("MESSAGE_ACK_ERROR", map[string]any{
					logging.ERROR: err.Error(),
				})
			} else {
				// Record metrics for successful processing
				hermes_metrics.QueueMessagesProcessed.WithLabelValues(w.QueueName, "success").Inc()
			}
		}
	}
}

func (w *Worker) Done() <-chan struct{} {
	return w.done
}

func (w *Worker) Context() context.Context {
	return w.ctx
}

func (w *Worker) Wait() {
	<-w.done
}

// ScaleIn gracefully stops this worker by cancelling its context with autoscale cause
func (w *Worker) ScaleIn() {
	w.Debug(WORKER_STOPPING, map[string]any{
		logging.REASON: AUTOSCALE_IN,
	})
	// Use a specific error to distinguish autoscale from shutdown
	w.cancel(fmt.Errorf(AUTOSCALE_IN))
}

func (w *Worker) ProcessMessage(message amqp.Delivery) error {
	w.Debug("PROCESSING_MESSAGE_STARTED", nil)

	startTime := time.Now()
	err := w.processor(w, message)
	duration := time.Since(startTime)

	// Record processing duration metric
	hermes_metrics.QueueMessageProcessingDuration.WithLabelValues(w.QueueName).Observe(duration.Seconds())

	return err
}

func (w *Worker) addWorkerFields(fields map[string]any) map[string]any {
	if fields == nil {
		fields = make(map[string]any)
	}
	fields["queue"] = w.QueueName
	fields["worker_id"] = w.ID
	return fields
}

func (w *Worker) Debug(key string, fields map[string]any) {
	w.logger.Debug(key, w.addWorkerFields(fields))
}

func (w *Worker) Info(key string, fields map[string]any) {
	w.logger.Info(key, w.addWorkerFields(fields))
}

func (w *Worker) Warn(key string, fields map[string]any) {
	w.logger.Warn(key, w.addWorkerFields(fields))
}

func (w *Worker) Error(key string, fields map[string]any) {
	w.logger.Error(key, w.addWorkerFields(fields))
}

func (w *Worker) Fatal(key string, fields map[string]any) {
	w.logger.Fatal(key, w.addWorkerFields(fields))
}
