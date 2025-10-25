  -- NEW: The Desert Perpetual
INSERT INTO "activity_definition" (id, name, path, release_date, contest_end, week_one_end) VALUES
    (15, 'The Desert Perpetual', 'desertperpetual', '2025-07-19 17:00:00', '2025-07-21 17:00:00', '2025-07-22 17:00:00');

INSERT INTO "activity_version" ("activity_id", "version_id", "hash", "is_world_first", "is_contest_eligible") VALUES
    -- NORMAL
    (15, 1, 1044919065, false),
    -- CONTEST
    (15, 32, 3896382790, true);

INSERT INTO "season" ("id", "short_name", "long_name", "dlc", "start_date") VALUES
    (27, 'Reclamation', 'Season: Reclamation', 'The Edge of Fate', '2025-07-15 17:00:00Z'),
    (28, 'Lawless', 'Season: Lawless', 'Renegades', '2025-12-02 17:00:00Z');

ALTER TABLE "activity_version" ADD COLUMN "is_contest_eligible" BOOLEAN NOT NULL DEFAULT false;
UPDATE "activity_version" SET "is_contest_eligible" = true WHERE "is_world_first";
UPDATE "activity_version" SET "is_contest_eligible" = true WHERE "hash" IN (
    -- reprised raids
    4179289725, 1374392663, 3881495763
);