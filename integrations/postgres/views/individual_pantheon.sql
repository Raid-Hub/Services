
CREATE MATERIALIZED VIEW "individual_pantheon_version_leaderboard" AS
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
  FROM (
    WITH hashes AS (
        SELECT hash FROM activity_version WHERE activity_id = 101
    )
    SELECT 
        "lateral".membership_id,
        version_id,
        COUNT(*) AS clears,
        SUM(CASE WHEN "lateral".fresh THEN 1 ELSE 0 END) AS fresh_clears,
        SUM("lateral".score) AS score
    FROM hashes
    JOIN activity_version USING (hash)
    LEFT JOIN LATERAL (
        SELECT 
            membership_id,
            fresh,
            score
        FROM instance_player 
        JOIN instance USING (instance_id)
        JOIN player USING (membership_id)
        WHERE instance_player.completed
            AND activity_version.hash = instance.hash
            AND NOT player.is_private AND player.cheat_level < 2
    ) AS "lateral" ON TRUE
     GROUP BY membership_id, version_id
  ) as foo
  WHERE clears > 0;

CREATE UNIQUE INDEX idx_individual_pantheon_version_leaderboard_membership_id ON individual_pantheon_version_leaderboard (version_id ASC, membership_id ASC);
CREATE UNIQUE INDEX idx_individual_pantheon_version_leaderboard_clears ON individual_pantheon_version_leaderboard (version_id ASC, clears_position ASC);
CREATE UNIQUE INDEX idx_individual_pantheon_version_leaderboard_fresh_clears ON individual_pantheon_version_leaderboard (version_id ASC, fresh_clears_position ASC);
CREATE UNIQUE INDEX idx_individual_pantheon_version_leaderboard_score ON individual_pantheon_version_leaderboard (version_id ASC, score_position ASC);
