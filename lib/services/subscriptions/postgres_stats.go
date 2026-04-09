package subscriptions

import (
	"context"

	"raidhub/lib/database/postgres"
)

// InstancePlayerStats aggregates character-level combat stats for one membership in an instance.
type InstancePlayerStats struct {
	Kills             int
	Deaths            int
	Assists           int
	TimePlayedSeconds int
	FirstClear        bool
}

// loadInstancePlayerStats loads per-membership aggregates from extended.instance_character,
// joined with core.instance_player for first-clear flags.
func loadInstancePlayerStats(ctx context.Context, instanceID int64) (map[int64]InstancePlayerStats, error) {
	rows, err := postgres.DB.QueryContext(ctx, `
		SELECT
			ip.membership_id,
			ip.is_first_clear,
			COALESCE(SUM(ic.kills), 0)::bigint,
			COALESCE(SUM(ic.deaths), 0)::bigint,
			COALESCE(SUM(ic.assists), 0)::bigint,
			COALESCE(SUM(ic.time_played_seconds), 0)::bigint
		FROM core.instance_player ip
		LEFT JOIN extended.instance_character ic
			ON ic.instance_id = ip.instance_id AND ic.membership_id = ip.membership_id
		WHERE ip.instance_id = $1
		GROUP BY ip.membership_id, ip.is_first_clear
	`, instanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[int64]InstancePlayerStats)
	for rows.Next() {
		var mid int64
		var fc bool
		var k, d, a, t int64
		if err := rows.Scan(&mid, &fc, &k, &d, &a, &t); err != nil {
			return nil, err
		}
		out[mid] = InstancePlayerStats{
			Kills:             int(k),
			Deaths:            int(d),
			Assists:           int(a),
			TimePlayedSeconds: int(t),
			FirstClear:        fc,
		}
	}
	return out, rows.Err()
}
