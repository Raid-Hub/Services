package player

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"time"

	"raidhub/lib/database/postgres"

	"github.com/lib/pq"
)

const (
	privacyCacheTTL = 2 * time.Minute
	profileCacheTTL = 5 * time.Minute
)

type boolTTL struct {
	v   bool
	exp time.Time
}

type profilePartsTTL struct {
	name    string
	iconURL string
	exp     time.Time
}

var (
	privacyCacheMu sync.Mutex
	privacyCache   = make(map[int64]boolTTL)

	profilePartsCacheMu sync.Mutex
	profilePartsCache   = make(map[int64]profilePartsTTL)
)

// PrivateFlagsByMembershipIDs returns core.player.is_private for the given membership ids.
// Missing ids are omitted (caller treats as non-private only where present; subscriptions match uses participant list).
func PrivateFlagsByMembershipIDs(ctx context.Context, ids []int64) (map[int64]bool, error) {
	out := make(map[int64]bool)
	if len(ids) == 0 {
		return out, nil
	}
	now := time.Now()
	var needDB []int64
	privacyCacheMu.Lock()
	for _, id := range ids {
		e, ok := privacyCache[id]
		if ok && now.Before(e.exp) {
			out[id] = e.v
			continue
		}
		needDB = append(needDB, id)
	}
	privacyCacheMu.Unlock()
	if len(needDB) == 0 {
		return out, nil
	}

	rows, err := postgres.DB.QueryContext(ctx,
		`SELECT membership_id, is_private FROM core.player WHERE membership_id = ANY($1)`,
		pq.Array(needDB))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	exp := now.Add(privacyCacheTTL)
	privacyCacheMu.Lock()
	defer privacyCacheMu.Unlock()
	found := make(map[int64]struct{}, len(needDB))
	for rows.Next() {
		var mid int64
		var priv bool
		if err := rows.Scan(&mid, &priv); err != nil {
			return nil, err
		}
		out[mid] = priv
		privacyCache[mid] = boolTTL{v: priv, exp: exp}
		found[mid] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, id := range needDB {
		if _, ok := found[id]; !ok {
			privacyCache[id] = boolTTL{v: false, exp: exp}
		}
	}
	return out, nil
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
	ClassHash    uint32
	Finished     bool
	Kills        int
	Deaths       int
	Assists      int
}

// PlayerProfilesForDelivery returns one row per id in the same order as ids (omitted ids get empty name).
func PlayerProfilesForDelivery(ctx context.Context, ids []int64) ([]PlayerProfileForDelivery, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	type row struct {
		name    string
		iconURL string
	}
	now := time.Now()
	byID := make(map[int64]row, len(ids))
	var needDB []int64

	profilePartsCacheMu.Lock()
	for _, id := range ids {
		e, ok := profilePartsCache[id]
		if ok && now.Before(e.exp) {
			byID[id] = row{name: e.name, iconURL: e.iconURL}
			continue
		}
		needDB = append(needDB, id)
	}
	profilePartsCacheMu.Unlock()

	if len(needDB) > 0 {
		rows, err := postgres.DB.QueryContext(ctx, `
		SELECT membership_id,
		       COALESCE(NULLIF(TRIM(bungie_name), ''), NULLIF(TRIM(display_name), ''), ''),
		       icon_path
		FROM core.player WHERE membership_id = ANY($1)`,
			pq.Array(needDB))
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		fromDB := make(map[int64]row, len(needDB))
		for rows.Next() {
			var mid int64
			var name string
			var iconPath sql.NullString
			if err := rows.Scan(&mid, &name, &iconPath); err != nil {
				return nil, err
			}
			fromDB[mid] = row{name: name, iconURL: bungieIconURL(iconPath)}
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}

		exp := now.Add(profileCacheTTL)
		profilePartsCacheMu.Lock()
		for _, id := range needDB {
			if r, ok := fromDB[id]; ok {
				byID[id] = r
				profilePartsCache[id] = profilePartsTTL{name: r.name, iconURL: r.iconURL, exp: exp}
			} else {
				profilePartsCache[id] = profilePartsTTL{exp: exp}
			}
		}
		profilePartsCacheMu.Unlock()
	}

	out := make([]PlayerProfileForDelivery, 0, len(ids))
	for _, id := range ids {
		r, ok := byID[id]
		if !ok {
			out = append(out, PlayerProfileForDelivery{MembershipID: id})
			continue
		}
		out = append(out, PlayerProfileForDelivery{
			MembershipID: id,
			DisplayName:  r.name,
			IconURL:      r.iconURL,
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
