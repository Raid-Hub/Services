CREATE MATERIALIZED VIEW "clan_leaderboard" AS
WITH ranked_players AS (
    SELECT
        cm."group_id",
        p."membership_id",
        p."clears",
        p."fresh_clears",
        p."sherpas",
        p."total_time_played_seconds",
        p."wfr_score",
        ROW_NUMBER() OVER (PARTITION BY cm."group_id" ORDER BY p."wfr_score" DESC) AS rn
    FROM clan_members cm
    JOIN player p USING ("membership_id")
    JOIN clan USING ("group_id")
)
SELECT 
    rp."group_id",
    COUNT(rp."membership_id") AS "known_member_count",
    SUM(rp."clears") AS "clears",
    ROUND(AVG(rp."clears")) AS "average_clears",
    SUM(rp."fresh_clears") AS "fresh_clears",
    ROUND(AVG(rp."fresh_clears")) AS "average_fresh_clears",
    SUM(rp."sherpas") AS "sherpas",
    ROUND(AVG(rp."sherpas")) AS "average_sherpas",
    SUM(rp."total_time_played_seconds") AS "time_played_seconds",
    ROUND(AVG(rp."total_time_played_seconds")) AS "average_time_played_seconds",
    SUM(rp."wfr_score") AS "total_contest_score",
    3 * SUM(rp."wfr_score" * POWER(0.9, rp.rn - 6))::DOUBLE PRECISION / (POWER(1 + COUNT(rp."membership_id"), (1.0 / 3))) AS "weighted_contest_score"
FROM ranked_players rp
GROUP BY rp."group_id";

CREATE UNIQUE INDEX idx_clan_leaderboard_group_id ON clan_leaderboard (group_id);

ALTER MATERIALIZED VIEW "clan_leaderboard" OWNER TO raidhub_user;