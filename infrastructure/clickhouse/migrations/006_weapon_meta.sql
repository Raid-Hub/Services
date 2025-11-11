-- Weapon meta by hour table and materialized view

CREATE TABLE IF NOT EXISTS weapon_meta_by_hour
(
    hour DateTime,
    activity_id UInt16,
    weapon_hash UInt32,
    usage_count UInt32,
    kill_count UInt64,
    precision_kill_count UInt64
)
ENGINE = SummingMergeTree
ORDER BY (hour, activity_id, weapon_hash)
TTL hour + toIntervalMonth(1)
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS weapon_meta_by_hour_mv TO weapon_meta_by_hour AS
SELECT
    toStartOfHour(i.date_completed) AS hour,
    av.activity_id AS activity_id,
    weapon.weapon_hash AS weapon_hash,
    count(weapon) AS usage_count,
    sum(weapon.kills) AS kill_count,
    sum(weapon.precision_kills) AS precision_kill_count
FROM instance AS i
INNER JOIN activity_version AS av ON CAST(i.hash AS Int64) = av.hash
ARRAY JOIN arrayFlatten(arrayMap(p -> arrayMap(c -> c.weapons, p.characters), i.players)) AS weapon
GROUP BY
    hour,
    activity_id,
    weapon.weapon_hash;

