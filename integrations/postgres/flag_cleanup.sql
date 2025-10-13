BEGIN;

WITH duplicates AS (
  SELECT instance_id, cheat_check_version,
         ROW_NUMBER() OVER (
           PARTITION BY instance_id, cheat_check_bitmask, ROUND(cheat_probability::numeric, 2)
           ORDER BY flagged_at DESC
         ) AS rn
  FROM flag_instance
)
DELETE FROM flag_instance f
USING duplicates d
WHERE f.instance_id = d.instance_id
  AND f.cheat_check_version = d.cheat_check_version
  AND d.rn > 1;


WITH duplicates AS (
  SELECT instance_id, membership_id, cheat_check_version,
         ROW_NUMBER() OVER (
           PARTITION BY instance_id, membership_id, cheat_check_bitmask, ROUND(cheat_probability::numeric, 2)
           ORDER BY flagged_at DESC
         ) AS rn
  FROM flag_instance_player
)
DELETE FROM flag_instance_player f
USING duplicates d
WHERE f.instance_id = d.instance_id
  AND f.membership_id = d.membership_id
  AND f.cheat_check_version = d.cheat_check_version
  AND d.rn > 1;

COMMIT;