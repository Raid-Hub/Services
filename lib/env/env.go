package env

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

var (
	// Database
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string
	PostgresPort     string

	// RabbitMQ
	RabbitMQUser     string
	RabbitMQPassword string
	RabbitMQPort     string

	// ClickHouse
	ClickHouseUser     string
	ClickHousePassword string
	ClickHouseDB       string
	ClickHousePort     string

	// API
	BungieAPIKey  string
	BungieURLBase string
	ZeusAPIKeys   string
	ZeusIPV6      string
	PGCRURLBase   string

	// Webhooks (optional)
	AtlasWebhookURL      string
	HadesWebhookURL      string
	AlertsRoleID         string
	CheatCheckWebhookURL string
	NemesisWebhookURL    string
	GMReportWebhookURL   string
	GMReportWebhookAuth  string

	// Other
	IsContestWeekend      bool
	MissedPGCRLogFilePath string

	// Prometheus API (for querying metrics, not the exporter)
	PrometheusPort string

	// Metrics export ports (for Prometheus scraping)
	AtlasMetricsPort  string
	HermesMetricsPort string
	ZeusMetricsPort   string
)

var envIssues []string

func init() {
	// Load .env file (ignore error - variables may be set via environment)
	godotenv.Load()

	// Database
	PostgresUser = requireEnv("POSTGRES_USER")
	PostgresPassword = requireEnv("POSTGRES_PASSWORD")
	PostgresDB = requireEnv("POSTGRES_DB")
	PostgresPort = requireEnv("POSTGRES_PORT")

	// RabbitMQ
	RabbitMQUser = requireEnv("RABBITMQ_USER")
	RabbitMQPassword = requireEnv("RABBITMQ_PASSWORD")
	RabbitMQPort = requireEnv("RABBITMQ_PORT")

	// ClickHouse
	ClickHouseUser = requireEnv("CLICKHOUSE_USER")
	ClickHousePassword = requireEnv("CLICKHOUSE_PASSWORD")
	ClickHouseDB = requireEnv("CLICKHOUSE_DB")
	ClickHousePort = requireEnv("CLICKHOUSE_PORT")

	// API
	BungieAPIKey = requireEnv("BUNGIE_API_KEY")
	BungieURLBase = requireEnv("BUNGIE_URL_BASE")
	ZeusAPIKeys = getEnv("ZEUS_API_KEYS")
	ZeusIPV6 = getEnv("ZEUS_IPV6")
	PGCRURLBase = requireEnv("PGCR_URL_BASE")

	// Webhooks (optional)
	AtlasWebhookURL = getEnv("ATLAS_WEBHOOK_URL")
	HadesWebhookURL = getEnv("HADES_WEBHOOK_URL")
	AlertsRoleID = getEnv("ALERTS_ROLE_ID")
	CheatCheckWebhookURL = getEnv("CHEAT_CHECK_WEBHOOK_URL")
	NemesisWebhookURL = getEnv("NEMESIS_WEBHOOK_URL")
	GMReportWebhookURL = getEnv("GM_REPORT_WEBHOOK_URL")
	GMReportWebhookAuth = getEnv("GM_REPORT_WEBHOOK_AUTH")

	// Config
	IsContestWeekend = getEnv("IS_CONTEST_WEEKEND") == "true"
	MissedPGCRLogFilePath = requireEnv("MISSED_PGCR_LOG_FILE_PATH")

	// Prometheus API (required)
	PrometheusPort = requireEnv("PROMETHEUS_PORT")

	// Metrics export ports (for Prometheus scraping)
	AtlasMetricsPort = requireEnv("ATLAS_METRICS_PORT")
	HermesMetricsPort = requireEnv("HERMES_METRICS_PORT")
	ZeusMetricsPort = getEnv("ZEUS_METRICS_PORT")

	if len(envIssues) > 0 {
		panic("required environment variables are not set: " + strings.Join(envIssues, ", "))
	}
}

func getEnv(key string) string {
	return os.Getenv(key)
}

func requireEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		envIssues = append(envIssues, key)
	}
	return val
}
