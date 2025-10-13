CREATE MATERIALIZED VIEW "individual_raid_leaderboard" AS
  SELECT
    membership_id,
    activity_id,

    player_stats.clears,
    ROW_NUMBER() OVER (PARTITION BY activity_id ORDER BY player_stats.clears DESC, membership_id ASC) AS clears_position,
    RANK() OVER (PARTITION BY activity_id ORDER BY player_stats.clears DESC) AS clears_rank,

    player_stats.fresh_clears,
    ROW_NUMBER() OVER (PARTITION BY activity_id ORDER BY player_stats.fresh_clears DESC, membership_id ASC) AS fresh_clears_position,
    RANK() OVER (PARTITION BY activity_id ORDER BY player_stats.fresh_clears DESC) AS fresh_clears_rank,
    
    player_stats.sherpas,
    ROW_NUMBER() OVER (PARTITION BY activity_id ORDER BY player_stats.sherpas DESC, membership_id ASC) AS sherpas_position,
    RANK() OVER (PARTITION BY activity_id ORDER BY player_stats.sherpas DESC) AS sherpas_rank,

    player_stats.total_time_played_seconds AS total_time_played,
    ROW_NUMBER() OVER (PARTITION BY activity_id ORDER BY player_stats.total_time_played_seconds DESC, membership_id ASC) AS total_time_played_position,
    RANK() OVER (PARTITION BY activity_id ORDER BY player_stats.total_time_played_seconds DESC) AS total_time_played_rank
  FROM player_stats
  JOIN player USING (membership_id) 
  WHERE player_stats.clears > 0 AND activity_id IN (
    SELECT id FROM activity_definition WHERE is_raid = true
  )
  AND NOT player.is_private AND player.cheat_level < 2;

CREATE UNIQUE INDEX idx_individual_raid_leaderboard_membership_id ON individual_raid_leaderboard (activity_id DESC, membership_id ASC);
CREATE UNIQUE INDEX idx_individual_raid_leaderboard_clears ON individual_raid_leaderboard (activity_id DESC, clears_position ASC);
CREATE UNIQUE INDEX idx_individual_raid_leaderboard_fresh_clears ON individual_raid_leaderboard (activity_id DESC, fresh_clears_position ASC);
CREATE UNIQUE INDEX idx_individual_raid_leaderboard_sherpas ON individual_raid_leaderboard (activity_id DESC, sherpas_position ASC);
CREATE UNIQUE INDEX idx_individual_raid_leaderboard_total_time_played ON individual_raid_leaderboard (activity_id DESC, total_time_played_position ASC);

-- ALTER MATERIALIZED VIEW "individual_raid_leaderboard" OWNER TO raidhub_user;