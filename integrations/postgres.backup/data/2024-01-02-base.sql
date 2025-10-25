INSERT INTO "class_definition" ("hash", "name") VALUES 
    (671679327, 'Hunter'),
    (2271682572, 'Warlock'),
    (3655393761, 'Titan');

INSERT INTO "season" ("id", "short_name", "long_name", "dlc", "start_date") VALUES
    (1, 'Red War', 'Red War', 'Vanilla', '2017-09-06 09:00:00Z'),
    (2, 'Curse of Osiris', 'Curse of Osiris', 'Curse of Osiris', '2017-12-05 17:00:00Z'),
    (3, 'Warmind', 'Warmind', 'Warmind', '2018-05-08 18:00:00Z'),
    (4, 'Outlaw', 'Season of the Outlaw', 'Forsaken', '2018-09-04 17:00:00Z'),
    (5, 'Forge', 'Season of the Forge', 'Forsaken', '2018-11-27 17:00:00Z'),
    (6, 'Drifter', 'Season of the Drifter', 'Forsaken', '2019-03-05 17:00:00Z'),
    (7, 'Opulence', 'Season of Opulence', 'Forsaken', '2019-06-04 17:00:00Z'),
    (8, 'Undying', 'Season of the Undying', 'Shadowkeep', '2019-10-01 17:00:00Z'),
    (9, 'Dawn', 'Season of Dawn', 'Shadowkeep', '2019-12-10 17:00:00Z'),
    (10, 'Worthy', 'Season of the Worthy', 'Shadowkeep', '2020-03-10 17:00:00Z'),
    (11, 'Arrivals', 'Season of Arrivals', 'Shadowkeep', '2020-06-09 17:00:00Z'),
    (12, 'Hunt', 'Season of the Hunt', 'Beyond Light', '2020-11-10 17:00:00Z'),
    (13, 'Chosen', 'Season of the Chosen', 'Beyond Light', '2021-02-09 17:00:00Z'),
    (14, 'Splicer', 'Season of the Splicer', 'Beyond Light', '2021-05-11 17:00:00Z'),
    (15, 'Lost', 'Season of the Lost', 'Beyond Light', '2021-08-24 17:00:00Z'),
    (16, 'Risen', 'Season of the Risen', 'The Witch Queen', '2022-02-22 17:00:00Z'),
    (17, 'Haunted', 'Season of the Haunted', 'The Witch Queen', '2022-05-24 17:00:00Z'),
    (18, 'Plunder', 'Season of Plunder', 'The Witch Queen', '2022-08-23 17:00:00Z'),
    (19, 'Seraph', 'Season of the Seraph', 'The Witch Queen', '2022-12-06 17:00:00Z'),
    (20, 'Defiance', 'Season of Defiance', 'Lightfall', '2023-02-28 17:00:00Z'),
    (21, 'Deep', 'Season of the Deep', 'Lightfall', '2023-05-23 17:00:00Z'),
    (22, 'Witch', 'Season of the Witch', 'Lightfall', '2023-08-22 17:00:00Z'),
    (23, 'Wish', 'Season of the Wish', 'Lightfall', '2023-11-28 17:00:00Z'),
    (24, 'Echoes', 'Episode: Echoes', 'The Final Shape', '2024-06-04 17:00:00Z'),
    (25, 'Revenant', 'Episode: Revenant', 'The Final Shape', '2024-10-08 17:00:00Z');


