-- RaidHub Services - Leaderboards Schema Migration
-- Computed leaderboards and rankings

-- =============================================================================
-- CACHE VIEWS (Pre-filtered base data for faster leaderboard refreshes)
-- =============================================================================

-- Cache for global leaderboard (filters once, reused by individual_global_leaderboard)
CREATE MATERIALIZED VIEW "cache"."_global_leaderboard_cache" AS
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

CREATE UNIQUE INDEX idx_global_leaderboard_cache_membership_id ON "cache"."_global_leaderboard_cache" (membership_id ASC);
CREATE INDEX idx_global_leaderboard_cache_clears ON "cache"."_global_leaderboard_cache" (clears DESC, membership_id ASC);
CREATE INDEX idx_global_leaderboard_cache_fresh_clears ON "cache"."_global_leaderboard_cache" (fresh_clears DESC, membership_id ASC);
CREATE INDEX idx_global_leaderboard_cache_sherpas ON "cache"."_global_leaderboard_cache" (sherpas DESC, membership_id ASC);
CREATE INDEX idx_global_leaderboard_cache_speed ON "cache"."_global_leaderboard_cache" (sum_of_best ASC NULLS LAST, membership_id ASC);
CREATE INDEX idx_global_leaderboard_cache_time ON "cache"."_global_leaderboard_cache" (total_time_played_seconds DESC, membership_id ASC);
CREATE INDEX idx_global_leaderboard_cache_wfr ON "cache"."_global_leaderboard_cache" (wfr_score DESC, membership_id ASC);

-- Cache for raid leaderboard (filters once, reused by individual_raid_leaderboard)
CREATE MATERIALIZED VIEW "cache"."_individual_activity_leaderboard_cache" AS
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

CREATE UNIQUE INDEX idx_individual_activity_leaderboard_cache_activity_membership ON "cache"."_individual_activity_leaderboard_cache" (activity_id ASC, membership_id ASC);
CREATE INDEX idx_individual_activity_leaderboard_cache_clears ON "cache"."_individual_activity_leaderboard_cache" (activity_id ASC, clears DESC, membership_id ASC);
CREATE INDEX idx_individual_activity_leaderboard_cache_fresh_clears ON "cache"."_individual_activity_leaderboard_cache" (activity_id ASC, fresh_clears DESC, membership_id ASC);
CREATE INDEX idx_individual_activity_leaderboard_cache_sherpas ON "cache"."_individual_activity_leaderboard_cache" (activity_id ASC, sherpas DESC, membership_id ASC);
CREATE INDEX idx_individual_activity_leaderboard_cache_time ON "cache"."_individual_activity_leaderboard_cache" (activity_id ASC, total_time_played_seconds DESC, membership_id ASC);

-- =============================================================================
-- LEADERBOARD VIEWS
-- =============================================================================

-- Clan leaderboard
CREATE MATERIALIZED VIEW "leaderboard"."clan_leaderboard" AS
WITH ranked_players AS (
    SELECT
        cm."group_id",
        p."membership_id",
        p."clears",
        p."fresh_clears",
        p."sherpas",
        p."total_time_played_seconds",
        p."wfr_score",
        ROW_NUMBER() OVER (PARTITION BY cm."group_id" ORDER BY p."wfr_score" DESC) AS rn
    FROM "clan"."clan_members" cm
    JOIN "core"."player" p USING ("membership_id")
    JOIN "clan"."clan" USING ("group_id")
)
SELECT 
    rp."group_id",
    COUNT(rp."membership_id") AS "known_member_count",
    SUM(rp."clears") AS "clears",
    ROUND(AVG(rp."clears")) AS "average_clears",
    SUM(rp."fresh_clears") AS "fresh_clears",
    ROUND(AVG(rp."fresh_clears")) AS "average_fresh_clears",
    SUM(rp."sherpas") AS "sherpas",
    ROUND(AVG(rp."sherpas")) AS "average_sherpas",
    SUM(rp."total_time_played_seconds") AS "time_played_seconds",
    ROUND(AVG(rp."total_time_played_seconds")) AS "average_time_played_seconds",
    SUM(rp."wfr_score") AS "total_contest_score",
    3 * SUM(rp."wfr_score" * POWER(0.9, rp.rn - 6))::DOUBLE PRECISION / (POWER(1 + COUNT(rp."membership_id"), (1.0 / 3))) AS "weighted_contest_score"
FROM ranked_players rp
GROUP BY rp."group_id";

CREATE UNIQUE INDEX idx_clan_leaderboard_group_id ON "leaderboard"."clan_leaderboard" (group_id);

