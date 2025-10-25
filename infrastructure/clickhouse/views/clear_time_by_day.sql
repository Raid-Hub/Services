CREATE TABLE clear_time_by_day
(
    `bungie_day` Date,
    `activity_id` UInt16,
    `version_id` UInt16,
    `clear_time` AggregateFunction(quantiles(0.05, 0.1, 0.5, 0.9), UInt32)
)
ENGINE = AggregatingMergeTree()
ORDER BY (bungie_day, activity_id, version_id)
TTL bungie_day + toIntervalYear(1)
SETTINGS index_granularity = 8192;


CREATE MATERIALIZED VIEW clear_time_by_day_mv TO clear_time_by_day
AS
SELECT
    CAST(toStartOfDay(i.date_completed - toIntervalHour(17)), 'Date') AS bungie_day,
    hash_map.activity_id AS activity_id,
    hash_map.version_id AS version_id,
    quantilesState(0.05, 0.1, 0.5, 0.9)(i.duration) AS clear_time
FROM default.instance AS i
INNER JOIN default.hash_map USING (hash)
WHERE i.completed AND i.fresh
GROUP BY
    bungie_day,
    activity_id,
    version_id;
