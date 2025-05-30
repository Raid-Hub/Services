
CREATE MATERIALIZED VIEW "world_first_contest_leaderboard" AS
   WITH "entries" AS (
    SELECT
      "activity_id",
      ROW_NUMBER() OVER (PARTITION BY "activity_id" ORDER BY "date_completed" ASC) AS "position",
      RANK() OVER (PARTITION BY "activity_id" ORDER BY "date_completed" ASC) AS "rank",
      "instance_id",
      "date_completed",
      EXTRACT(EPOCH FROM ("date_completed" - "release_date")) AS "time_after_launch",
      "is_challenge_mode"
    FROM "activity_version"
    INNER JOIN "activity_definition" ON "activity_definition"."id" = "activity_version"."activity_id"
    INNER JOIN "version_definition" ON "version_definition"."id" = "activity_version"."version_id"
    LEFT JOIN LATERAL (
      SELECT 
        "instance"."instance_id", 
        "date_completed"
      FROM "instance"
      LEFT JOIN "blacklist_instance" b USING ("instance_id")
      WHERE "hash" = "activity_version"."hash" 
        AND "completed" 
        AND b."instance_id" IS NULL
        AND "date_completed" < COALESCE("contest_end", "week_one_end")
      LIMIT 100000
    ) as "__inner__" ON true
    WHERE "is_world_first" = true
  )
  SELECT "entries".*, "players"."membership_ids" FROM "entries"
  LEFT JOIN LATERAL (
    SELECT JSONB_AGG("membership_id") AS "membership_ids"
    FROM "instance_player"
    WHERE "instance_player"."instance_id" = "entries"."instance_id"
      AND "instance_player"."completed"
    LIMIT 12
  ) AS "players" ON true;

CREATE INDEX idx_world_first_contest_leaderboard_rank ON world_first_contest_leaderboard (activity_id, position ASC);
CREATE UNIQUE INDEX idx_world_first_contest_leaderboard_instance ON world_first_contest_leaderboard (instance_id);
CREATE INDEX idx_world_first_contest_leaderboard_membership_ids ON world_first_contest_leaderboard USING GIN (membership_ids);

ALTER MATERIALIZED VIEW "world_first_contest_leaderboard" OWNER TO raidhub_user;

CREATE MATERIALIZED VIEW "world_first_player_rankings" AS 
WITH unnested_entries AS (
    SELECT
        world_first_contest_leaderboard.*,
        jsonb_array_elements(membership_ids)::bigint AS membership_id
    FROM
        world_first_contest_leaderboard
), tmp AS (
    SELECT DISTINCT ON (membership_id, activity_id)
        membership_id,
        ((1 / SQRT(rank)) * POWER(1.25, activity_id - 1)) as score
    FROM unnested_entries
    ORDER BY membership_id, activity_id, rank ASC
)
SELECT
    membership_id,
    SUM(score) AS score,
    RANK() OVER (ORDER BY SUM(score) DESC) AS rank,
    ROW_NUMBER() OVER (ORDER BY SUM(score) DESC) AS position
FROM tmp
JOIN player USING (membership_id)
WHERE cheat_level < 2
GROUP BY membership_id
ORDER BY rank ASC;

CREATE UNIQUE INDEX idx_world_first_player_ranking_membership_id ON world_first_player_rankings (membership_id);
CREATE INDEX idx_world_first_player_ranking_position ON world_first_player_rankings (position ASC);

ALTER MATERIALIZED VIEW "world_first_player_rankings" OWNER TO raidhub_user;

CREATE MATERIALIZED VIEW "clan_leaderboard" AS (
    WITH
    "ranked_scores" AS (
        SELECT 
            cm."membership_id",
            cm."group_id",
            wpr."score",
            ROW_NUMBER() OVER (PARTITION BY cm."group_id" ORDER BY wpr."score" DESC) AS "intra_clan_ranking"
        FROM "clan_members" cm
        LEFT JOIN "world_first_player_rankings" wpr ON cm."membership_id" = wpr."membership_id"
    )
    SELECT 
        "group_id",
        COUNT("membership_id") AS "known_member_count",
        SUM("p"."clears") AS "clears",
        ROUND(AVG("p"."clears")) AS "average_clears",
        SUM("p"."fresh_clears") AS "fresh_clears",
        ROUND(AVG("p"."fresh_clears")) AS "average_fresh_clears",
        SUM("p"."sherpas") AS "sherpas",
        ROUND(AVG("p"."sherpas")) AS "average_sherpas",
        SUM("p"."total_time_played_seconds") AS "time_played_seconds",
        ROUND(AVG("p"."total_time_played_seconds")) AS "average_time_played_seconds",
        COALESCE(SUM(rs."score"), 0) AS "total_contest_score",
        COALESCE(SUM(rs."score" * POWER(0.9, rs."intra_clan_ranking" - 6))::DOUBLE PRECISION / (POWER(1 + COUNT("membership_id"), (1 / 3))), 0) AS "weighted_contest_score"
    FROM "clan_members" cm
    JOIN "player" p USING ("membership_id")
    JOIN "ranked_scores" rs USING ("group_id", "membership_id")
    JOIN "clan" USING ("group_id")
    GROUP BY "group_id", "clan"."name"
);
CREATE UNIQUE INDEX idx_clan_leaderboard_group_id ON clan_leaderboard (group_id);

ALTER MATERIALIZED VIEW "clan_leaderboard" OWNER TO raidhub_user;