-- Insert Raid data
INSERT INTO "activity_definition" (id, name, is_sunset, is_raid, path, release_date, contest_end, week_one_end, milestone_hash)
VALUES
    (1, 'Leviathan', true, true, 'leviathan', '2017-09-13 17:00:00', NULL, '2017-09-19 17:00:00', NULL),
    (2, 'Eater of Worlds', true, true, 'eaterofworlds', '2017-12-08 18:00:00', NULL, '2017-12-12 17:00:00', NULL),
    (3, 'Spire of Stars', true, true, 'spireofstars', '2018-05-11 17:00:00', NULL, '2018-05-15 17:00:00', NULL),
    (4, 'Last Wish', false, true, 'lastwish', '2018-09-14 17:00:00', NULL, '2018-09-18 17:00:00', 3181387331),
    (5, 'Scourge of the Past', true, true, 'scourgeofthepast', '2018-12-07 17:00:00', NULL, '2018-12-11 17:00:00', NULL),
    (6, 'Crown of Sorrow', true, true, 'crownofsorrow', '2019-06-04 23:00:00', '2019-06-05 23:00:00', '2019-06-11 17:00:00', NULL),
    (7, 'Garden of Salvation', false, true, 'gardenofsalvation', '2019-10-05 17:00:00', '2019-10-06 17:00:00', '2019-10-08 17:00:00', 2712317338),
    (8, 'Deep Stone Crypt', false, true, 'deepstonecrypt', '2020-11-21 18:00:00', '2020-11-22 18:00:00', '2020-11-24 17:00:00', 2712317338),
    (9, 'Vault of Glass', false, true, 'vaultofglass', '2021-05-22 17:00:00', '2021-05-23 17:00:00', '2021-05-25 17:00:00', 1888320892),
    (10, 'Vow of the Disciple', false, true, 'vowofthedisciple', '2022-03-05 18:00:00', '2022-03-07 18:00:00', '2022-03-08 17:00:00', 2136320298),
    (11, 'King''s Fall', false, true, 'kingsfall', '2022-08-26 17:00:00', '2022-08-27 17:00:00', '2022-08-30 17:00:00', 292102995),
    (12, 'Root of Nightmares', false, true, 'rootofnightmares', '2023-03-10 17:00:00', '2023-03-12 17:00:00', '2023-03-14 17:00:00', 3699252268),
    (13, 'Crota''s End', false, true, 'crotasend', '2023-09-01 17:00:00', '2023-09-03 17:00:00', '2023-09-05 17:00:00', NULL),
    (101, 'The Pantheon', false, false, 'pantheon', '2024-04-30 17:00:00', NULL, NULL, NULL);

-- Insert Version data
INSERT INTO "version_definition" ("id", "name", "path", "associated_activity_id") VALUES
    (1, 'Normal', 'normal', NULL),
    (2, 'Guided Games', 'guided', NULL),
    (3, 'Prestige', 'prestige', NULL),
    (4, 'Master', 'master', NULL),
    (32, 'Contest', 'contest', NULL),
    (64, 'Tempo''s Edge', 'challenge', 9),
    (65, 'Regicide', 'challenge', 11),
    (66, 'Superior Swordplay', 'challenge', 13);

