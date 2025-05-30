package monitoring

import (
	"fmt"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

func serveMetrics(port int) {
	http.Handle("/metrics", promhttp.Handler())

	go func() {
		port := fmt.Sprintf(":%d", port)
		log.Fatal(http.ListenAndServe(port, nil))
	}()
}

// Track the count of each Bungie error code returned by the API
func RegisterAtlas(port int) {
	prometheus.MustRegister(ActiveWorkers)
	prometheus.MustRegister(PGCRCrawlLag)
	prometheus.MustRegister(PGCRCrawlStatus)
	prometheus.MustRegister(GetPostGameCarnageReportRequest)
	prometheus.MustRegister(PGCRStoreActivity)

	serveMetrics(port)
}

func RegisterHermes(port int) {
	prometheus.MustRegister(FloodgatesRecent)
	prometheus.MustRegister(PGCRStoreActivity)

	serveMetrics(port)
}
