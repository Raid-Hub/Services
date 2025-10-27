-- Clear time by day table and materialized view

CREATE TABLE IF NOT EXISTS clear_time_by_day
(
    bungie_day Date,
    activity_id UInt16,
    version_id UInt16,
    clear_time AggregateFunction(quantiles(0.05, 0.1, 0.5, 0.9), UInt32)
)
ENGINE = AggregatingMergeTree
ORDER BY (bungie_day, activity_id, version_id)
TTL bungie_day + toIntervalYear(1)
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS clear_time_by_day_mv TO clear_time_by_day AS
SELECT
    CAST(toStartOfDay(i.date_completed - toIntervalHour(17)), 'Date') AS bungie_day,
    av.activity_id AS activity_id,
    av.version_id AS version_id,
    quantilesState(0.05, 0.1, 0.5, 0.9)(i.duration) AS clear_time
FROM instance AS i
INNER JOIN activity_version AS av ON CAST(i.hash AS Int64) = av.hash
WHERE i.completed AND i.fresh
GROUP BY
    bungie_day,
    activity_id,
    version_id;

