CREATE TABLE player_population_by_hour
(
    `hour` DateTime,
    `activity_id` UInt16,
    `player_count` UInt32
)
ENGINE = SummingMergeTree
ORDER BY (hour, activity_id)
TTL hour + toIntervalMonth(1)
SETTINGS index_granularity = 8192

CREATE MATERIALIZED VIEW player_population_by_hour_mv TO player_population_by_hour
(
    `hour` DateTime,
    `activity_id` UInt16,
    `player_count` UInt64
)
AS SELECT
    arrayJoin(arrayMap(x -> CAST(x, 'DateTime'), range(toUnixTimestamp(toStartOfHour(i.date_started)), toUnixTimestamp(i.date_completed), 3600))) AS hour,
    hash_map.activity_id AS activity_id,
    sum(i.player_count) AS player_count
FROM default.instance AS i
INNER JOIN default.hash_map USING (hash)
WHERE i.player_count < 50
GROUP BY
    hour,
    activity_id

