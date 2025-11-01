package env

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

var (
	// Database
	PostgresHost     string
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string
	PostgresPort     string

	// RabbitMQ
	RabbitMQHost     string
	RabbitMQUser     string
	RabbitMQPassword string
	RabbitMQPort     string

	// ClickHouse
	ClickHouseHost     string
	ClickHouseUser     string
	ClickHousePassword string
	ClickHouseDB       string
	ClickHousePort     string

	// Prometheus
	PrometheusHost string

	// Zeus
	ZeusHost string
	ZeusPort string

	// API
	BungieAPIKey string
	ZeusAPIKeys  string
	ZeusIPV6     string

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

	// Database (defaults: user=postgres, password="", db=raidhub)
	PostgresHost = getHostEnv("POSTGRES_HOST")
	PostgresUser = getEnvWithDefault("POSTGRES_USER", "postgres")
	PostgresPassword = getEnv("POSTGRES_PASSWORD") // Default: empty string
	PostgresDB = getEnvWithDefault("POSTGRES_DB", "raidhub")
	PostgresPort = requireEnv("POSTGRES_PORT")

	// RabbitMQ (defaults: user=guest, password=guest)
	RabbitMQHost = getHostEnv("RABBITMQ_HOST")
	RabbitMQUser = getEnvWithDefault("RABBITMQ_USER", "guest")
	RabbitMQPassword = getEnvWithDefault("RABBITMQ_PASSWORD", "guest")
	RabbitMQPort = requireEnv("RABBITMQ_PORT")

	// ClickHouse (defaults: user=default, password="", db=raidhub)
	ClickHouseHost = getHostEnv("CLICKHOUSE_HOST")
	ClickHouseUser = getEnvWithDefault("CLICKHOUSE_USER", "default")
	ClickHousePassword = getEnvWithDefault("CLICKHOUSE_PASSWORD", "")
	ClickHouseDB = getEnvWithDefault("CLICKHOUSE_DB", "default")
	ClickHousePort = requireEnv("CLICKHOUSE_PORT")

	// Zeus
	ZeusHost = getHostEnv("ZEUS_HOST")
	ZeusPort = getHostEnv("ZEUS_PORT")

	// API
	BungieAPIKey = requireEnv("BUNGIE_API_KEY")
	ZeusAPIKeys = getEnv("ZEUS_API_KEYS")
	ZeusIPV6 = getEnv("ZEUS_IPV6")

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
	PrometheusHost = getHostEnv("PROMETHEUS_HOST")
	PrometheusPort = requireEnv("PROMETHEUS_PORT")

	// Metrics export ports (for Prometheus scraping)
	AtlasMetricsPort = requireEnv("ATLAS_METRICS_PORT")
	HermesMetricsPort = getEnv("HERMES_METRICS_PORT") // Optional for individual topic runners
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

func getEnvWithDefault(key string, defaultValue string) string {
	if val := getEnv(key); val != "" {
		return val
	}
	return defaultValue
}


func getHostEnv(key string) string {
	return getEnvWithDefault(key, "localhost")
}
