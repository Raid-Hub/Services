CREATE TABLE player_relation_weights_bidirectional
(
    membership_id UInt64,
    related_membership_id UInt64,
    weight UInt64
) ENGINE = MergeTree()

CREATE MATERIALIZED VIEW player_relation_weights_mv TO player_relation_weights_bidirectional AS
SELECT 
     tupleElement(p1, 1) AS membership_id,
     tupleElement(p2, 1) AS related_membership_id,
     LEAST(tupleElement(p1, 2), tupleElement(p2, 2)) * (tupleElement(p1, 3) + 1) * (tupleElement(p2, 3) + 1) AS weight
FROM instance
ARRAY JOIN arrayMap(p -> (p.membership_id, p.time_played_seconds, p.completed), instance.players) AS p1
ARRAY JOIN arrayMap(p -> (p.membership_id, p.time_played_seconds, p.completed), instance.players) AS p2
WHERE membership_id <> related_membership_id;

