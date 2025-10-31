#!/bin/bash
# Leaderboard Refresher for RaidHub Services
# Refreshes cache view first, then the leaderboard view (completely independent)
# After leaderboard refresh, clears the cache to free up space
# Usage: refresh-leaderboard.sh LEADERBOARD_NAME
#
# Supported leaderboard names:
#   - individual_global_leaderboard (uses _global_leaderboard_cache)
#   - individual_raid_leaderboard (uses _raid_leaderboard_cache)

set -e

LEADERBOARD_NAME="$1"
LOG_FILE="/RaidHub/cron.log"

if [ -z "$LEADERBOARD_NAME" ]; then
    echo "Error: No leaderboard name provided"
    exit 1
fi

# Load environment variables from .env
cd "$RAIDHUB_PATH"
if [ -f .env ]; then
    set -a
    source .env
    set +a
fi

# Determine which cache view to refresh based on leaderboard name
case "$LEADERBOARD_NAME" in
    "individual_global_leaderboard")
        CACHE_VIEW="_global_leaderboard_cache"
        ;;
    "individual_raid_leaderboard")
        CACHE_VIEW="_raid_leaderboard_cache"
        ;;
    *)
        echo "Error: Unknown leaderboard '$LEADERBOARD_NAME'"
        echo "Supported: individual_global_leaderboard, individual_raid_leaderboard"
        exit 1
        ;;
esac

# Refresh the cache view first
echo "[$(date)] Refreshing cache: leaderboard.$CACHE_VIEW" >> "$LOG_FILE"
PGPASSWORD="$POSTGRES_PASSWORD" psql -h localhost -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
    -c "REFRESH MATERIALIZED VIEW CONCURRENTLY leaderboard.$CACHE_VIEW WITH DATA" >> "$LOG_FILE" 2>&1

# Then refresh the leaderboard view
echo "[$(date)] Refreshing leaderboard: leaderboard.$LEADERBOARD_NAME" >> "$LOG_FILE"
PGPASSWORD="$POSTGRES_PASSWORD" psql -h localhost -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
    -c "REFRESH MATERIALIZED VIEW CONCURRENTLY leaderboard.$LEADERBOARD_NAME WITH DATA" >> "$LOG_FILE" 2>&1

# Clear the cache after leaderboard refresh to free up space
# Recreate the cache view with original definition (empty until next refresh)
echo "[$(date)] Clearing cache: leaderboard.$CACHE_VIEW" >> "$LOG_FILE"
PGPASSWORD="$POSTGRES_PASSWORD" psql -h localhost -U "$POSTGRES_USER" -d "$POSTGRES_DB" <<EOF >> "$LOG_FILE" 2>&1
-- Refresh without CONCURRENTLY first (required before dropping)
REFRESH MATERIALIZED VIEW leaderboard.$CACHE_VIEW WITH DATA;
-- Drop and recreate with original query (will be empty until next refresh)
DROP MATERIALIZED VIEW leaderboard.$CACHE_VIEW CASCADE;
EOF

# Recreate with original query structure (empty until next refresh)
if [ "$CACHE_VIEW" = "_global_leaderboard_cache" ]; then
    PGPASSWORD="$POSTGRES_PASSWORD" psql -h localhost -U "$POSTGRES_USER" -d "$POSTGRES_DB" <<EOF >> "$LOG_FILE" 2>&1
CREATE MATERIALIZED VIEW leaderboard._global_leaderboard_cache AS
SELECT
  membership_id,
  clears,
  fresh_clears,
  sherpas,
  sum_of_best,
  total_time_played_seconds,
  wfr_score,
  COUNT(*) OVER ()::numeric AS total_count
FROM "core"."player"
WHERE clears > 0 AND NOT is_private AND cheat_level < 2;
CREATE UNIQUE INDEX idx_global_leaderboard_cache_membership_id ON leaderboard._global_leaderboard_cache (membership_id ASC);
CREATE INDEX idx_global_leaderboard_cache_clears ON leaderboard._global_leaderboard_cache (clears DESC, membership_id ASC);
CREATE INDEX idx_global_leaderboard_cache_fresh_clears ON leaderboard._global_leaderboard_cache (fresh_clears DESC, membership_id ASC);
CREATE INDEX idx_global_leaderboard_cache_sherpas ON leaderboard._global_leaderboard_cache (sherpas DESC, membership_id ASC);
CREATE INDEX idx_global_leaderboard_cache_speed ON leaderboard._global_leaderboard_cache (sum_of_best ASC NULLS LAST, membership_id ASC);
CREATE INDEX idx_global_leaderboard_cache_time ON leaderboard._global_leaderboard_cache (total_time_played_seconds DESC, membership_id ASC);
CREATE INDEX idx_global_leaderboard_cache_wfr ON leaderboard._global_leaderboard_cache (wfr_score DESC, membership_id ASC);
EOF
elif [ "$CACHE_VIEW" = "_raid_leaderboard_cache" ]; then
    PGPASSWORD="$POSTGRES_PASSWORD" psql -h localhost -U "$POSTGRES_USER" -d "$POSTGRES_DB" <<EOF >> "$LOG_FILE" 2>&1
CREATE MATERIALIZED VIEW leaderboard._raid_leaderboard_cache AS
SELECT
  ps.activity_id,
  ps.membership_id,
  ps.clears,
  ps.fresh_clears,
  ps.sherpas,
  ps.total_time_played_seconds,
  COUNT(*) OVER (PARTITION BY ps.activity_id) AS total_count
FROM "core"."player_stats" ps
JOIN "core"."player" p ON p.membership_id = ps.membership_id
WHERE ps.clears > 0
  AND NOT p.is_private
  AND p.cheat_level < 2;
CREATE UNIQUE INDEX idx_raid_leaderboard_cache_activity_membership ON leaderboard._raid_leaderboard_cache (activity_id ASC, membership_id ASC);
CREATE INDEX idx_raid_leaderboard_cache_clears ON leaderboard._raid_leaderboard_cache (activity_id ASC, clears DESC, membership_id ASC);
CREATE INDEX idx_raid_leaderboard_cache_fresh_clears ON leaderboard._raid_leaderboard_cache (activity_id ASC, fresh_clears DESC, membership_id ASC);
CREATE INDEX idx_raid_leaderboard_cache_sherpas ON leaderboard._raid_leaderboard_cache (activity_id ASC, sherpas DESC, membership_id ASC);
CREATE INDEX idx_raid_leaderboard_cache_time ON leaderboard._raid_leaderboard_cache (activity_id ASC, total_time_played_seconds DESC, membership_id ASC);
EOF
fi

exit $?
