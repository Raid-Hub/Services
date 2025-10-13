-- Materialized Views
CREATE MATERIALIZED VIEW individual_global_leaderboard AS
WITH total_count AS (
  SELECT COUNT(*)::float AS cnt
  FROM player
  WHERE clears > 0 AND NOT is_private AND cheat_level < 2
)
SELECT
  membership_id,

  clears,
  ROW_NUMBER() OVER (ORDER BY clears DESC, membership_id ASC) AS clears_position,
  RANK() OVER (ORDER BY clears DESC) AS clears_rank,
  1.0 - ((RANK() OVER (ORDER BY clears DESC) - 1) / (tc.cnt - 1)) AS clears_percentile,

  fresh_clears,
  ROW_NUMBER() OVER (ORDER BY fresh_clears DESC, membership_id ASC) AS fresh_clears_position,
  RANK() OVER (ORDER BY fresh_clears DESC) AS fresh_clears_rank,
  1.0 - ((RANK() OVER (ORDER BY fresh_clears DESC) - 1) / (tc.cnt - 1)) AS fresh_clears_percentile,

  sherpas,
  ROW_NUMBER() OVER (ORDER BY sherpas DESC, membership_id ASC) AS sherpas_position,
  RANK() OVER (ORDER BY sherpas DESC) AS sherpas_rank,
  1.0 - ((RANK() OVER (ORDER BY sherpas DESC) - 1) / (tc.cnt - 1)) AS sherpas_percentile,

  sum_of_best AS speed,
  ROW_NUMBER() OVER (ORDER BY sum_of_best ASC, membership_id ASC) AS speed_position,
  RANK() OVER (ORDER BY sum_of_best ASC) AS speed_rank,
  1.0 - ((RANK() OVER (ORDER BY sum_of_best ASC) - 1) / (tc.cnt - 1)) AS speed_percentile,

  total_time_played_seconds AS total_time_played,
  ROW_NUMBER() OVER (ORDER BY total_time_played_seconds DESC, membership_id ASC) AS total_time_played_position,
  RANK() OVER (ORDER BY total_time_played_seconds DESC) AS total_time_played_rank,
  1.0 - ((RANK() OVER (ORDER BY total_time_played_seconds DESC) - 1) / (tc.cnt - 1)) AS total_time_played_percentile,

  wfr_score,
  ROW_NUMBER() OVER (ORDER BY wfr_score DESC, membership_id ASC) AS wfr_score_position,
  RANK() OVER (ORDER BY wfr_score DESC) AS wfr_score_rank,
  1.0 - ((RANK() OVER (ORDER BY wfr_score DESC) - 1) / (tc.cnt - 1)) AS wfr_score_percentile

FROM player, total_count tc
WHERE clears > 0 AND NOT is_private AND cheat_level < 2;


CREATE UNIQUE INDEX idx_global_leaderboard_membership_id ON individual_global_leaderboard (membership_id ASC);
CREATE UNIQUE INDEX idx_global_leaderboard_clears ON individual_global_leaderboard (clears_position ASC);
CREATE UNIQUE INDEX idx_global_leaderboard_fresh_clears ON individual_global_leaderboard (fresh_clears_position ASC);
CREATE UNIQUE INDEX idx_global_leaderboard_sherpas ON individual_global_leaderboard (sherpas_position ASC);
CREATE UNIQUE INDEX idx_global_leaderboard_speed ON individual_global_leaderboard (speed_position ASC);
CREATE UNIQUE INDEX idx_global_leaderboard_total_time_played ON individual_global_leaderboard (total_time_played_position ASC);
CREATE UNIQUE INDEX idx_global_leaderboard_wfr_score ON individual_global_leaderboard (wfr_score_position ASC);

ALTER MATERIALIZED VIEW "individual_global_leaderboard" OWNER TO raidhub_user;