-- World first contest leaderboard
CREATE MATERIALIZED VIEW "leaderboard"."world_first_contest_leaderboard" AS
   WITH "entries" AS (
    SELECT
      "activity_id",
      ROW_NUMBER() OVER (PARTITION BY "activity_id" ORDER BY "date_completed" ASC) AS "position",
      RANK() OVER (PARTITION BY "activity_id" ORDER BY "date_completed" ASC) AS "rank",
      "instance_id",
      "date_completed",
      EXTRACT(EPOCH FROM ("date_completed" - "release_date")) AS "time_after_launch",
      "is_challenge_mode"
    FROM "definitions"."activity_version"
    INNER JOIN "definitions"."activity_definition" ON "definitions"."activity_definition"."id" = "definitions"."activity_version"."activity_id"
    INNER JOIN "definitions"."version_definition" ON "definitions"."version_definition"."id" = "definitions"."activity_version"."version_id"
    LEFT JOIN LATERAL (
      SELECT 
        "core"."instance"."instance_id", 
        "date_completed"
      FROM "core"."instance"
      LEFT JOIN "flagging"."blacklist_instance" b USING ("instance_id")
      WHERE "hash" = "definitions"."activity_version"."hash" 
        AND "completed" 
        AND b."instance_id" IS NULL
        AND "date_started" < COALESCE("contest_end", NOW())
        AND "date_completed" < "week_one_end"
      LIMIT 100000
    ) as "__inner__" ON true
    WHERE "is_world_first" = true
  )
  SELECT "entries".*, "players"."membership_ids" FROM "entries"
  LEFT JOIN LATERAL (
    SELECT JSONB_AGG("membership_id") AS "membership_ids"
    FROM "core"."instance_player"
    WHERE "core"."instance_player"."instance_id" = "entries"."instance_id"
      AND "core"."instance_player"."completed"
    LIMIT 25
  ) AS "players" ON true;

CREATE INDEX idx_world_first_contest_leaderboard_rank ON "leaderboard"."world_first_contest_leaderboard" (activity_id, position ASC);
CREATE UNIQUE INDEX idx_world_first_contest_leaderboard_instance ON "leaderboard"."world_first_contest_leaderboard" (instance_id);
CREATE INDEX idx_world_first_contest_leaderboard_membership_ids ON "leaderboard"."world_first_contest_leaderboard" USING GIN (membership_ids);

-- Team activity version leaderboard
CREATE MATERIALIZED VIEW "leaderboard"."team_activity_version_leaderboard" AS
  WITH raw AS (
    SELECT
      activity_id,
      version_id,
      instance_id,
      time_after_launch AS value,
      ROW_NUMBER() OVER (PARTITION BY activity_id, version_id ORDER BY date_completed ASC) AS position,
      RANK() OVER (PARTITION BY activity_id, version_id ORDER BY date_completed ASC) AS rank
    FROM (
      SELECT hash, activity_id, version_id, release_date_override
      FROM "definitions"."activity_version"
      WHERE version_id <> 2 -- Ignore Guided Games
      ORDER BY activity_id ASC, version_id ASC
      LIMIT 100
    ) AS activity_version
    JOIN "definitions"."activity_definition" ON activity_version.activity_id = "definitions"."activity_definition".id
    LEFT JOIN LATERAL (
      SELECT 
        "core"."instance".instance_id, 
        date_completed,
        EXTRACT(EPOCH FROM (date_completed - COALESCE(release_date_override, release_date))) AS time_after_launch 
      FROM "core"."instance"
      LEFT JOIN "flagging"."blacklist_instance" b USING ("instance_id")
      WHERE "core"."instance".hash = activity_version.hash
        AND "core"."instance".completed 
        AND b.instance_id IS NULL
      ORDER BY "core"."instance".date_completed ASC
      LIMIT 1000
    ) AS first_thousand ON true
  )
  SELECT raw.*, "players".membership_ids FROM raw
  LEFT JOIN LATERAL (
    SELECT JSONB_AGG(membership_id) AS membership_ids
    FROM "core"."instance_player"
    WHERE "core"."instance_player".instance_id = raw.instance_id
      AND "core"."instance_player".completed
    LIMIT 12
  ) as "players" ON true
  WHERE position <= 1000;

CREATE UNIQUE INDEX idx_team_activity_version_leaderboard_position ON "leaderboard"."team_activity_version_leaderboard" (activity_id ASC, version_id ASC, position ASC);
CREATE INDEX idx_team_activity_version_leaderboard_membership_id ON "leaderboard"."team_activity_version_leaderboard" USING GIN (membership_ids);

