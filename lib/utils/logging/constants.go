package logging

// Log levels
const (
	debug = "DEBUG" // Diagnostic information for IT/sysadmins when verbose flag is passed
	info  = "INFO"  // Generally useful information (service start/stop, configuration assumptions)
	warn  = "WARN"  // Recoverable issues (failover, retries, missing secondary data) - no alerts
	error = "ERROR" // Operation-fatal errors requiring user intervention - triggers alerts/Sentry
	fatal = "FATAL" // Service-fatal errors that crash the application to prevent data loss
)

// Standard logging field keys - use constants to ensure consistency
const (
	// Common fields
	TYPE    = "type"
	STATUS  = "status"
	ERROR   = "error" // Field key for error messages
	SERVICE = "service"
	VERSION = "version"
	REASON  = "reason"
	FORMAT  = "format"
	SOURCE  = "source"
	PATH    = "path"

	// Database fields
	RETRIES   = "retries"
	OPERATION = "operation"
	TABLE     = "table"
	FILENAME  = "filename"
	RANGE     = "range"
	FROM      = "from"
	TO        = "to"
	ATTEMPTS  = "attempts"

	// Network fields
	METHOD      = "method"
	ENDPOINT    = "endpoint"
	STATUS_CODE = "status_code"

	// User/Entity fields
	MEMBERSHIP_ID = "membership_id"
	INSTANCE_ID   = "instance_id"
	GROUP_ID      = "group_id"
	CHARACTER_ID  = "character_id"
	PLAYERS       = "players"
	HASH          = "hash"
	NAME          = "name"
	ACTIVITIES    = "activities"

	// Process fields
	WORKER_COUNT = "worker_count"
	COUNT        = "count"
	PHASE        = "phase"
	ATTEMPT      = "attempt"
	ACTION       = "action"
	SUCCESSFUL   = "successful"
	TOTAL        = "total"
	FAILED       = "failed"
	DIRECTORY    = "directory"
	FIRST_ID     = "first_id"
	MIN          = "min"
	MAX          = "max"
	GAPS         = "gaps"
	MIN_FAILED   = "min_failed"
	MAX_FAILED   = "max_failed"
	QUEUE_DEPTH  = "queue_depth"

	// Security/Detection fields
	CLASS_A_FLAGS  = "class_a_flags"
	PREVIOUS_LEVEL = "previous_level"
	NEW_LEVEL      = "new_level"
	BUNGIE_NAME    = "bungie_name"
	CLEARS         = "clears"
	AGE_DAYS       = "age_days"
	CHEAT_CHANCE   = "cheat_chance"
	FLAGS          = "flags"
	LAST_PLAYED    = "last_played"

	// Progress fields
	PROGRESS_PERCENT  = "progress_percent"
	PARTITIONS_TOTAL  = "partitions_total"
	PARTITIONS_DONE   = "partitions_done"
	PROGRESS_MEASURED = "progress_measured"
	UNIT              = "unit"

	// Timing fields
	DURATION = "duration"
	LAG      = "lag"
)

const (
	WAITING_ON_CONNECTIONS   = "WAITING_ON_CONNECTIONS"
	RECEIVED_SHUTDOWN_SIGNAL = "RECEIVED_SHUTDOWN_SIGNAL"
)
