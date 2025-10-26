package stats

import (
	"database/sql"
)

func UpdatePlayerSumOfBest(
	membershipId int64, tx *sql.Tx,
) (sql.Result, error) {
	return tx.Exec(`WITH c AS (SELECT COUNT(*) as expected FROM activity_definition WHERE is_raid = true AND is_sunset = false)
		UPDATE player p
		SET sum_of_best = ptd.total_duration
		FROM (
			SELECT
				ps.membership_id,
				SUM(a.duration) AS total_duration
			FROM player_stats ps
			JOIN activity_definition r ON ps.activity_id = r.id
			LEFT JOIN instance a ON ps.fastest_instance_id = a.instance_id
			WHERE a.duration IS NOT NULL AND is_raid = true AND is_sunset = false 
				AND ps.membership_id = $1
			GROUP BY ps.membership_id
			HAVING COUNT(a.instance_id) = (SELECT expected FROM c)
		) ptd
		WHERE p.membership_id = ptd.membership_id;`, membershipId)

}
