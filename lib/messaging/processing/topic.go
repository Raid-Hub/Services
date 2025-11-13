package processing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// UnretryableError is an error that indicates the message should NOT be requeued
// This is used for permanent failures that will not succeed on retry
// By default, errors are treated as retryable (transient), so only mark permanent failures as UnretryableError
type UnretryableError struct {
	Err error
}

func (e *UnretryableError) Error() string {
	return e.Err.Error()
}

func (e *UnretryableError) Unwrap() error {
	return e.Err
}

// NewUnretryableError wraps an error to indicate it should NOT be retried
func NewUnretryableError(err error) *UnretryableError {
	return &UnretryableError{Err: err}
}

// IsUnretryableError checks if an error is an UnretryableError
// Uses errors.As to properly handle wrapped errors (e.g., fmt.Errorf with %w)
func IsUnretryableError(err error) bool {
	var unretryableErr *UnretryableError
	return errors.As(err, &unretryableErr)
}

// ProcessorFunc defines the function signature for processing messages
type ProcessorFunc func(worker WorkerInterface, message amqp.Delivery) error

// Topic represents a queue processing topic with self-scaling
type Topic struct {
	Config    TopicConfig
	Processor ProcessorFunc // Exported so apps/hermes can access it
}

// WorkerInterface provides a minimal interface for processors to interact with workers
type WorkerInterface interface {
	Info(key string, fields map[string]any)
	Warn(key string, err error, fields map[string]any)
	Error(key string, err error, fields map[string]any)
	Debug(key string, fields map[string]any)
	Context() context.Context
}

// ParseJSON parses JSON from message body using a generic type parameter and logs errors automatically
func ParseJSON[T any](worker WorkerInterface, data []byte) (T, error) {
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		worker.Error("FAILED_TO_UNMARSHAL_MESSAGE", err, nil)
		return v, err
	}
	return v, nil
}

// ParseText parses text from message body and logs errors automatically
func ParseText(worker WorkerInterface, data []byte) (string, error) {
	text := string(data)
	if text == "" {
		err := fmt.Errorf("empty message body")
		worker.Error("FAILED_TO_PARSE_TEXT", err, nil)
		return "", err
	}
	return text, nil
}

// ParseInt64 parses int64 from message body and logs errors automatically
func ParseInt64(worker WorkerInterface, data []byte) (int64, error) {
	text := string(data)
	if text == "" {
		err := fmt.Errorf("empty message body")
		worker.Error("FAILED_TO_PARSE_INT64", err, nil)
		return 0, err
	}
	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		worker.Error("FAILED_TO_PARSE_INT64", err, map[string]any{
			"body": text,
		})
		return 0, err
	}
	return value, nil
}

// TopicConfig defines the configuration for a topic
type TopicConfig struct {
	QueueName             string
	MinWorkers            int
	MaxWorkers            int
	DesiredWorkers        int
	ContestWeekendWorkers int
	KeepInReady           bool // Messages are not pre-consumed by workers before processing
	PrefetchCount         int
	ScaleUpThreshold      int
	ScaleDownThreshold    int
	ScaleUpPercent        float64
	ScaleDownPercent      float64
	ScaleCheckInterval    time.Duration
	ScaleCooldown         time.Duration // Minimum time between scaling decisions
	MaxWorkersPerStep     int           // Maximum workers to add/remove per scaling action
	MinWorkersPerStep     int           // Minimum workers to add/remove per scaling action
	ConsecutiveChecksUp   int           // Consecutive checks above threshold before scaling up
	ConsecutiveChecksDown int           // Consecutive checks below threshold before scaling down
	BungieSystemDeps      []string      // Which API systems must be available for the topic to scale
}

// NewTopic creates a new topic with the given config and processor
func NewTopic(config TopicConfig, processor ProcessorFunc) Topic {
	return Topic{
		Config:    config,
		Processor: processor,
	}
}
