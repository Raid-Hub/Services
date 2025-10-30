package monitoring

import (
	"fmt"
	"net/http"
	"raidhub/lib/env"
	"raidhub/lib/utils/logging"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var logger = logging.NewLogger("MONITORING")

func serveMetrics(port int) {
	http.Handle("/metrics", promhttp.Handler())

	go func() {
		port := fmt.Sprintf(":%d", port)
		if err := http.ListenAndServe(port, nil); err != nil {
			logger.Fatal("PROMETHEUS_SERVER_ERROR", map[string]any{
				logging.ERROR: err.Error(),
			})
		}
	}()
}

func loadMetricsPort(port string) (int, bool) {
	if metricsPort, err := strconv.Atoi(port); err == nil {
		return metricsPort, true
	} else {
		logger.Warn("INVALID_METRICS_PORT", map[string]any{
			logging.ERROR: err.Error(),
			"port":        port,
		})
		return 0, false
	}
}

// Track the count of each Bungie error code returned by the API
func RegisterAtlasMetrics() {

	prometheus.MustRegister(ActiveWorkers)
	prometheus.MustRegister(PGCRCrawlLag)
	prometheus.MustRegister(PGCRCrawlStatus)
	prometheus.MustRegister(GetPostGameCarnageReportRequest)
	prometheus.MustRegister(PGCRStoreActivity)

	if metricsPort, ok := loadMetricsPort(env.AtlasMetricsPort); ok {
		serveMetrics(metricsPort)
	}

}

func RegisterHermesMetrics() {
	prometheus.MustRegister(FloodgatesRecent)
	prometheus.MustRegister(PGCRCrawlLag)
	prometheus.MustRegister(PGCRStoreActivity)
	prometheus.MustRegister(GetPostGameCarnageReportRequest)

	// Queue worker metrics
	prometheus.MustRegister(QueueWorkerCount)
	prometheus.MustRegister(QueueDepth)
	prometheus.MustRegister(QueueMessagesProcessed)
	prometheus.MustRegister(QueueMessageProcessingDuration)
	prometheus.MustRegister(QueueScalingDecisions)

	if metricsPort, ok := loadMetricsPort(env.HermesMetricsPort); ok {
		serveMetrics(metricsPort)
	}
}
