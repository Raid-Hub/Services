package global_metrics

import (
	"github.com/prometheus/client_golang/prometheus"
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

var PGCRCrawlLag = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "pgcr_crawl_lag",
		Buckets: []float64{5, 15, 25, 30, 35, 40, 45, 60, 90, 300, 1800, 14400, 86400},
	},
	[]string{"status"},
)

// RegisterGlobalMetrics registers all metrics that can be exported by any app
func RegisterGlobalMetrics() {
	prometheus.MustRegister(GetPostGameCarnageReportRequest)
	prometheus.MustRegister(PGCRStoreActivity)
}
