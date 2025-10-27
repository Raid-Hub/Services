-- Create PostgreSQL engine tables for dictionary lookups

CREATE TABLE IF NOT EXISTS activity_version
(
    hash Int64,
    activity_id UInt16,
    version_id UInt16
)
ENGINE = PostgreSQL(postgres_creds, 'definitions', 'activity_version');

CREATE TABLE IF NOT EXISTS activity
(
    id Int32,
    name String,
    is_sunset UInt8,
    is_raid UInt8,
    path String,
    release_date DateTime,
    contest_end Nullable(DateTime),
    week_one_end DateTime,
    milestone_hash UInt32,
    day_one_end DateTime
)
ENGINE = PostgreSQL(postgres_creds, 'definitions', 'activity_definition');

CREATE TABLE IF NOT EXISTS version
(
    id Int32,
    name String
)
ENGINE = PostgreSQL(postgres_creds, 'definitions', 'version_definition');

CREATE TABLE IF NOT EXISTS weapon_definition
(
    hash Int64,
    name String,
    icon_path String,
    element String,
    slot String,
    ammo_type String,
    rarity String,
    weapon_type String
)
ENGINE = PostgreSQL(postgres_creds, 'definitions', 'weapon_definition');

CREATE TABLE IF NOT EXISTS player
(
    membership_id UInt64,
    membership_type Int32,
    icon_path String,
    display_name String,
    bungie_global_display_name String,
    bungie_global_display_name_code String,
    bungie_name String,
    last_seen DateTime,
    sherpas Int32,
    sum_of_best Nullable(Int32)
)
ENGINE = PostgreSQL(postgres_creds, 'definitions', 'player');
