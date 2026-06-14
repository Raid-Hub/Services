-- Pantheon community raid race: 5-feat Insurrection Prime Revolutionary
-- Tracks first completions within 48h of launch (2026-06-13 17:00 UTC).

CREATE MATERIALIZED VIEW "leaderboard"."team_pantheon_custom_race_leaderboard" AS
WITH race_constants AS (
    SELECT
        '2026-06-13 17:00:00+00'::timestamptz AS race_start,
        '2026-06-15 17:00:00+00'::timestamptz AS race_end,
        134::int AS version_id,
        790421403::bigint AS skull_empty_feat,
        5::int AS required_feat_count
),
eligible AS (
    SELECT
        i.instance_id,
        i.date_completed,
        EXTRACT(EPOCH FROM (i.date_completed - rc.race_start)) AS value
    FROM "core"."instance" i
    JOIN "definitions"."activity_version" av ON av.hash = i.hash
    CROSS JOIN race_constants rc
    LEFT JOIN "flagging"."blacklist_instance" b ON b.instance_id = i.instance_id
    WHERE av.version_id = rc.version_id
      AND i.completed
      AND b.instance_id IS NULL
      AND i.date_completed >= rc.race_start
      AND i.date_completed < rc.race_end
      AND (
          SELECT COUNT(DISTINCT u.skull_hash)
          FROM unnest(i.skull_hashes) AS u(skull_hash)
          INNER JOIN "definitions"."activity_feat_definition" fd ON fd.skull_hash = u.skull_hash
          WHERE u.skull_hash <> rc.skull_empty_feat
      ) = rc.required_feat_count
),
ranked AS (
    SELECT
        instance_id,
        value,
        ROW_NUMBER() OVER (ORDER BY date_completed ASC, instance_id ASC) AS position,
        RANK() OVER (ORDER BY date_completed ASC) AS rank
    FROM eligible
)
SELECT
    ranked.position,
    ranked.rank,
    ranked.value,
    ranked.instance_id,
    players.membership_ids
FROM ranked
LEFT JOIN LATERAL (
    SELECT JSONB_AGG(ip.membership_id ORDER BY ip.completed DESC, ip.time_played_seconds DESC) AS membership_ids
    FROM "core"."instance_player" ip
    WHERE ip.instance_id = ranked.instance_id
    LIMIT 12
) AS players ON true;

CREATE UNIQUE INDEX idx_team_pantheon_custom_race_leaderboard_position
    ON "leaderboard"."team_pantheon_custom_race_leaderboard" (position ASC);
CREATE UNIQUE INDEX idx_team_pantheon_custom_race_leaderboard_instance
    ON "leaderboard"."team_pantheon_custom_race_leaderboard" (instance_id);
CREATE INDEX idx_team_pantheon_custom_race_leaderboard_membership_ids
    ON "leaderboard"."team_pantheon_custom_race_leaderboard" USING GIN (membership_ids);
