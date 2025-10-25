-- Pantheon
INSERT INTO "activity_definition" (id, name, is_sunset, is_raid, path, release_date)
VALUES (101, 'The Pantheon', false, false, 'pantheon', '2024-04-30 17:00:00');

INSERT INTO "version_definition" ("id", "name", "associated_activity_id", "path") VALUES
    (128, 'Atraks Sovereign', 101, 'atraks'),
    (129, 'Oryx Exalted', 101, 'oryx'),
    (130, 'Rhulk Indomitable', 101, 'rhulk'),
    (131, 'Nezarec Sublime', 101, 'nezarec');

INSERT INTO "activity_hash" ("hash", "activity_id", "version_id", "release_date_override") VALUES 
    -- Atraks Sovereign
    (4169648179, 101, 128, '2024-04-30 17:00:00'),
    -- Oryx Exalted
    (4169648176, 101, 129, '2024-05-07 17:00:00'),
    -- Rhulk Indomitable
    (4169648177, 101, 130, '2024-05-14 17:00:00'),
    -- Nezarec Sublime
    (4169648182, 101, 131, '2024-05-21 17:00:00');