#!/bin/bash
# Script to sync Grafana dashboards from the database back to infrastructure directory
# This allows you to export dashboards saved in the Grafana UI back to your infrastructure files
#
# Usage:
#   make sync-dashboards
#   or
#   ./infrastructure/grafana/sync-dashboards.sh
#
# The script reads GRAFANA_PORT from your .env file automatically

set -e

# Load .env file if it exists to get GRAFANA_PORT
if [ -f .env ]; then
    export $(grep -v '^#' .env | xargs)
fi

GRAFANA_PORT="${GRAFANA_PORT:-3000}"
GRAFANA_URL="${GRAFANA_URL:-http://localhost:${GRAFANA_PORT}}"
GRAFANA_USER="${GRAFANA_ADMIN_USER:-dev}"
GRAFANA_PASSWORD="${GRAFANA_ADMIN_PASSWORD:-password}"
DASHBOARDS_DIR="${DASHBOARDS_DIR:-./infrastructure/grafana/dashboards}"

# Check dependencies
if ! command -v jq &> /dev/null; then
    echo "❌ Error: jq is required but not installed"
    echo "   Install with: brew install jq (macOS) or apt-get install jq (Linux)"
    exit 1
fi

if ! command -v curl &> /dev/null; then
    echo "❌ Error: curl is required but not installed"
    exit 1
fi

# Get API key or use basic auth
if [ -n "$GRAFANA_API_KEY" ]; then
    CURL_AUTH=(-H "Authorization: Bearer $GRAFANA_API_KEY")
else
    CURL_AUTH=(-u "$GRAFANA_USER:$GRAFANA_PASSWORD")
fi

# Get all dashboards from Grafana
echo "📥 Fetching dashboards from Grafana ($GRAFANA_URL)..."
DASHBOARDS_JSON=$(curl -s "${CURL_AUTH[@]}" "$GRAFANA_URL/api/search?type=dash-db")
DASHBOARD_UIDS=$(echo "$DASHBOARDS_JSON" | jq -r '.[].uid // empty')

if [ -z "$DASHBOARD_UIDS" ]; then
    echo "⚠️  No dashboards found in Grafana"
    # Delete all existing dashboard files since there are no dashboards
    if [ -d "$DASHBOARDS_DIR" ]; then
        find "$DASHBOARDS_DIR" -name "*.json" -type f -delete
        echo "🗑️  Removed all dashboard files (no dashboards in Grafana)"
    fi
    exit 0
fi

echo "📊 Found dashboards, syncing..."

# Create a temporary file to track which files should exist
TEMP_UIDS_FILE=$(mktemp)
echo "$DASHBOARD_UIDS" > "$TEMP_UIDS_FILE"

# Track which files we've processed
PROCESSED_FILES=()

# Export each dashboard
for uid in $DASHBOARD_UIDS; do
    echo "  → Exporting dashboard UID: $uid"
    
    # Get dashboard JSON
    DASHBOARD_RESPONSE=$(curl -s "${CURL_AUTH[@]}" "$GRAFANA_URL/api/dashboards/uid/$uid")
    DASHBOARD_JSON=$(echo "$DASHBOARD_RESPONSE" | jq '.dashboard')
    
    # Get dashboard title for filename
    TITLE=$(echo "$DASHBOARD_JSON" | jq -r '.title // "dashboard"' | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9]/-/g' | sed 's/--*/-/g')
    
    # Get dashboard UID from the response
    DASHBOARD_UID=$(echo "$DASHBOARD_JSON" | jq -r '.uid // empty')
    
    # Save to file (use UID as filename if title is empty)
    if [ -z "$TITLE" ] || [ "$TITLE" == "null" ]; then
        FILENAME="$uid.json"
    else
        FILENAME="${TITLE}.json"
    fi
    
    OUTPUT_FILE="$DASHBOARDS_DIR/$FILENAME"
    PROCESSED_FILES+=("$OUTPUT_FILE")
    
    # Save the dashboard JSON (remove metadata like id, version, etc. for cleaner files)
    echo "$DASHBOARD_JSON" | jq 'del(.id, .version, .created, .updated, .createdBy, .updatedBy)' > "$OUTPUT_FILE"
    
    echo "    ✓ Saved to: $OUTPUT_FILE"
done

# Delete dashboard files that no longer exist in Grafana
echo ""
echo "🗑️  Checking for deleted dashboards..."
DELETED_COUNT=0
if [ -d "$DASHBOARDS_DIR" ]; then
    while IFS= read -r -d '' file; do
        # Check if this file was processed (exists in Grafana)
        FILE_EXISTS=false
        for processed_file in "${PROCESSED_FILES[@]}"; do
            if [ "$file" == "$processed_file" ]; then
                FILE_EXISTS=true
                break
            fi
        done
        
        # If file wasn't processed, it means it was deleted from Grafana
        if [ "$FILE_EXISTS" == "false" ]; then
            echo "  → Deleting: $(basename "$file")"
            rm -f "$file"
            DELETED_COUNT=$((DELETED_COUNT + 1))
        fi
    done < <(find "$DASHBOARDS_DIR" -name "*.json" -type f -print0 2>/dev/null || true)
fi

# Clean up temp file
rm -f "$TEMP_UIDS_FILE"

echo "✅ Dashboard sync complete!"
if [ $DELETED_COUNT -gt 0 ]; then
    echo "   Deleted $DELETED_COUNT dashboard file(s) that no longer exist in Grafana"
fi
echo ""
echo "💡 Tip: Review the changes and commit to git:"
echo "   git add $DASHBOARDS_DIR/*.json"
echo "   git commit -m 'Update Grafana dashboards'"

