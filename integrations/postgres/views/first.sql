
CREATE MATERIALIZED VIEW "team_activity_version_leaderboard" AS
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
      FROM activity_version
      WHERE version_id <> 2 -- Ignore Guided Games
      ORDER BY activity_id ASC, version_id ASC
      LIMIT 100
    ) AS activity_version
    JOIN activity_definition ON activity_version.activity_id = activity_definition.id
    LEFT JOIN LATERAL (
      SELECT 
        instance.instance_id, 
        date_completed,
        EXTRACT(EPOCH FROM (date_completed - COALESCE(release_date_override, release_date))) AS time_after_launch 
      FROM instance
      LEFT JOIN blacklist_instance b USING ("instance_id")
      WHERE instance.hash = activity_version.hash
        AND instance.completed 
        AND b.instance_id IS NULL
      ORDER BY instance.date_completed ASC
      LIMIT 1000
    ) AS first_thousand ON true
  )
  SELECT raw.*, "players".membership_ids FROM raw
  LEFT JOIN LATERAL (
    SELECT JSONB_AGG(membership_id) AS membership_ids
    FROM instance_player
    WHERE instance_player.instance_id = raw.instance_id
      AND instance_player.completed
    LIMIT 12
  ) as "players" ON true
  WHERE position <= 1000;

CREATE UNIQUE INDEX idx_team_activity_version_leaderboard_position ON team_activity_version_leaderboard (activity_id ASC, version_id ASC, position ASC);
CREATE INDEX idx_team_activity_version_leaderboard_membership_id ON team_activity_version_leaderboard USING GIN (membership_ids);

ALTER MATERIALIZED VIEW "team_activity_version_leaderboard" OWNER TO raidhub_user;