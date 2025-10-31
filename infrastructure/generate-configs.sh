#!/bin/bash
# Generate service configurations from .env file using templates

set -e

echo "🔧 Generating service configurations from .env..."

# Load .env file
if [ ! -f .env ]; then
    echo "❌ .env file not found. Please create one from example.env"
    exit 1
fi

# Source .env file and handle errors properly
if ! source .env; then
    echo "❌ Error: Failed to load .env file. Check for syntax errors."
    echo "   Common issues:"
    echo "   - Unquoted URLs with < > characters"
    echo "   - Missing quotes around values with spaces"
    echo "   - Invalid shell syntax"
    exit 1
fi

# Validate required environment variables (fail fast if undefined)
required_vars=(
    "POSTGRES_USER"
    "POSTGRES_PASSWORD"
    "POSTGRES_DB"
    "POSTGRES_READONLY_USER"
    "POSTGRES_READONLY_PASSWORD"
    "RABBITMQ_USER"
    "RABBITMQ_PASSWORD"
    "CLICKHOUSE_USER"
    "CLICKHOUSE_PASSWORD"
)

for var in "${required_vars[@]}"; do
    if [ -z "${!var}" ] || [ "${!var}" = "undefined" ]; then
        echo "❌ Error: $var is not defined in .env file"
        exit 1
    fi
done

# Create infrastructure directories
mkdir -p infrastructure/postgres/init
mkdir -p infrastructure/rabbitmq
mkdir -p infrastructure/clickhouse

# Helper function to replace template variables
replace_template() {
    local template_file=$1
    local output_file=$2
    
    # Replace all {{VAR}} placeholders with actual values (using | as delimiter to avoid conflicts with URLs)
    sed \
        -e "s|{{POSTGRES_USER}}|${POSTGRES_USER}|g" \
        -e "s|{{POSTGRES_PASSWORD}}|${POSTGRES_PASSWORD}|g" \
        -e "s|{{POSTGRES_DB}}|${POSTGRES_DB}|g" \
        -e "s|{{POSTGRES_READONLY_USER}}|${POSTGRES_READONLY_USER}|g" \
        -e "s|{{POSTGRES_READONLY_PASSWORD}}|${POSTGRES_READONLY_PASSWORD}|g" \
        -e "s|{{POSTGRES_PORT}}|${POSTGRES_PORT}|g" \
        -e "s|{{RABBITMQ_USER}}|${RABBITMQ_USER}|g" \
        -e "s|{{RABBITMQ_PASSWORD}}|${RABBITMQ_PASSWORD}|g" \
        -e "s|{{CLICKHOUSE_USER}}|${CLICKHOUSE_USER}|g" \
        -e "s|{{CLICKHOUSE_DB}}|${CLICKHOUSE_DB}|g" \
        -e "s|{{CLICKHOUSE_PASSWORD}}|${CLICKHOUSE_PASSWORD}|g" \
        -e "s|{{POSTGRES_HOST}}|${POSTGRES_HOST:-localhost}|g" \
        -e "s|{{POSTGRES_PORT}}|${POSTGRES_PORT:-5432}|g" \
        -e "s|{{ATLAS_METRICS_PORT}}|${ATLAS_METRICS_PORT}|g" \
        -e "s|{{HERMES_METRICS_PORT}}|${HERMES_METRICS_PORT}|g" \
        -e "s|{{ZEUS_METRICS_PORT}}|${ZEUS_METRICS_PORT}|g" \
        "$template_file" > "$output_file"
}

# Generate PostgreSQL init-users.sql
echo "  → Generating PostgreSQL users..."
replace_template \
    infrastructure/postgres/init-database.sql.template \
    infrastructure/postgres/init/01-users.sql

# Generate RabbitMQ definitions.json
echo "  → Generating RabbitMQ definitions..."
replace_template \
    infrastructure/rabbitmq/definitions.json.template \
    infrastructure/rabbitmq/definitions.json

# Generate ClickHouse config.xml
echo "  → Generating ClickHouse config..."
replace_template \
    infrastructure/clickhouse/config.xml.template \
    infrastructure/clickhouse/config.xml

# Generate ClickHouse users.xml
echo "  → Generating ClickHouse users..."
replace_template \
    infrastructure/clickhouse/users.xml.template \
    infrastructure/clickhouse/users.xml

# Generate ClickHouse named collections
echo "  → Generating ClickHouse named collections..."
replace_template \
    infrastructure/clickhouse/named_collections.xml.template \
    infrastructure/clickhouse/named_collections.xml

# Generate Prometheus prometheus.yml
echo "  → Generating Prometheus config..."
replace_template \
    infrastructure/prometheus/prometheus.yml.template \
    infrastructure/prometheus/prometheus.yml

echo "✅ Service configurations generated successfully"
