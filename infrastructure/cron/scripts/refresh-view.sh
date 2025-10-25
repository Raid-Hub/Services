#!/bin/bash
# Materialized View Refresher for RaidHub Services
# Usage: sql-executor.sh VIEW_NAME

set -e

VIEW_NAME="$1"
LOG_FILE="/RaidHub/cron.log"

if [ -z "$VIEW_NAME" ]; then
    echo "Error: No view name provided"
    exit 1
fi

# Load environment variables from .env
cd "$RAIDHUB_PATH"
if [ -f .env ]; then
    set -a
    source .env
    set +a
fi

# Refresh the materialized view
PGPASSWORD="$POSTGRES_PASSWORD" psql -h localhost -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "REFRESH MATERIALIZED VIEW CONCURRENTLY $VIEW_NAME WITH DATA" >> "$LOG_FILE" 2>&1

exit $?
