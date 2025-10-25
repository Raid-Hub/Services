
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
        AND "date_started" < COALESCE("contest_end", NOW())
        AND "date_completed" < "week_one_end"
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
    LIMIT 25
  ) AS "players" ON true;

CREATE INDEX idx_world_first_contest_leaderboard_rank ON world_first_contest_leaderboard (activity_id, position ASC);
CREATE UNIQUE INDEX idx_world_first_contest_leaderboard_instance ON world_first_contest_leaderboard (instance_id);
CREATE INDEX idx_world_first_contest_leaderboard_membership_ids ON world_first_contest_leaderboard USING GIN (membership_ids);

ALTER MATERIALIZED VIEW "world_first_contest_leaderboard" OWNER TO raidhub_user;
