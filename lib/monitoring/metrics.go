package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Track the number of active workers
var ActiveWorkers = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "atlas_active_workers",
	},
)

var PGCRCrawlStatus = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "pgcr_crawl_summary_status",
	},
	[]string{"status", "attempts"},
)

var PGCRCrawlLag = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "pgcr_crawl_summary_lag",
		Buckets: []float64{5, 15, 25, 30, 35, 40, 45, 60, 90, 300, 1800, 14400, 86400},
	},
	[]string{"status", "attempts"},
)

var GetPostGameCarnageReportRequest = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "get_pgcr_req",
		Buckets: []float64{10, 20, 50, 100, 150, 200, 250, 300, 500, 750, 1000, 1500, 2000, 5000},
	},
	[]string{"status"},
)

var PGCRStoreActivity = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "pgcr_instance_type",
	},
	[]string{"activity_name", "version_name", "completed"},
)

var FloodgatesRecent = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "floodgates_recent_pgcr",
	},
)

// Queue worker metrics
var QueueWorkerCount = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "queue_worker_count",
		Help: "Current number of active workers per queue topic",
	},
	[]string{"queue_name"},
)

var QueueDepth = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "queue_depth",
		Help: "Current queue depth (number of messages)",
	},
	[]string{"queue_name"},
)

var QueueMessagesProcessed = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "queue_messages_processed_total",
		Help: "Total number of messages processed by queue workers",
	},
	[]string{"queue_name", "status"}, // status: "success", "error"
)

var QueueMessageProcessingDuration = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "queue_message_processing_duration_seconds",
		Help:    "Time taken to process a message",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
	},
	[]string{"queue_name"},
)

var QueueScalingDecisions = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "queue_scaling_decisions_total",
		Help: "Total number of scaling decisions made",
	},
	[]string{"queue_name", "direction"}, // direction: "up", "down"
)