-- Insert RaidHash data
INSERT INTO "activity_hash" ("activity_id", "version_id", "hash", "is_world_first")
    -- LEVIATHAN
    (1, 1, 2693136600, false),
    (1, 1, 2693136601, true),
    (1, 1, 2693136602, false),
    (1, 1, 2693136603, false),
    (1, 1, 2693136604, false),
    (1, 1, 2693136605, false),
    -- LEVIATHAN GUIDEDGAMES
    (1, 2, 89727599, false),
    (1, 2, 287649202, false),
    (1, 2, 1699948563, false),
    (1, 2, 1875726950, false),
    (1, 2, 3916343513, false),
    (1, 2, 4039317196, false),
    -- LEVIATHAN PRESTIGE
    (1, 3, 417231112, false),
    (1, 3, 508802457, false),
    (1, 3, 757116822, false),
    (1, 3, 771164842, false),
    (1, 3, 1685065161, false),
    (1, 3, 1800508819, false),
    (1, 3, 2449714930, false),
    (1, 3, 3446541099, false),
    (1, 3, 4206123728, false),
    (1, 3, 3912437239, false),
    (1, 3, 3879860661, false),
    (1, 3, 3857338478, false),
    -- EATER_OF_WORLDS
    (2, 1, 3089205900, true),
    -- EATER_OF_WORLDS GUIDEDGAMES
    (2, 2, 2164432138, false),
    -- EATER_OF_WORLDS PRESTIGE
    (2, 3, 809170886, false),
    -- SPIRE_OF_STARS
    (3, 1, 119944200, true),
    -- SPIRE_OF_STARS GUIDEDGAMES
    (3, 2, 3004605630, false),
    -- SPIRE_OF_STARS PRESTIGE
    (3, 3, 3213556450, false),
    -- LAST_WISH
    (4, 1, 2122313384, true),
    (4, 1, 2214608157, false),
    -- LAST_WISH GUIDEDGAMES
    (4, 2, 1661734046, false),
    -- SCOURGE_OF_THE_PAST
    (5, 1, 548750096, true),
    -- SCOURGE_OF_THE_PAST GUIDEDGAMES
    (5, 2, 2812525063, false),
    -- CROWN_OF_SORROW
    (6, 1, 3333172150, true),
    -- CROWN_OF_SORROW GUIDEDGAMES
    (6, 2, 960175301, false),
    -- GARDEN_OF_SALVATION
    (7, 1, 2659723068, true),
    (7, 1, 3458480158, false),
    (7, 1, 1042180643, false),
    -- GARDEN_OF_SALVATION GUIDEDGAMES
    (7, 2, 2497200493, false),
    (7, 2, 3845997235, false),
    (7, 2, 3823237780, false),
    -- DEEP_STONE_CRYPT
    (8, 1, 910380154, true),
    -- DEEP_STONE_CRYPT GUIDEDGAMES
    (8, 2, 3976949817, false),
    -- VAULT_OF_GLASS
    (9, 1, 3881495763, false),
    -- VAULT_OF_GLASS GUIDEDGAMES
    (9, 2, 3711931140, false),
    -- VAULT_OF_GLASS CHALLENGE_VOG
    (9, 64, 1485585878, true),
    -- VAULT_OF_GLASS MASTER
    (9, 4, 1681562271, false),
    (9, 4, 3022541210, false),
    -- VOW_OF_THE_DISCIPLE
    (10, 1, 1441982566, true),
    (10, 1, 2906950631, false),
    -- VOW_OF_THE_DISCIPLE GUIDEDGAMES
    (10, 2, 4156879541, false),
    -- VOW_OF_THE_DISCIPLE MASTER
    (10, 4, 4217492330, false),
    (10, 4, 3889634515, false),
    -- KINGS_FALL
    (11, 1, 1374392663, false),
    -- KINGS_FALL GUIDEDGAMES
    (11, 2, 2897223272, false),
    -- KINGS_FALL CHALLENGE_KF
    (11, 65, 1063970578, true),
    -- KINGS_FALL MASTER
    (11, 4, 2964135793, false),
    (11, 4, 3257594522, false),
    -- ROOT_OF_NIGHTMARES
    (12, 1, 2381413764, true),
    -- ROOT_OF_NIGHTMARES GUIDEDGAMES
    (12, 2, 1191701339, false),
    -- ROOT_OF_NIGHTMARES MASTER
    (12, 4, 2918919505, false),
    -- CROTAS_END
    (13, 1, 4179289725, false),
    (13, 1, 107319834, false),
    -- CROTAS_END GUIDEDGAMES
    (13, 2, 4103176774, false),
    -- CROTAS_END CHALLENGE_CROTA
    (13, 66, 156253568, true),
    -- CROTAS_END MASTER
    (13, 4, 1507509200, false);

UPDATE activity_hash SET release_date_override = '2017-10-18 17:00:00' WHERE activity_id = 1 AND version_id = 3;
UPDATE activity_hash SET release_date_override = '2018-07-17 17:00:00' WHERE activity_id = 2 AND version_id = 3;
UPDATE activity_hash SET release_date_override = '2018-07-18 17:00:00' WHERE activity_id = 3 AND version_id = 3;

UPDATE activity_hash SET release_date_override = '2021-07-06 17:00:00' WHERE activity_id = 9 AND version_id = 4;
UPDATE activity_hash SET release_date_override = '2022-04-19 17:00:00' WHERE activity_id = 10 AND version_id = 4;
UPDATE activity_hash SET release_date_override = '2022-09-20 17:00:00' WHERE activity_id = 11 AND version_id = 4;
UPDATE activity_hash SET release_date_override = '2023-03-28 17:00:00' WHERE activity_id = 12 AND version_id = 4;
UPDATE activity_hash SET release_date_override = '2023-09-21 17:00:00' WHERE activity_id = 13 AND version_id = 4;