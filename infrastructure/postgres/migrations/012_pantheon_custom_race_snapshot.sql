-- Freeze the Pantheon community raid race leaderboard (Insurrection Prime Revolutionary).
--
-- Prod applies manually in two phases (companion Raid-Hub/API#150):
--   Phase 1 — CREATE TABLE + INSERT ... SELECT from team_pantheon_custom_race_leaderboard
--   Phase 2 — DROP MATERIALIZED VIEW (after API reads pantheon_custom_race_snapshot)
-- CI/local: run the full file.

-- =============================================================================
-- Phase 1 — snapshot table (run before API deploy)
-- =============================================================================

CREATE TABLE "leaderboard"."pantheon_custom_race_snapshot" (
    "position" INT NOT NULL PRIMARY KEY,
    "rank" INT NOT NULL,
    "value" DOUBLE PRECISION NOT NULL,
    "instance_id" BIGINT NOT NULL UNIQUE
);

INSERT INTO "leaderboard"."pantheon_custom_race_snapshot" ("position", "rank", "value", "instance_id")
SELECT
    "position",
    "rank",
    "value",
    "instance_id"
FROM "leaderboard"."team_pantheon_custom_race_leaderboard"
ORDER BY "position" ASC;

-- =============================================================================
-- Phase 2 — drop live MV (run after API deploy)
-- =============================================================================

DROP MATERIALIZED VIEW IF EXISTS "leaderboard"."team_pantheon_custom_race_leaderboard";
