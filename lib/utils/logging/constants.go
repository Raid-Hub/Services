package logging

// Log levels
const (
	DEBUG = "DEBUG" // Diagnostic information for IT/sysadmins when verbose flag is passed
	INFO  = "INFO"  // Generally useful information (service start/stop, configuration assumptions)
	WARN  = "WARN"  // Recoverable issues (failover, retries, missing secondary data) - no alerts
	ERROR = "ERROR" // Operation-fatal errors requiring user intervention - triggers alerts/Sentry
	FATAL = "FATAL" // Service-fatal errors that crash the application to prevent data loss
)

const (
	Error = "error"
	Fatal = "fatal"
	Warn  = "warn"
	Info  = "info"
	Debug = "debug"
)

// Standard logging field keys - use constants to ensure consistency
const (
	// Core fields - most commonly used generic fields
	ACTION  = "action"
	CACHE   = "cache"
	COUNT   = "count"
	KEY     = "key"
	NAME    = "name"
	PATH    = "path"
	PHASE   = "phase"
	REASON  = "reason"
	SIZE    = "size"
	STATUS  = "status"
	TYPE    = "type"
	VALUE   = "value"
	VERSION = "version"

	// Infrastructure fields - host, port, network components
	HOST    = "host"
	PORT    = "port"
	QUEUE   = "queue"
	SYSTEM  = "system"
	SERVICE = "service"
	SOURCE  = "source"

	// Network/HTTP fields
	ENDPOINT          = "endpoint"
	ERROR_STATUS      = "error_status"
	METHOD            = "method"
	STATUS_CODE       = "status_code"
	BUNGIE_ERROR_CODE = "bungie_error_code"

	// Database fields
	APPLIED_COUNT = "applied_count"
	ATTEMPTS      = "attempts"
	FILENAME      = "filename"
	FROM          = "from"
	OPERATION     = "operation"
	RANGE         = "range"
	RETRIES       = "retries"
	TABLE         = "table"
	TO            = "to"

	// User/Entity fields
	ACTIVITIES      = "activities"
	CHARACTER_ID    = "character_id"
	FIRST_SEEN      = "first_seen"
	GROUP_ID        = "group_id"
	HASH            = "hash"
	INSTANCE_ID     = "instance_id"
	IS_NEW_PLAYER   = "is_new_player"
	LAST_SEEN       = "last_seen"
	MEMBERSHIP_ID   = "membership_id"
	MEMBERSHIP_TYPE = "membership_type"
	PLAYERS         = "players"
	USER_ID         = "user_id"

	// Process/Operation fields
	ATTEMPT      = "attempt"
	COMMAND      = "command"
	DIRECTORY    = "directory"
	ENTITY       = "entity"
	FAILED       = "failed"
	FIRST_ID     = "first_id"
	FORMAT       = "format"
	GAPS         = "gaps"
	ISSUE        = "issue"
	JOB          = "job"
	JOB_ID       = "job_id"
	LOG_TYPE     = "log_type"
	MAX          = "max"
	MAX_FAILED   = "max_failed"
	MERGED       = "merged"
	MIN          = "min"
	MIN_FAILED   = "min_failed"
	PARAMS       = "params"
	QUERY        = "query"
	QUEUE_DEPTH  = "queue_depth"
	RATE         = "rate"
	SUCCESSFUL   = "successful"
	TOTAL        = "total"
	WORKER_COUNT = "worker_count"

	// Timing/Duration fields
	DURATION = "duration"
	LAG      = "lag"
	VIEW     = "view"
)
