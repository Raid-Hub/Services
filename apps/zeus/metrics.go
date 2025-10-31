package main

import (
	"time"

	"raidhub/lib/monitoring"
	"raidhub/lib/monitoring/zeus_metrics"
)

// metricsEvent represents a metrics event to be processed asynchronously
type metricsEvent struct {
	endpointType    string
	duration        time.Duration
	rateLimiterWait time.Duration
}

var metricsChan = make(chan metricsEvent, 10000) // Buffered channel to avoid blocking

func metricsWorker() {
	monitoring.RegisterZeusMetrics()

	for event := range metricsChan {
		zeus_metrics.RequestCount.WithLabelValues(event.endpointType).Inc()
		zeus_metrics.RequestDuration.WithLabelValues(event.endpointType).Observe(float64(event.duration.Milliseconds()))
		zeus_metrics.RateLimiterWaitTime.WithLabelValues(event.endpointType).Observe(float64(event.rateLimiterWait.Milliseconds()))
	}
}
