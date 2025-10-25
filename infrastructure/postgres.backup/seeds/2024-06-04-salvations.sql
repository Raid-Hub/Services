-- Salvation's Edge
INSERT INTO "activity_definition" (id, name, path, release_date, contest_end, week_one_end)
    (14, 'Salvation''s Edge', 'salvationsedge', '2024-06-07 17:00:00', '2024-06-09 17:00:00', '2024-06-11 17:00:00');

INSERT INTO "version_definition" ("id", "name", "path", "associated_activity_id") VALUES
    (32, 'Contest', 'contest', NULL);

INSERT INTO "activity_hash" ("activity_id", "version_id", "hash", "is_world_first") VALUES
    -- SALVATIONS_EDGE
    (14, 1, 1541433876, false),
    -- SALVATIONS_EDGE CONTEST
    (14, 32, 2192826039, true);
    -- SALVATIONS_EDGE MASTER
    (14, 4, 4129614942, false);

UPDATE "activity_definition" SET "milestone_hash" = 540415767 WHERE id = 13;
UPDATE "activity_definition" SET "milestone_hash" = 4196566271 WHERE id = 14;
UPDATE "activity_hash" SET "release_date_override" = '2024-06-25T17:00:00Z' WHERE hash = 4129614942;

UPDATE "version_definition" SET "name" = 'Standard' WHERE "id" = 1;