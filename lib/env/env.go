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
	PostgresHost     string
	PostgresPort     string

	// RabbitMQ
	RabbitMQUser     string
	RabbitMQPassword string
	RabbitMQHost     string
	RabbitMQPort     string

	// ClickHouse
	ClickHouseUser     string
	ClickHousePassword string
	ClickHouseHost     string
	ClickHousePort     string

	// API
	BungieAPIKey  string
	BungieURLBase string
	ZeusAPIKeys   string
	IPV6          string
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
)

var envIssues []string

func init() {
	// Load .env file (ignore error - variables may be set via environment)
	godotenv.Load()

	// Database
	PostgresUser = requireEnv("POSTGRES_USER")
	PostgresPassword = requireEnv("POSTGRES_PASSWORD")
	PostgresDB = requireEnv("POSTGRES_DB")
	PostgresHost = requireEnv("POSTGRES_HOST")
	PostgresPort = requireEnv("POSTGRES_PORT")

	// RabbitMQ
	RabbitMQUser = requireEnv("RABBITMQ_USER")
	RabbitMQPassword = requireEnv("RABBITMQ_PASSWORD")
	RabbitMQHost = requireEnv("RABBITMQ_HOST")
	RabbitMQPort = requireEnv("RABBITMQ_PORT")

	// ClickHouse
	ClickHouseUser = requireEnv("CLICKHOUSE_USER")
	ClickHousePassword = requireEnv("CLICKHOUSE_PASSWORD")
	ClickHouseHost = requireEnv("CLICKHOUSE_HOST")
	ClickHousePort = requireEnv("CLICKHOUSE_PORT")

	// API
	BungieAPIKey = requireEnv("BUNGIE_API_KEY")
	BungieURLBase = requireEnv("BUNGIE_URL_BASE")
	ZeusAPIKeys = getEnv("ZEUS_API_KEYS")
	IPV6 = getEnv("IPV6")
	PGCRURLBase = requireEnv("PGCR_URL_BASE")

	// Webhooks (optional)
	AtlasWebhookURL = getEnv("ATLAS_WEBHOOK_URL")
	HadesWebhookURL = getEnv("HADES_WEBHOOK_URL")
	AlertsRoleID = getEnv("ALERTS_ROLE_ID")
	CheatCheckWebhookURL = getEnv("CHEAT_CHECK_WEBHOOK_URL")
	NemesisWebhookURL = getEnv("NEMESIS_WEBHOOK_URL")
	GMReportWebhookURL = getEnv("GM_REPORT_WEBHOOK_URL")
	GMReportWebhookAuth = getEnv("GM_REPORT_WEBHOOK_AUTH")

	// Other
	IsContestWeekend = getEnv("IS_CONTEST_WEEKEND") == "true"
	MissedPGCRLogFilePath = getEnv("MISSED_PGCR_LOG_FILE_PATH")

	if len(envIssues) > 0 {
		panic("Required environment variables are not set: " + strings.Join(envIssues, ", "))
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
