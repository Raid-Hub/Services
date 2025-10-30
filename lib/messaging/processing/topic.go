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

	// New scaling parameters for improved accuracy
	ScaleUpHysteresis     int           // Hysteresis: scale-down threshold (typically lower than ScaleDownThreshold)
	ScaleDownHysteresis   int           // Hysteresis: scale-up threshold (typically higher than ScaleUpThreshold)
	ScaleCooldown         time.Duration // Minimum time between scaling decisions
	MaxWorkersPerStep     int           // Maximum workers to add/remove per scaling action
	MinWorkersPerStep     int           // Minimum workers to add/remove per scaling action
	ConsecutiveChecksUp   int           // Consecutive checks above threshold before scaling up
	ConsecutiveChecksDown int           // Consecutive checks below threshold before scaling down
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
