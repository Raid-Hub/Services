package queueworkers

import "time"

// HermesOutboundHTTPRetryDelay is the standard backoff for workers that only do outbound HTTP
// (e.g. subscription webhooks, Discord linked-role PUT). Matches prior per-topic copies:
// attempts 1–10: 5s; 11–15: 12s step to 60s; 16+: 30m.
func HermesOutboundHTTPRetryDelay(newRetryCount int) time.Duration {
	switch {
	case newRetryCount <= 10:
		return 5 * time.Second
	case newRetryCount <= 15:
		step := newRetryCount - 11
		return time.Duration(12+step*12) * time.Second
	default:
		return 30 * time.Minute
	}
}
