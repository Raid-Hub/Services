package processing

import (
	"time"
)

// TopicConfig defines the configuration for a topic
type TopicConfig struct {
	QueueName             string
	MinWorkers            int
	MaxWorkers            int
	DesiredWorkers        int
	ContestWeekendWorkers int
	KeepInReady           bool
	PrefetchCount         int
	ScaleUpThreshold      int
	ScaleDownThreshold    int
	ScaleUpPercent        float64
	ScaleDownPercent      float64
	ScaleCheckInterval    time.Duration
}

// Topic represents a queue processing topic with self-scaling
type Topic struct {
	Config    TopicConfig
	processor ProcessorFunc
}

// NewTopic creates a new topic with the given config and processor
func NewTopic(config TopicConfig, processor ProcessorFunc) Topic {
	return Topic{
		Config:    config,
		processor: processor,
	}
}
