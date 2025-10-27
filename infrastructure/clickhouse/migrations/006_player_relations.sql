-- Player relation weights table and materialized view

CREATE TABLE IF NOT EXISTS player_relation_weights_bidirectional
(
    membership_id UInt64,
    related_membership_id UInt64,
    weight UInt64
)
ENGINE = SummingMergeTree
PRIMARY KEY (membership_id, related_membership_id)
ORDER BY (membership_id, related_membership_id)
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS player_relation_weights_mv TO player_relation_weights_bidirectional AS
SELECT
    p1.1 AS membership_id,
    p2.1 AS related_membership_id,
    (least(p1.2, p2.2) * ((p1.3) + 1)) * ((p2.3) + 1) AS weight
FROM instance
ARRAY JOIN arrayMap(p -> (p.membership_id, p.time_played_seconds, p.completed), instance.players) AS p1
ARRAY JOIN arrayMap(p -> (p.membership_id, p.time_played_seconds, p.completed), instance.players) AS p2
WHERE membership_id != related_membership_id;

