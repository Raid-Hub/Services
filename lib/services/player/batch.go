package player

import (
	"context"
	"database/sql"
	"strings"

	"raidhub/lib/database/postgres"

	"github.com/lib/pq"
)

// PrivateFlagsByMembershipIDs returns core.player.is_private for the given membership ids.
// Missing ids are omitted (caller treats as non-private only where present; subscriptions match uses participant list).
func PrivateFlagsByMembershipIDs(ctx context.Context, ids []int64) (map[int64]bool, error) {
	out := make(map[int64]bool)
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := postgres.DB.QueryContext(ctx,
		`SELECT membership_id, is_private FROM core.player WHERE membership_id = ANY($1)`,
		pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var mid int64
		var priv bool
		if err := rows.Scan(&mid, &priv); err != nil {
			return nil, err
		}
		out[mid] = priv
	}
	return out, rows.Err()
}

// DisplayNamesByMembershipIDs returns a display label per id (bungie name fallback) for webhook copy.
func DisplayNamesByMembershipIDs(ctx context.Context, ids []int64) (map[int64]string, error) {
	out := make(map[int64]string)
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := postgres.DB.QueryContext(ctx, `
		SELECT membership_id,
		       COALESCE(NULLIF(TRIM(bungie_name), ''), NULLIF(TRIM(display_name), ''), '')
		FROM core.player WHERE membership_id = ANY($1)`,
		pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var mid int64
		var name string
		if err := rows.Scan(&mid, &name); err != nil {
			return nil, err
		}
		out[mid] = name
	}
	return out, rows.Err()
}

// PlayerProfileForDelivery is display data for a single player in outbound alerts (e.g. Discord).
type PlayerProfileForDelivery struct {
	MembershipID int64
	DisplayName  string
	IconURL      string // empty if unknown; otherwise https://www.bungie.net…
}

// PlayerProfilesForDelivery returns one row per id in the same order as ids (omitted ids get empty name).
func PlayerProfilesForDelivery(ctx context.Context, ids []int64) ([]PlayerProfileForDelivery, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := postgres.DB.QueryContext(ctx, `
		SELECT membership_id,
		       COALESCE(NULLIF(TRIM(bungie_name), ''), NULLIF(TRIM(display_name), ''), ''),
		       icon_path
		FROM core.player WHERE membership_id = ANY($1)`,
		pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byID := make(map[int64]struct {
		name    string
		iconURL string
	})
	for rows.Next() {
		var mid int64
		var name string
		var iconPath sql.NullString
		if err := rows.Scan(&mid, &name, &iconPath); err != nil {
			return nil, err
		}
		byID[mid] = struct {
			name    string
			iconURL string
		}{name: name, iconURL: bungieIconURL(iconPath)}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]PlayerProfileForDelivery, 0, len(ids))
	for _, id := range ids {
		row, ok := byID[id]
		if !ok {
			out = append(out, PlayerProfileForDelivery{MembershipID: id})
			continue
		}
		out = append(out, PlayerProfileForDelivery{
			MembershipID: id,
			DisplayName:  row.name,
			IconURL:      row.iconURL,
		})
	}
	return out, nil
}

func bungieIconURL(iconPath sql.NullString) string {
	if !iconPath.Valid {
		return ""
	}
	p := strings.TrimSpace(iconPath.String)
	if p == "" {
		return ""
	}
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		return p
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return "https://www.bungie.net" + p
}
