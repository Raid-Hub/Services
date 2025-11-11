package global_metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	StatusDimension    = "status"
	OperationDimension = "operation"
)

var GetPostGameCarnageReportRequest = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "get_pgcr_req",
		Buckets: []float64{10, 20, 50, 100, 150, 200, 250, 300, 500, 750, 1000, 1500, 2000, 5000},
	},
	[]string{StatusDimension},
)

var PGCRCrawlLag = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "pgcr_crawl_lag",
		Buckets: []float64{5, 15, 25, 30, 35, 40, 45, 60, 90, 300, 1800, 14400, 86400},
	},
	[]string{StatusDimension},
)

// Instance storage operation metrics
var InstanceStorageOperations = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "instance_storage_operations_total",
		Help: "Total number of instance storage operations by type and status",
	},
	[]string{OperationDimension, StatusDimension}, // operation: "store_pgcr", "store_to_clickhouse", etc.; status: "success", "error"
)

var InstanceStorageOperationDuration = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "instance_storage_operation_duration_seconds",
		Help:    "Duration of instance storage operations in seconds",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	},
	[]string{OperationDimension, StatusDimension},
)

// Publishing operation metrics
var PublishingOperations = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "publishing_operations_total",
		Help: "Total number of publishing operations by queue and status",
	},
	[]string{"queue_name", StatusDimension}, // status: "success", "error"
)

// RegisterGlobalMetrics registers all metrics that can be exported by any app
func RegisterGlobalMetrics() {
	prometheus.MustRegister(GetPostGameCarnageReportRequest)
	prometheus.MustRegister(PGCRCrawlLag)
	prometheus.MustRegister(InstanceStorageOperations)
	prometheus.MustRegister(InstanceStorageOperationDuration)
	prometheus.MustRegister(PublishingOperations)
}