-- Individual global leaderboard
CREATE MATERIALIZED VIEW "leaderboard"."individual_global_leaderboard" AS
WITH base AS (
  SELECT
    membership_id,
    clears,
    fresh_clears,
    sherpas,
    sum_of_best,
    total_time_played_seconds,
    wfr_score,
    total_count
  FROM "cache"."_global_leaderboard_cache"
),
total_count AS (
  SELECT DISTINCT total_count::numeric AS cnt FROM base LIMIT 1
),
clears_ranked AS (
  SELECT
    membership_id,
    clears,
    ROW_NUMBER() OVER (ORDER BY clears DESC, membership_id ASC) AS clears_position,
    RANK() OVER (ORDER BY clears DESC) AS clears_rank,
    1.0 - ((RANK() OVER (ORDER BY clears DESC) - 1) / (tc.cnt - 1)) AS clears_percentile
  FROM base, total_count tc
),
fresh_clears_ranked AS (
  SELECT
    membership_id,
    fresh_clears,
    ROW_NUMBER() OVER (ORDER BY fresh_clears DESC, membership_id ASC) AS fresh_clears_position,
    RANK() OVER (ORDER BY fresh_clears DESC) AS fresh_clears_rank,
    1.0 - ((RANK() OVER (ORDER BY fresh_clears DESC) - 1) / (tc.cnt - 1)) AS fresh_clears_percentile
  FROM base, total_count tc
),
sherpas_ranked AS (
  SELECT
    membership_id,
    sherpas,
    ROW_NUMBER() OVER (ORDER BY sherpas DESC, membership_id ASC) AS sherpas_position,
    RANK() OVER (ORDER BY sherpas DESC) AS sherpas_rank,
    1.0 - ((RANK() OVER (ORDER BY sherpas DESC) - 1) / (tc.cnt - 1)) AS sherpas_percentile
  FROM base, total_count tc
),
speed_ranked AS (
  SELECT
    membership_id,
    sum_of_best AS speed,
    ROW_NUMBER() OVER (ORDER BY sum_of_best ASC, membership_id ASC) AS speed_position,
    RANK() OVER (ORDER BY sum_of_best ASC) AS speed_rank,
    1.0 - ((RANK() OVER (ORDER BY sum_of_best ASC) - 1) / (tc.cnt - 1)) AS speed_percentile
  FROM base, total_count tc
),
total_time_played_ranked AS (
  SELECT
    membership_id,
    total_time_played_seconds AS total_time_played,
    ROW_NUMBER() OVER (ORDER BY total_time_played_seconds DESC, membership_id ASC) AS total_time_played_position,
    RANK() OVER (ORDER BY total_time_played_seconds DESC) AS total_time_played_rank,
    1.0 - ((RANK() OVER (ORDER BY total_time_played_seconds DESC) - 1) / (tc.cnt - 1)) AS total_time_played_percentile
  FROM base, total_count tc
),
wfr_score_ranked AS (
  SELECT
    membership_id,
    wfr_score,
    ROW_NUMBER() OVER (ORDER BY wfr_score DESC, membership_id ASC) AS wfr_score_position,
    RANK() OVER (ORDER BY wfr_score DESC) AS wfr_score_rank,
    1.0 - ((RANK() OVER (ORDER BY wfr_score DESC) - 1) / (tc.cnt - 1)) AS wfr_score_percentile
  FROM base, total_count tc
)
SELECT
  base.membership_id,
  clears_ranked.clears,
  clears_ranked.clears_position,
  clears_ranked.clears_rank,
  clears_ranked.clears_percentile,
  fresh_clears_ranked.fresh_clears,
  fresh_clears_ranked.fresh_clears_position,
  fresh_clears_ranked.fresh_clears_rank,
  fresh_clears_ranked.fresh_clears_percentile,
  sherpas_ranked.sherpas,
  sherpas_ranked.sherpas_position,
  sherpas_ranked.sherpas_rank,
  sherpas_ranked.sherpas_percentile,
  speed_ranked.speed,
  speed_ranked.speed_position,
  speed_ranked.speed_rank,
  speed_ranked.speed_percentile,
  total_time_played_ranked.total_time_played,
  total_time_played_ranked.total_time_played_position,
  total_time_played_ranked.total_time_played_rank,
  total_time_played_ranked.total_time_played_percentile,
  wfr_score_ranked.wfr_score,
  wfr_score_ranked.wfr_score_position,
  wfr_score_ranked.wfr_score_rank,
  wfr_score_ranked.wfr_score_percentile
FROM base
JOIN clears_ranked ON base.membership_id = clears_ranked.membership_id
JOIN fresh_clears_ranked ON base.membership_id = fresh_clears_ranked.membership_id
JOIN sherpas_ranked ON base.membership_id = sherpas_ranked.membership_id
JOIN speed_ranked ON base.membership_id = speed_ranked.membership_id
JOIN total_time_played_ranked ON base.membership_id = total_time_played_ranked.membership_id
JOIN wfr_score_ranked ON base.membership_id = wfr_score_ranked.membership_id;

