package env

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

var (
	// Postgres
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
	EnvPath               string
	LogLevel              string

	// Prometheus API (for querying metrics, not the exporter)
	PrometheusPort string

	// Metrics export ports (for Prometheus scraping)
	AtlasMetricsPort  string
	HermesMetricsPort string
	ZeusMetricsPort   string

	// Logging file descriptors
	StdoutPath string
	StderrPath string

	// Cron Manager
	CronManagerPort string

	// Sentry
	SentryDSN   string
	Environment string
	Release     string
)

var envIssues []string

func init() {

	envPaths := []string{".env"}
	if envPath := getEnv("ENV_PATH"); envPath != "" {
		envPaths = append(envPaths, envPath)
	}

	// Load .env files separately - ignore errors for missing files
	// godotenv.Load() stops on first error, so we need to load each file individually
	for _, envPath := range envPaths {
		if err := godotenv.Load(envPath); err != nil {
			// Ignore "file not found" errors - file might not exist
			if !os.IsNotExist(err) {
				// Log non-existent file errors, but continue
				_ = err
			}
		}
	}

	// Database (defaults: user=postgres, password="password", db=raidhub)
	PostgresHost = getHostEnv("POSTGRES_HOST")
	PostgresUser = getEnvWithDefault("POSTGRES_USER", "dev")
	PostgresPassword = getEnvWithDefault("POSTGRES_PASSWORD", "password")
	PostgresDB = getEnvWithDefault("POSTGRES_DB", "raidhub")
	PostgresPort = requireEnv("POSTGRES_PORT")

	// RabbitMQ (defaults: user=dev, password=password)
	RabbitMQHost = getHostEnv("RABBITMQ_HOST")
	RabbitMQUser = getEnvWithDefault("RABBITMQ_USER", "dev")
	RabbitMQPassword = getEnvWithDefault("RABBITMQ_PASSWORD", "password")
	RabbitMQPort = requireEnv("RABBITMQ_PORT")

	// ClickHouse (defaults: user=dev, password=password, db=default)
	ClickHouseHost = getHostEnv("CLICKHOUSE_HOST")
	ClickHouseUser = getEnvWithDefault("CLICKHOUSE_USER", "dev")
	ClickHousePassword = getEnvWithDefault("CLICKHOUSE_PASSWORD", "password")
	ClickHouseDB = getEnvWithDefault("CLICKHOUSE_DB", "default")
	ClickHousePort = requireEnv("CLICKHOUSE_PORT")

	// Zeus
	ZeusHost = getHostEnv("ZEUS_HOST")
	ZeusPort = getHostEnv("ZEUS_PORT")
	ZeusAPIKeys = getEnv("ZEUS_API_KEYS")
	ZeusIPV6 = getEnvWithDefault("ZEUS_IPV6", "")

	// API
	BungieAPIKey = requireEnv("BUNGIE_API_KEY")

	// Discord Webhooks (optional)
	AtlasWebhookURL = getEnv("ATLAS_WEBHOOK_URL")
	HadesWebhookURL = getEnv("HADES_WEBHOOK_URL")
	CheatCheckWebhookURL = getEnv("CHEAT_CHECK_WEBHOOK_URL")
	AlertsRoleID = getEnv("DISCORD_ALERTS_ROLE_ID")

	// Config
	IsContestWeekend = getEnv("IS_CONTEST_WEEKEND") == "true"
	MissedPGCRLogFilePath = requireEnv("MISSED_PGCR_LOG_FILE_PATH")
	LogLevel = getEnv("LOG_LEVEL")
	// Prometheus API (required)
	PrometheusHost = getEnv("PROMETHEUS_HOST")
	PrometheusPort = requireEnv("PROMETHEUS_PORT")

	// Metrics export ports (for Prometheus scraping)
	AtlasMetricsPort = requireEnv("ATLAS_METRICS_PORT")
	HermesMetricsPort = requireEnv("HERMES_METRICS_PORT")
	ZeusMetricsPort = requireEnv("ZEUS_METRICS_PORT")

	// Initialize stdout/stderr file descriptors
	StdoutPath = getEnv("STDOUT")
	StderrPath = getEnv("STDERR")

	// Cron Manager
	CronManagerPort = getEnv("CRON_MANAGER_PORT")

	// Sentry (optional)
	SentryDSN = getEnv("SENTRY_DSN")
	Environment = getEnvWithDefault("ENVIRONMENT", "development")
	Release = getEnv("RELEASE")

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
