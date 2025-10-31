package atlas_metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var ActiveWorkers = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "atlas_active_workers",
	},
)

var PGCRC_CRAWL_SUMMARY_DIMENSIONS = []string{"status", "attempts"}

var PGCRCrawlStatus = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "pgcr_crawl_summary_status",
	},
	PGCRC_CRAWL_SUMMARY_DIMENSIONS,
)

var PGCRCrawlLag = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "pgcr_crawl_summary_lag",
		Buckets: []float64{5, 15, 25, 30, 35, 40, 45, 60, 90, 300, 1800, 14400, 86400},
	},
	PGCRC_CRAWL_SUMMARY_DIMENSIONS,
)

// Register registers all Atlas-specific metrics with Prometheus
func Register() {
	prometheus.MustRegister(ActiveWorkers)
	prometheus.MustRegister(PGCRCrawlStatus)
	prometheus.MustRegister(PGCRCrawlLag)
}
