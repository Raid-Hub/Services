#!/bin/bash

# Add config import to files that need it
for file in apps/hades/main.go apps/atlas/worker.go apps/atlas/main.go apps/atlas/offload-worker.go apps/atlas/alerting.go apps/zeus/main.go; do
    if [ -f "$file" ]; then
        # Check if config import already exists
        if ! grep -q "raidhub/lib/env" "$file"; then
            # Add import after raidhub imports
            sed -i.bak '/^	"raidhub\/lib\//a\
	"raidhub/lib/env"
' "$file"
            rm "$file.bak" 2>/dev/null
        fi
    fi
done

# Fix lib files
for file in lib/pgcr/fetch.go lib/cheat_detection/external.go lib/cheat_detection/webhooks.go lib/messaging/processing/topic_manager.go; do
    if [ -f "$file" ]; then
        if ! grep -q "raidhub/lib/env" "$file"; then
            sed -i.bak '/^	"github\.com/a\
	"raidhub/lib/env"
' "$file"
            rm "$file.bak" 2>/dev/null
        fi
    fi
done

