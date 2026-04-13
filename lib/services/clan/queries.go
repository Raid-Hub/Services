package clan

import (
	"context"

	"raidhub/lib/database/postgres"

	"github.com/lib/pq"
)

// GroupIDsByMembershipIDs returns clan group_ids per membership_id from clan.clan_members.
func GroupIDsByMembershipIDs(ctx context.Context, ids []int64) (map[int64][]int64, error) {
	out := make(map[int64][]int64)
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := postgres.DB.QueryContext(ctx,
		`SELECT membership_id, group_id FROM clan.clan_members WHERE membership_id = ANY($1)`,
		pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var mid, gid int64
		if err := rows.Scan(&mid, &gid); err != nil {
			return nil, err
		}
		out[mid] = append(out[mid], gid)
	}
	return out, rows.Err()
}

// NamesByGroupIDs returns clan display names for the given Bungie group ids.
func NamesByGroupIDs(ctx context.Context, ids []int64) (map[int64]string, error) {
	out := make(map[int64]string)
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := postgres.DB.QueryContext(ctx,
		`SELECT group_id, name FROM clan.clan WHERE group_id = ANY($1)`,
		pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var gid int64
		var name string
		if err := rows.Scan(&gid, &name); err != nil {
			return nil, err
		}
		out[gid] = name
	}
	return out, rows.Err()
}
