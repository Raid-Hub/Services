-- Pantheon 2.0: definition data + pantheon version leaderboard MV for activity 102.

-- Season 29 (Monument of Triumph / update 9.7.0)
INSERT INTO "definitions"."season" (
    "id", "short_name", "long_name", "dlc", "start_date"
) VALUES (
    29, 'Triumph', 'Monument of Triumph', 'Monument of Triumph', '2026-06-09T17:00:00Z'
) ON CONFLICT ("id") DO UPDATE SET
    "short_name" = EXCLUDED."short_name",
    "long_name" = EXCLUDED."long_name",
    "dlc" = EXCLUDED."dlc",
    "start_date" = EXCLUDED."start_date";

-- Sunset historical The Pantheon (activity 101)
UPDATE "definitions"."activity_definition"
SET "is_sunset" = true
WHERE "id" = 101;

-- Permanent The Pantheon (activity 102)
INSERT INTO "definitions"."activity_definition" (
    "id", "name", "is_sunset", "is_raid", "path", "release_date", "contest_end", "week_one_end", "milestone_hash", "splash_path"
) VALUES (
    102, 'The Pantheon', false, false, 'pantheon', '2026-06-09T17:00:00Z', NULL, NULL, NULL, 'pantheon'
) ON CONFLICT ("id") DO UPDATE SET
    "name" = EXCLUDED."name",
    "path" = EXCLUDED."path",
    "release_date" = EXCLUDED."release_date",
    "splash_path" = EXCLUDED."splash_path";

-- Pantheon 2.0 versions on activity 102
INSERT INTO "definitions"."version_definition" (
    "id", "name", "associated_activity_id", "path", "is_challenge_mode"
) VALUES
    (132, 'Calus Resplendent', 102, 'calus', false),
    (133, 'Morgeth Surpassing', 102, 'morgeth', false),
    (134, 'Insurrection Prime Revolutionary', 102, 'gauntlet', false),
    (135, 'Morgeth Encore', 102, 'morgeth-encore', false),
    (136, 'Insurrection Prime Encore', 102, 'insurrection-encore', false),
    (137, 'Warpriest Encore', 102, 'warpriest', false),
    (138, 'Warpriest Atraks', 102, 'warpriest-atraks', false),
    (139, 'Consecrated Mind Encore', 102, 'consecrated-mind', false),
    (140, 'Argos Reprise', 102, 'argos', false),
    (141, 'Calus Atraks Reprise', 102, 'calus-atraks', false),
    (142, 'Calus Reprise', 102, 'calus-reprise', false),
    (143, 'Gahlran Reprise', 102, 'gahlran', false)
ON CONFLICT ("id") DO UPDATE SET
    "name" = EXCLUDED."name",
    "associated_activity_id" = EXCLUDED."associated_activity_id",
    "path" = EXCLUDED."path";

UPDATE "definitions"."version_definition"
SET "associated_activity_id" = 102
WHERE "id" BETWEEN 132 AND 143 AND "associated_activity_id" = 101;

-- Bungie activity hash mappings (manifest hashes for activity 102)
INSERT INTO "definitions"."activity_version" (
    "hash", "activity_id", "version_id", "is_world_first"
) VALUES
    (1516551982, 102, 132, false),
    (2530656885, 102, 133, false),
    (747671496, 102, 134, false),
    (43862588, 102, 135, false),
    (206811036, 102, 136, false),
    (145874766, 102, 137, false),
    (4147455553, 102, 138, false),
    (3975235718, 102, 139, false),
    (796488315, 102, 140, false),
    (153253948, 102, 141, false),
    (1566552947, 102, 142, false),
    (1953549041, 102, 143, false)
ON CONFLICT ("hash") DO UPDATE SET
    "activity_id" = EXCLUDED."activity_id",
    "version_id" = EXCLUDED."version_id";

-- Recreate pantheon version leaderboard MV (now includes activity 102)
DROP MATERIALIZED VIEW IF EXISTS "leaderboard"."individual_pantheon_version_leaderboard";

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
    WHERE av."activity_id" IN (101, 102)
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
