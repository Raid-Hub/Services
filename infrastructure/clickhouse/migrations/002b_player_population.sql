-- Player population by hour table and materialized view

CREATE TABLE IF NOT EXISTS player_population_by_hour
(
    hour DateTime,
    activity_id UInt16,
    player_count UInt32
)
ENGINE = SummingMergeTree
ORDER BY (hour, activity_id)
TTL hour + toIntervalMonth(1)
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS player_population_by_hour_mv TO player_population_by_hour AS
SELECT
    arrayJoin(arrayMap(x -> CAST(x, 'DateTime'), range(toUnixTimestamp(toStartOfHour(i.date_started)), toUnixTimestamp(i.date_completed), 3600))) AS hour,
    av.activity_id AS activity_id,
    sum(i.player_count) AS player_count
FROM instance AS i
INNER JOIN activity_version AS av ON CAST(i.hash AS Int64) = av.hash
WHERE i.player_count < 50
GROUP BY
    hour,
    activity_id;

