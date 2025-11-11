package zeus_metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	ZEUS_ENDPOINT_TYPE_DIMENSION = "endpoint_type"
	ZEUS_STATUS_DIMENSION        = "status"
)

var RequestCount = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "zeus_requests_total",
		Help: "Total number of HTTP requests proxied by Zeus",
	},
	[]string{ZEUS_ENDPOINT_TYPE_DIMENSION, ZEUS_STATUS_DIMENSION}, // endpoint_type: "stats", "www"; status: HTTP status code as string
)

var RequestDuration = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "zeus_request_duration_ms",
		Help:    "Time taken to proxy HTTP requests in milliseconds",
		Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000},
	},
	[]string{ZEUS_ENDPOINT_TYPE_DIMENSION},
)

var RateLimiterWaitTime = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "zeus_rate_limiter_wait_ms",
		Help:    "Time spent waiting on rate limiters in milliseconds",
		Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000},
	},
	[]string{ZEUS_ENDPOINT_TYPE_DIMENSION},
)

// Register registers all Zeus-specific metrics with Prometheus
func Register() {
	prometheus.MustRegister(RequestCount)
	prometheus.MustRegister(RequestDuration)
	prometheus.MustRegister(RateLimiterWaitTime)
}
