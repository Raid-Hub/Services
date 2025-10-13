SELECT update_wfr_scores();

CREATE OR REPLACE FUNCTION update_wfr_scores()
RETURNS INTEGER AS $$
DECLARE
    updated_count INTEGER;
BEGIN
    -- Lock the player table to avoid deadlocks during concurrent updates
    LOCK TABLE player IN SHARE ROW EXCLUSIVE MODE;

    -- Reset scores
    UPDATE player SET wfr_score = 0 WHERE wfr_score <> 0;

    -- Compute new scores
    WITH unnested_entries AS (
        SELECT
            world_first_contest_leaderboard.*,
            jsonb_array_elements(membership_ids)::bigint AS membership_id
        FROM world_first_contest_leaderboard
    ), pscores AS (
        SELECT DISTINCT ON (membership_id, activity_id)
            membership_id,
            ((1 / SQRT(rank)) * POWER(1.25, activity_id - 1)) AS score
        FROM unnested_entries
        ORDER BY membership_id, activity_id, rank ASC
    ), total_scores AS (
        SELECT
            membership_id,
            SUM(score) AS wfr_score
        FROM pscores
        GROUP BY membership_id
    )
    -- Apply new scores
    UPDATE player
    SET wfr_score = total_scores.wfr_score
    FROM total_scores
    WHERE player.membership_id = total_scores.membership_id;

    -- Return number of affected players
    GET DIAGNOSTICS updated_count = ROW_COUNT;
    RETURN updated_count;
END;
$$ LANGUAGE plpgsql;