CREATE UNIQUE INDEX idx_global_leaderboard_membership_id ON "leaderboard"."individual_global_leaderboard" (membership_id ASC);
CREATE UNIQUE INDEX idx_global_leaderboard_clears ON "leaderboard"."individual_global_leaderboard" (clears_position ASC);
CREATE UNIQUE INDEX idx_global_leaderboard_fresh_clears ON "leaderboard"."individual_global_leaderboard" (fresh_clears_position ASC);
CREATE UNIQUE INDEX idx_global_leaderboard_sherpas ON "leaderboard"."individual_global_leaderboard" (sherpas_position ASC);
CREATE UNIQUE INDEX idx_global_leaderboard_speed ON "leaderboard"."individual_global_leaderboard" (speed_position ASC);
CREATE UNIQUE INDEX idx_global_leaderboard_total_time_played ON "leaderboard"."individual_global_leaderboard" (total_time_played_position ASC);
CREATE UNIQUE INDEX idx_global_leaderboard_wfr_score ON "leaderboard"."individual_global_leaderboard" (wfr_score_position ASC);

-- Individual raid leaderboard (per activity)
CREATE MATERIALIZED VIEW "leaderboard"."individual_raid_leaderboard" AS
WITH base AS (
  SELECT
    activity_id,
    membership_id,
    clears,
    fresh_clears,
    sherpas,
    total_time_played_seconds,
    total_count
  FROM "cache"."_individual_activity_leaderboard_cache"
  WHERE activity_id IN (
    SELECT id FROM "definitions"."activity_definition" WHERE is_raid = true
  )
),
ranked AS (
  SELECT
    activity_id,
    membership_id,
    clears,
    ROW_NUMBER() OVER (PARTITION BY activity_id ORDER BY clears DESC, membership_id ASC) AS clears_position,
    RANK() OVER (PARTITION BY activity_id ORDER BY clears DESC) AS clears_rank,
    fresh_clears,
    ROW_NUMBER() OVER (PARTITION BY activity_id ORDER BY fresh_clears DESC, membership_id ASC) AS fresh_clears_position,
    RANK() OVER (PARTITION BY activity_id ORDER BY fresh_clears DESC) AS fresh_clears_rank,
    sherpas,
    ROW_NUMBER() OVER (PARTITION BY activity_id ORDER BY sherpas DESC, membership_id ASC) AS sherpas_position,
    RANK() OVER (PARTITION BY activity_id ORDER BY sherpas DESC) AS sherpas_rank,
    total_time_played_seconds AS total_time_played,
    ROW_NUMBER() OVER (PARTITION BY activity_id ORDER BY total_time_played_seconds DESC, membership_id ASC) AS total_time_played_position,
    RANK() OVER (PARTITION BY activity_id ORDER BY total_time_played_seconds DESC) AS total_time_played_rank,
    NULLIF(total_count - 1, 0) AS count_minus_one
  FROM base
)
SELECT
  activity_id,
  membership_id,
  clears,
  clears_position,
  clears_rank,
  1.0 - ((clears_rank - 1)::numeric / count_minus_one) AS clears_percentile,
  fresh_clears,
  fresh_clears_position,
  fresh_clears_rank,
  1.0 - ((fresh_clears_rank - 1)::numeric / count_minus_one) AS fresh_clears_percentile,
  sherpas,
  sherpas_position,
  sherpas_rank,
  1.0 - ((sherpas_rank - 1)::numeric / count_minus_one) AS sherpas_percentile,
  total_time_played,
  total_time_played_position,
  total_time_played_rank,
  1.0 - ((total_time_played_rank - 1)::numeric / count_minus_one) AS total_time_played_percentile
FROM ranked;

CREATE UNIQUE INDEX idx_raid_leaderboard_activity_membership ON "leaderboard"."individual_raid_leaderboard" (activity_id ASC, membership_id ASC);
CREATE UNIQUE INDEX idx_raid_leaderboard_clears ON "leaderboard"."individual_raid_leaderboard" (activity_id ASC, clears_position ASC);
CREATE UNIQUE INDEX idx_raid_leaderboard_fresh_clears ON "leaderboard"."individual_raid_leaderboard" (activity_id ASC, fresh_clears_position ASC);
CREATE UNIQUE INDEX idx_raid_leaderboard_sherpas ON "leaderboard"."individual_raid_leaderboard" (activity_id ASC, sherpas_position ASC);
CREATE UNIQUE INDEX idx_raid_leaderboard_total_time_played ON "leaderboard"."individual_raid_leaderboard" (activity_id ASC, total_time_played_position ASC);

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
    WHERE av."activity_id" = 101
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