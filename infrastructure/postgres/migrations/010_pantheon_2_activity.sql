-- Pantheon 2.0: definition data + pantheon version leaderboard MV.

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

-- Permanent The Pantheon modes (activity 102)
INSERT INTO "definitions"."activity_definition" (
    "id", "name", "is_sunset", "is_raid", "path", "release_date", "contest_end", "week_one_end", "milestone_hash", "splash_path"
) VALUES (
    102, 'The Pantheon', false, false, 'pantheon', '2026-06-09T17:00:00Z', NULL, NULL, NULL, 'pantheon'
) ON CONFLICT ("id") DO UPDATE SET
    "name" = EXCLUDED."name",
    "path" = EXCLUDED."path",
    "release_date" = EXCLUDED."release_date",
    "splash_path" = EXCLUDED."splash_path";

-- Featured encounter rotation (activity 201)
INSERT INTO "definitions"."activity_definition" (
    "id", "name", "is_sunset", "is_raid", "path", "release_date", "contest_end", "week_one_end", "milestone_hash", "splash_path"
) VALUES (
    201, 'Pantheon Encounters', false, false, 'encounters', '2026-06-09T17:00:00Z', NULL, NULL, NULL, 'pantheon'
) ON CONFLICT ("id") DO UPDATE SET
    "name" = EXCLUDED."name",
    "path" = EXCLUDED."path",
    "release_date" = EXCLUDED."release_date",
    "splash_path" = EXCLUDED."splash_path";

-- Activity 102: permanent Pantheon modes (Customize)
INSERT INTO "definitions"."version_definition" (
    "id", "name", "associated_activity_id", "path", "is_challenge_mode"
) VALUES
    (132, 'Calus Resplendent', 102, 'calus', false),
    (133, 'Morgeth Surpassing', 102, 'morgeth', false),
    (134, 'Insurrection Prime Revolutionary', 102, 'gauntlet', false)
ON CONFLICT ("id") DO UPDATE SET
    "name" = EXCLUDED."name",
    "associated_activity_id" = EXCLUDED."associated_activity_id",
    "path" = EXCLUDED."path";

-- Activity 201: one version per featured encounter boss
INSERT INTO "definitions"."version_definition" (
    "id", "name", "associated_activity_id", "path", "is_challenge_mode"
) VALUES
    (135, 'Morgeth', 201, 'morgeth', false),
    (136, 'Insurrection Prime', 201, 'insurrection', false),
    (137, 'Warpriest', 201, 'warpriest', false),
    (138, 'Consecrated Mind', 201, 'consecrated-mind', false),
    (139, 'Argos', 201, 'argos', false),
    (140, 'Calus', 201, 'calus', false),
    (141, 'Gahlran', 201, 'gahlran', false)
ON CONFLICT ("id") DO UPDATE SET
    "name" = EXCLUDED."name",
    "associated_activity_id" = EXCLUDED."associated_activity_id",
    "path" = EXCLUDED."path";

UPDATE "definitions"."version_definition"
SET "associated_activity_id" = 102
WHERE "id" IN (132, 133, 134) AND "associated_activity_id" = 101;

DELETE FROM "definitions"."activity_version"
WHERE "hash" IN (4147455553, 153253948);

DELETE FROM "definitions"."version_definition"
WHERE "id" IN (142, 143);

-- Bungie activity hash mappings
INSERT INTO "definitions"."activity_version" (
    "hash", "activity_id", "version_id", "is_world_first"
) VALUES
    (1516551982, 102, 132, false),
    (2530656885, 102, 133, false),
    (747671496, 102, 134, false),
    (43862588, 201, 135, false),
    (206811036, 201, 136, false),
    (145874766, 201, 137, false),
    (3975235718, 201, 138, false),
    (796488315, 201, 139, false),
    (1566552947, 201, 140, false),
    (1953549041, 201, 141, false)
ON CONFLICT ("hash") DO UPDATE SET
    "activity_id" = EXCLUDED."activity_id",
    "version_id" = EXCLUDED."version_id";

-- Recreate pantheon version leaderboard MV (activities 101, 102, 201)
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
    WHERE av."activity_id" IN (101, 102, 201)
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
