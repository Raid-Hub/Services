package processing

import (
	"context"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

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
	Warn(key string, fields map[string]any)
	Error(key string, fields map[string]any)
	Debug(key string, fields map[string]any)
	Context() context.Context
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
}

// NewTopic creates a new topic with the given config and processor
func NewTopic(config TopicConfig, processor ProcessorFunc) Topic {
	return Topic{
		Config:    config,
		Processor: processor,
	}
}
