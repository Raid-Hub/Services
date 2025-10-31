package hermes_metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const QUEUE_NAME_DIMENSION = "queue_name"

// Queue worker metrics
var QueueWorkerCount = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "queue_worker_count",
		Help: "Current number of active workers per queue topic",
	},
	[]string{QUEUE_NAME_DIMENSION},
)

var QueueDepth = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "queue_depth",
		Help: "Current queue depth (number of messages)",
	},
	[]string{QUEUE_NAME_DIMENSION},
)

var QueueMessagesProcessed = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "queue_messages_processed_total",
		Help: "Total number of messages processed by queue workers",
	},
	[]string{QUEUE_NAME_DIMENSION, "status"}, // status: "success", "error"
)

var QueueMessageProcessingDuration = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "queue_message_processing_duration_seconds",
		Help:    "Time taken to process a message",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
	},
	[]string{QUEUE_NAME_DIMENSION},
)

var QueueScalingDecisions = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "queue_scaling_decisions_total",
		Help: "Total number of scaling decisions made",
	},
	[]string{QUEUE_NAME_DIMENSION, "direction"}, // direction: "up", "down"
)

var FloodgatesRecent = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "floodgates_recent_pgcr",
	},
)

// Register registers all Hermes-specific metrics with Prometheus
func Register() {
	prometheus.MustRegister(QueueWorkerCount)
	prometheus.MustRegister(QueueDepth)
	prometheus.MustRegister(QueueMessagesProcessed)
	prometheus.MustRegister(QueueMessageProcessingDuration)
	prometheus.MustRegister(QueueScalingDecisions)
	prometheus.MustRegister(FloodgatesRecent)
}
