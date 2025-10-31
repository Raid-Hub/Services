package prometheus_api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"raidhub/lib/env"
	"raidhub/lib/utils/logging"
)

var clientLogger = logging.NewLogger("PROMETHEUS_API_CLIENT")

// QueryRangeResponse represents the structure of a Prometheus query_range API response
type QueryRangeResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Values [][]any `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

// PrometheusClient provides a client for querying Prometheus API
type PrometheusClient struct {
	baseURL string
	timeout time.Duration
	client  *http.Client
}

// NewPrometheusClient creates a new Prometheus API query client
// port should be just the port number (e.g., "9090") for the Prometheus API server
// Note: This is for the Prometheus API (query service), not the exporter
func NewPrometheusClient(port string) *PrometheusClient {
	baseURL := fmt.Sprintf("http://%s:%s", env.PrometheusHost, port)
	clientLogger.Info("PROMETHEUS_CLIENT_CREATED", map[string]any{
		"host": env.PrometheusHost,
		"port": port,
	})
	return &PrometheusClient{
		baseURL: baseURL,
		timeout: 10 * time.Second,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// QueryRange executes a Prometheus query_range query
func (c *PrometheusClient) QueryRange(query string, intervalMins int) (*QueryRangeResponse, error) {
	params := url.Values{}
	params.Add("query", query)
	params.Add("start", time.Now().Add(time.Duration(-intervalMins)*time.Minute).Format(time.RFC3339))
	params.Add("end", time.Now().Format(time.RFC3339))
	params.Add("step", "1m")

	queryURL := fmt.Sprintf("%s/api/v1/query_range?%s", c.baseURL, params.Encode())

	resp, err := c.client.Get(queryURL)
	if err != nil {
		clientLogger.Error("PROMETHEUS_QUERY_FAILED", map[string]any{
			logging.ENDPOINT: queryURL,
			logging.ERROR:    err.Error(),
		})
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		clientLogger.Error("PROMETHEUS_QUERY_BAD_STATUS", map[string]any{
			logging.ENDPOINT:    queryURL,
			logging.STATUS_CODE: resp.StatusCode,
		})
		return nil, fmt.Errorf("prometheus query failed with status %d", resp.StatusCode)
	}

	decoder := json.NewDecoder(resp.Body)
	var res QueryRangeResponse
	err = decoder.Decode(&res)
	if err != nil {
		clientLogger.Error("PROMETHEUS_QUERY_DECODE_FAILED", map[string]any{
			logging.ERROR: err.Error(),
		})
		return nil, err
	}

	return &res, nil
}
