package monitoring

import (
	"fmt"
	"net/http"
	"raidhub/lib/env"
	"raidhub/lib/monitoring/atlas_metrics"
	"raidhub/lib/monitoring/global_metrics"
	"raidhub/lib/monitoring/hermes_metrics"
	"raidhub/lib/monitoring/zeus_metrics"
	"raidhub/lib/utils/logging"
	"strconv"

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

// RegisterAtlasMetrics registers Atlas-specific metrics and starts the metrics server
func RegisterAtlasMetrics() {
	global_metrics.RegisterGlobalMetrics()
	atlas_metrics.Register()

	if metricsPort, ok := loadMetricsPort(env.AtlasMetricsPort); ok {
		serveMetrics(metricsPort)
	}
}

// RegisterHermesMetrics registers Hermes-specific metrics and starts the metrics server
func RegisterHermesMetrics(portOverride string) {
	global_metrics.RegisterGlobalMetrics()
	hermes_metrics.Register()

	// Use override port if provided, otherwise use env var
	port := portOverride
	if port == "" {
		port = env.HermesMetricsPort
	}

	if metricsPort, ok := loadMetricsPort(port); ok {
		serveMetrics(metricsPort)
	}
}

// RegisterZeusMetrics registers Zeus-specific metrics and starts the metrics server
func RegisterZeusMetrics() {
	zeus_metrics.Register()

	if metricsPort, ok := loadMetricsPort(env.ZeusMetricsPort); ok {
		serveMetrics(metricsPort)
	}
}
