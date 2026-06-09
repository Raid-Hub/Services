-- Extend pantheon version leaderboard to include activity 102 (definitions come from seeds).

DROP MATERIALIZED VIEW IF EXISTS "leaderboard"."individual_pantheon_version_leaderboard";

CREATE MATERIALIZED VIEW "leaderboard"."individual_pantheon_version_leaderboard" AS
WITH pantheon_base AS (
    SELECT
        ip."membership_id",
        av."version_id",
        COUNT(*) AS clears,
        COUNT(*) FILTER (WHERE i."fresh") AS fresh_clears,
        SUM(COALESCE(i."score", 0)) AS score
    FROM "core"."instance_player" ip
    JOIN "core"."instance" i ON i."instance_id" = ip."instance_id"
    JOIN "core"."player" p ON p."membership_id" = ip."membership_id"
    JOIN "definitions"."activity_version" av ON av."hash" = i."hash"
    WHERE av."activity_id" IN (101, 102)
      AND ip."completed"
      AND i."completed"
      AND NOT p."is_private"
      AND p."cheat_level" < 2
    GROUP BY ip."membership_id", av."version_id"
)
SELECT
    membership_id,
    version_id,
    clears,
    ROW_NUMBER() OVER (PARTITION BY version_id ORDER BY clears DESC, membership_id ASC) AS clears_position,
    RANK() OVER (PARTITION BY version_id ORDER BY clears DESC) AS clears_rank,
    fresh_clears,
    ROW_NUMBER() OVER (PARTITION BY version_id ORDER BY fresh_clears DESC, membership_id ASC) AS fresh_clears_position,
    RANK() OVER (PARTITION BY version_id ORDER BY fresh_clears DESC) AS fresh_clears_rank,
    score,
    ROW_NUMBER() OVER (PARTITION BY version_id ORDER BY score DESC, membership_id ASC) AS score_position,
    RANK() OVER (PARTITION BY version_id ORDER BY score DESC) AS score_rank
FROM pantheon_base
WHERE clears > 0;

CREATE UNIQUE INDEX idx_individual_pantheon_version_leaderboard_membership_id ON "leaderboard"."individual_pantheon_version_leaderboard" (version_id ASC, membership_id ASC);
CREATE UNIQUE INDEX idx_individual_pantheon_version_leaderboard_clears ON "leaderboard"."individual_pantheon_version_leaderboard" (version_id ASC, clears_position ASC);
CREATE UNIQUE INDEX idx_individual_pantheon_version_leaderboard_fresh_clears ON "leaderboard"."individual_pantheon_version_leaderboard" (version_id ASC, fresh_clears_position ASC);
CREATE UNIQUE INDEX idx_individual_pantheon_version_leaderboard_score ON "leaderboard"."individual_pantheon_version_leaderboard" (version_id ASC, score_position ASC);
