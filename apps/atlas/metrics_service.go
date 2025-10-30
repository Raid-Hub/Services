package main

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"raidhub/lib/env"
	"raidhub/lib/services/prometheus_api"
	"raidhub/lib/utils/logging"
)

var metricsLogger = logging.NewLogger("ATLAS::METRICS_SERVICE")

var prometheusClient *prometheus_api.PrometheusClient

func init() {
	prometheusClient = prometheus_api.NewPrometheusClient(env.PrometheusPort)
}

// AtlasMetrics contains all metrics needed for scaling decisions
type AtlasMetrics struct {
	P20Lag        float64 // 20th percentile lag (how far behind head we are)
	Fraction404   float64
	ErrorFraction float64
	PGCRRate      float64
	Count404      float64
}

// GetMetricsForScaling fetches all metrics needed for scaling decisions
// intervalMinutes is automatically calculated from elapsedTime
func GetMetricsForScaling(elapsedTime time.Duration) (*AtlasMetrics, error) {
	intervalMinutes := min(4, int(elapsedTime.Minutes()))
	return GetMetrics(intervalMinutes)
}

// GetMetrics fetches all metrics for the given interval
func GetMetrics(intervalMinutes int) (*AtlasMetrics, error) {
	metrics := &AtlasMetrics{}

	// Fetch all metrics in parallel would be ideal, but for now we'll do sequentially
	// to keep it simple and avoid complex error handling

	p20Lag, err := getP20Lag(intervalMinutes)
	if err != nil {
		return nil, fmt.Errorf("failed to get p20 lag: %w", err)
	}
	metrics.P20Lag = p20Lag

	fraction404, err := get404Fraction(intervalMinutes)
	if err != nil {
		return nil, fmt.Errorf("failed to get 404 fraction: %w", err)
	}
	metrics.Fraction404 = fraction404

	errorFraction, err := getErrorFraction(intervalMinutes)
	if err != nil {
		return nil, fmt.Errorf("failed to get error fraction: %w", err)
	}
	metrics.ErrorFraction = errorFraction

	pgcrRate, err := getPgcrsPerSecond(intervalMinutes)
	if err != nil {
		return nil, fmt.Errorf("failed to get PGCR rate: %w", err)
	}
	metrics.PGCRRate = pgcrRate

	count404, err := get404Rate(intervalMinutes)
	if err != nil {
		return nil, fmt.Errorf("failed to get 404 count: %w", err)
	}
	metrics.Count404 = count404

	return metrics, nil
}

func get404Fraction(intervalMins int) (float64, error) {
	query := fmt.Sprintf(`sum(rate(pgcr_crawl_summary_status{status="3"}[%dm])) / sum(rate(pgcr_crawl_summary_status{}[%dm]))`, intervalMins, intervalMins)
	f, err := execWeightedQuery(query, intervalMins)
	if err != nil || f == -1 {
		return 0, err
	}
	return f, nil
}

func getErrorFraction(intervalMins int) (float64, error) {
	query := fmt.Sprintf(`sum(rate(pgcr_crawl_summary_status{status=~"6|7|8|9|10"}[%dm])) / sum(rate(pgcr_crawl_summary_status{}[%dm]))`, intervalMins, intervalMins)
	f, err := execWeightedQuery(query, intervalMins)
	if err != nil || f == -1 {
		return 0, err
	}
	return f, nil
}

func getP20Lag(intervalMins int) (float64, error) {
	query := `histogram_quantile(0.20, sum(rate(pgcr_crawl_summary_lag_bucket[2m])) by (le))`
	p20Lag, err := execWeightedQuery(query, intervalMins)
	if err != nil {
		return 0, err
	}
	if p20Lag == -1 {
		p20Lag = 900 // Default fallback
	}
	return p20Lag, nil
}

func get404Rate(intervalMins int) (float64, error) {
	query := fmt.Sprintf(`sum(rate(pgcr_crawl_summary_status{status="3"}[%dm])) * %d * 60`, intervalMins, intervalMins)
	res, err := prometheusClient.QueryRange(query, 0)
	if err != nil {
		return 0, err
	}
	if len(res.Data.Result) == 0 {
		return 0, nil
	}
	return strconv.ParseFloat(res.Data.Result[0].Values[0][1].(string), 64)
}

func getPgcrsPerSecond(intervalMins int) (float64, error) {
	query := fmt.Sprintf(`sum(rate(pgcr_crawl_summary_status{status=~"1|2"}[%dm]))`, intervalMins)
	f, err := execWeightedQuery(query, intervalMins)
	if err != nil || f == -1 {
		return 0, err
	}
	return f, nil
}

// execWeightedQuery executes a Prometheus query and calculates a weighted average
func execWeightedQuery(query string, intervalMins int) (float64, error) {
	res, err := prometheusClient.QueryRange(query, intervalMins)
	if err != nil {
		return 0, err
	}

	if len(res.Data.Result) == 0 {
		return -1, nil
	}

	// Creates a weighted average over the interval
	c := 0
	sum := 0.0

	for idx, y := range res.Data.Result[0].Values {
		val, err := strconv.ParseFloat(y[1].(string), 64)
		if err != nil {
			metricsLogger.Error("PROMETHEUS_VALUE_PARSE_FAILED", map[string]any{
				logging.ERROR: err.Error(),
				"value":       y[1],
			})
			return 0, err
		}
		if math.IsNaN(val) {
			continue
		}
		weight := idx + 1
		c += weight
		sum += float64(weight) * val
	}

	if c == 0 {
		return -1, nil
	}

	return sum / float64(c), nil
}
