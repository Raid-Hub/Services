package main

import (
	"context"
	"errors"
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
	done        chan struct{}  // Channel that closes when worker is finished
	currentMsg  *amqp.Delivery // Current message being processed (for retry count tracking)
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
				err := fmt.Errorf("channel_closed")
				w.Error(WORKER_STOPPING, err, nil)
				return
			}

			w.handleMessage(msg)
		}
	}
}

func (w *Worker) handleMessage(msg amqp.Delivery) {
	// Wait for API availability if needed
	if w.wg != nil {
		w.wg.Wait()
	}

	err := w.ProcessMessage(msg)
	if err != nil {
		retryCount := w.getRetryCount(msg)

		// Check if we've exceeded max retry count
		if w.shouldDeadLetter(retryCount) {
			w.dropMessage(msg, retryCount, w.Topic.Config.MaxRetryCount, err)
			return
		}

		// Extract original error details for logging
		fields := make(map[string]any)
		if originalErr := errors.Unwrap(err); originalErr != nil {
			fields["original_error"] = originalErr.Error()
		}
		w.Warn("MESSAGE_PROCESSING_ERROR", err, fields)
		// Record metrics for failed processing
		hermes_metrics.QueueMessagesProcessed.WithLabelValues(w.QueueName, "error").Inc()

		// Handle based on error type
		if processing.IsUnretryableError(err) {
			w.logUnretryableMessage(msg, err)
			w.handleUnretryableError(msg)
		} else {
			// Increment retry count and republish for retryable errors
			newRetryCount := retryCount + 1
			if err := w.republishForRetry(msg, newRetryCount); err != nil {
				w.Fatal("MESSAGE_REPUBLISH_ERROR", err, nil)
			}

			// Ack the original message since we've republished it
			if err := msg.Ack(false); err != nil {
				w.Fatal("MESSAGE_ACK_ERROR", err, nil)
			}
		}
		return
	}

	// Ack successful processing
	if err := msg.Ack(false); err != nil {
		w.Warn("MESSAGE_ACK_ERROR", err, nil)
	} else {
		// Record metrics for successful processing
		hermes_metrics.QueueMessagesProcessed.WithLabelValues(w.QueueName, "success").Inc()
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
	w.cancel(errors.New(AUTOSCALE_IN))
}

func (w *Worker) ProcessMessage(message amqp.Delivery) error {
	// Store current message for retry count tracking in logs
	w.currentMsg = &message
	defer func() { w.currentMsg = nil }()

	w.Debug("PROCESSING_MESSAGE_STARTED", nil)

	startTime := time.Now()
	err := w.processor(w, message)
	duration := time.Since(startTime)

	// Record processing duration metric
	hermes_metrics.QueueMessageProcessingDuration.WithLabelValues(w.QueueName).Observe(duration.Seconds())

	return err
}

func (w *Worker) getRetryCount(msg amqp.Delivery) int {
	if count, ok := msg.Headers["x-retry-count"].(int32); ok && count > 0 {
		return int(count)
	} else if count, ok := msg.Headers["x-retry-count"].(int64); ok && count > 0 {
		return int(count)
	}
	return 0
}

func (w *Worker) addWorkerFields(fields map[string]any) map[string]any {
	if fields == nil {
		fields = make(map[string]any)
	}
	// Note: queue is set as Sentry tag in CaptureError when field key is "$queue"
	fields["$queue"] = w.QueueName
	fields["worker_id"] = w.ID

	// Add retry count if we're currently processing a message
	if w.currentMsg != nil {
		fields["retry_count"] = w.getRetryCount(*w.currentMsg)
	}

	return fields
}

func (w *Worker) Debug(key string, fields map[string]any) {
	w.logger.Debug(key, w.addWorkerFields(fields))
}

func (w *Worker) Info(key string, fields map[string]any) {
	w.logger.Info(key, w.addWorkerFields(fields))
}

func (w *Worker) Warn(key string, err error, fields map[string]any) {
	fields = w.addWorkerFields(fields)
	w.logger.Warn(key, err, fields)
}

func (w *Worker) Error(key string, err error, fields map[string]any) {
	fields = w.addWorkerFields(fields)
	w.logger.Error(key, err, fields)
}

func (w *Worker) Fatal(key string, err error, fields map[string]any) {
	fields = w.addWorkerFields(fields)
	w.logger.Fatal(key, err, fields)
}

// shouldDeadLetter checks if message should be dead-lettered based on retry count
func (w *Worker) shouldDeadLetter(retryCount int) bool {
	maxRetries := w.Topic.Config.MaxRetryCount
	return maxRetries > 0 && retryCount >= maxRetries
}

// dropMessage permanently drops messages that exceed retry limits
func (w *Worker) dropMessage(msg amqp.Delivery, retryCount int, maxRetries int, processingErr error) {
	fields := map[string]any{
		"retry_count":     retryCount,
		"max_retries":     maxRetries,
		"action":          "dropping_message",
		"routing_key":     msg.RoutingKey,
		"delivery_tag":    msg.DeliveryTag,
		"unretryable_err": processing.IsUnretryableError(processingErr),
	}
	if msg.MessageId != "" {
		fields["message_id"] = msg.MessageId
	}
	if msg.Exchange != "" {
		fields["exchange"] = msg.Exchange
	}
	w.Error("MESSAGE_EXCEEDED_MAX_RETRIES", processingErr, fields)

	// Nack with requeue=false to permanently drop the message
	// This prevents infinite retry loops
	if err := msg.Nack(false, false); err != nil {
		w.Fatal("MESSAGE_NACK_ERROR", err, nil)
	}
}

// republishForRetry republishes message with incremented retry count
func (w *Worker) republishForRetry(msg amqp.Delivery, newRetryCount int) error {
	if msg.Headers == nil {
		msg.Headers = amqp.Table{}
	}
	msg.Headers["x-retry-count"] = int32(newRetryCount)

	// Republish the message to the same queue with updated headers
	return w.amqpChannel.Publish(
		msg.Exchange,   // exchange
		msg.RoutingKey, // routing key (queue name)
		false,          // mandatory
		false,          // immediate
		amqp.Publishing{
			Headers:      msg.Headers,
			ContentType:  msg.ContentType,
			Body:         msg.Body,
			DeliveryMode: msg.DeliveryMode,
			Priority:     msg.Priority,
		},
	)
}

// handleUnretryableError handles permanent processing failures
func (w *Worker) handleUnretryableError(msg amqp.Delivery) {
	// NACK with requeue=false for unretryable errors (permanent failures)
	// The message will be sent to the dead letter queue if configured
	if err := msg.Nack(false, false); err != nil {
		w.Fatal("MESSAGE_NACK_ERROR", err, nil)
	}
}

func (w *Worker) logUnretryableMessage(msg amqp.Delivery, err error) {
	fields := map[string]any{
		"routing_key":  msg.RoutingKey,
		"delivery_tag": msg.DeliveryTag,
		logging.REASON: "processor_marked_unretryable",
	}
	if msg.MessageId != "" {
		fields["message_id"] = msg.MessageId
	}
	if msg.Exchange != "" {
		fields["exchange"] = msg.Exchange
	}
	// Extract original error details for logging
	if originalErr := errors.Unwrap(err); originalErr != nil {
		fields["original_error"] = originalErr.Error()
	}
	w.Error("MESSAGE_UNRETRYABLE", err, fields)
}
