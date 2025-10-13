-- Cleanup migration for erroneous flags from 2025-09-16 through 2025-09-23
-- Actions:
-- 1) Preview players who have >=3 flags concentrated in Solo and/or TotalInstanceKills during the window
-- 2) Set those players' cheat_level = 0
-- 3) Delete ALL flag rows (instance-level and player-level) whose flagged_at is inside the window
-- IMPORTANT: Run the SELECTs first to verify affected rows. Take a DB backup (pg_dump) before running the UPDATE/DELETE statements.

-- Window (inclusive): 2025-09-16 00:00:00 to 2025-09-23 23:59:59 UTC
-- Solo bit index = 53, TotalInstanceKills bit index = 54 (see packages/cheat_detection/types.go)
\set start_ts '2025-09-16T00:00:00Z'
\set end_ts   '2025-09-23T23:59:59Z'

-- DRY-RUN: players with counts of Solo/TotalInstance flags in the window
SELECT
  fip.membership_id,
  SUM( ( (fip.cheat_check_bitmask & (1::bigint << 53)) <> 0 )::int ) AS solo_count,
  SUM( ( (fip.cheat_check_bitmask & (1::bigint << 54)) <> 0 )::int ) AS total_instance_count,
  COUNT(*) AS total_flag_rows,
  SUM( ( (fip.cheat_check_bitmask & (1::bigint << 53)) <> 0 )::int ) + SUM( ( (fip.cheat_check_bitmask & (1::bigint << 54)) <> 0 )::int ) AS solo_or_total_count
FROM flag_instance_player fip
WHERE fip.flagged_at >= :'start_ts'::timestamptz
  AND fip.flagged_at <= :'end_ts'::timestamptz
GROUP BY fip.membership_id
HAVING (SUM( ( (fip.cheat_check_bitmask & (1::bigint << 53)) <> 0 )::int ) + SUM( ( (fip.cheat_check_bitmask & (1::bigint << 54)) <> 0 )::int )) >= 3
ORDER BY solo_or_total_count DESC;

-- DRY-RUN: how many instance-level flags in the window (for visibility)
SELECT COUNT(*) AS instance_level_flags_in_window
FROM flag_instance fi
WHERE fi.flagged_at >= :'start_ts'::timestamptz
  AND fi.flagged_at <= :'end_ts'::timestamptz;

-- === ACTIONS ===
-- IMPORTANT: After you verify the DRY-RUN SELECTs, run the following inside a transaction. Make a DB backup first.
BEGIN;

-- 1) Capture affected membership_ids into a temp table for auditing (optional)
CREATE TEMP TABLE tmp_affected_players ON COMMIT DROP AS
SELECT fip.membership_id,
  SUM( ( (fip.cheat_check_bitmask & (1::bigint << 53)) <> 0 )::int ) AS solo_count,
  SUM( ( (fip.cheat_check_bitmask & (1::bigint << 54)) <> 0 )::int ) AS total_instance_count,
  SUM( ( (fip.cheat_check_bitmask & (1::bigint << 53)) <> 0 )::int ) + SUM( ( (fip.cheat_check_bitmask & (1::bigint << 54)) <> 0 )::int ) AS solo_or_total_count
FROM flag_instance_player fip
WHERE fip.flagged_at >= :'start_ts'::timestamptz
  AND fip.flagged_at <= :'end_ts'::timestamptz
GROUP BY fip.membership_id
HAVING (SUM( ( (fip.cheat_check_bitmask & (1::bigint << 53)) <> 0 )::int ) + SUM( ( (fip.cheat_check_bitmask & (1::bigint << 54)) <> 0 )::int )) >= 3;

-- 2) Update cheat_level = 0 for those players, return previous cheat_level for audit
CREATE TEMP TABLE tmp_player_audit ON COMMIT DROP AS
SELECT p.membership_id, p.cheat_level AS previous_cheat_level
FROM player p
JOIN tmp_affected_players t ON t.membership_id = p.membership_id;

UPDATE player p
SET cheat_level = 0
FROM tmp_affected_players t
WHERE p.membership_id = t.membership_id;

-- Return updated players for verification
SELECT p.membership_id, p.cheat_level
FROM player p
JOIN tmp_affected_players t USING (membership_id)
ORDER BY p.membership_id;

-- 3) Delete player-level flags in the window
DELETE FROM flag_instance_player
WHERE flagged_at >= :'start_ts'::timestamptz
  AND flagged_at <= :'end_ts'::timestamptz;

-- 4) Delete instance-level flags in the window
DELETE FROM flag_instance
WHERE flagged_at >= :'start_ts'::timestamptz
  AND flagged_at <= :'end_ts'::timestamptz;

-- Return counts of deletions (NOTE: PostgreSQL does not return deleted count from separate statements here; run separate SELECTs if needed)
-- You can run these for verification after COMMIT
-- SELECT COUNT(*) FROM flag_instance_player WHERE flagged_at >= :'start_ts'::timestamptz AND flagged_at <= :'end_ts'::timestamptz;
-- SELECT COUNT(*) FROM flag_instance WHERE flagged_at >= :'start_ts'::timestamptz AND flagged_at <= :'end_ts'::timestamptz;

COMMIT;

-- End of cleanup migration
-- update cheat_level
