INSERT INTO "activity_definition" (id, name, path, release_date, contest_end, week_one_end) VALUES
    (16, 'The Desert Perpetual (Epic)', 'desertperpetualepic', '2025-09-27 17:00:00', '2025-09-29 17:00:00', '2025-09-30 17:00:00');

INSERT INTO "activity_version" ("activity_id", "version_id", "hash", "is_world_first", "is_contest_eligible") VALUES
    -- NORMAL

    (16, 1, 3817322389, false),
    -- CONTEST
    (16, 32, 2586252122, true);
