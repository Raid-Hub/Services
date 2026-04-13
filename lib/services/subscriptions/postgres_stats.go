package subscriptions

import (
	"context"
	"database/sql"
	"sort"
	"strings"

	"raidhub/lib/database/postgres"
	"raidhub/lib/messaging/messages"
)

// InstancePlayerStats aggregates character-level combat stats for one membership in an instance.
type InstancePlayerStats struct {
	Kills             int
	Deaths            int
	Assists           int
	TimePlayedSeconds int
}

// loadInstancePlayerStats loads per-membership aggregates from extended.instance_character.
func loadInstancePlayerStats(ctx context.Context, instanceID int64) (map[int64]InstancePlayerStats, error) {
	rows, err := postgres.DB.QueryContext(ctx, `
		SELECT
			ip.membership_id,
			COALESCE(SUM(ic.kills), 0)::bigint,
			COALESCE(SUM(ic.deaths), 0)::bigint,
			COALESCE(SUM(ic.assists), 0)::bigint,
			COALESCE(SUM(ic.time_played_seconds), 0)::bigint
		FROM core.instance_player ip
		LEFT JOIN extended.instance_character ic
			ON ic.instance_id = ip.instance_id AND ic.membership_id = ip.membership_id
		WHERE ip.instance_id = $1
		GROUP BY ip.membership_id
	`, instanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[int64]InstancePlayerStats)
	for rows.Next() {
		var mid int64
		var k, d, a, t int64
		if err := rows.Scan(&mid, &k, &d, &a, &t); err != nil {
			return nil, err
		}
		out[mid] = InstancePlayerStats{
			Kills:             int(k),
			Deaths:            int(d),
			Assists:           int(a),
			TimePlayedSeconds: int(t),
		}
	}
	return out, rows.Err()
}

// loadInstancePlayerClasses picks one class_hash per membership from extended.instance_character.
func loadInstancePlayerClasses(ctx context.Context, instanceID int64) (map[int64]uint32, error) {
	rows, err := postgres.DB.QueryContext(ctx, `
		SELECT DISTINCT ON (membership_id)
			membership_id,
			class_hash
		FROM extended.instance_character
		WHERE instance_id = $1
			AND class_hash IS NOT NULL
			AND class_hash <> 0
		ORDER BY membership_id, completed DESC, time_played_seconds DESC, kills DESC, character_id
	`, instanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[int64]uint32)
	for rows.Next() {
		var mid int64
		var classHash int64
		if err := rows.Scan(&mid, &classHash); err != nil {
			return nil, err
		}
		out[mid] = uint32(classHash)
	}
	return out, rows.Err()
}

// loadInstanceFeatsForDiscord resolves core.instance.skull_hashes to manifest feats only when a definition row exists.
// PGCR selectedSkullHashes match activity_feat_definition.skull_hash (skullIdentifierHash). Omits unknown skulls and rows
// without a usable label or icon_path.
func loadInstanceFeatsForDiscord(ctx context.Context, instanceID int64) ([]messages.DiscordFeat, error) {
	rows, err := postgres.DB.QueryContext(ctx, `
		SELECT DISTINCT ON (u.skull_hash)
			fd.name_short,
			fd.name,
			fd.icon_path
		FROM core.instance i
		CROSS JOIN LATERAL unnest(COALESCE(i.skull_hashes, '{}')) AS u(skull_hash)
		INNER JOIN definitions.activity_feat_definition fd ON fd.skull_hash = u.skull_hash
		WHERE i.instance_id = $1
		ORDER BY u.skull_hash`,
		instanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []messages.DiscordFeat
	for rows.Next() {
		var nameShort, name sql.NullString
		var iconPath string
		if err := rows.Scan(&nameShort, &name, &iconPath); err != nil {
			return nil, err
		}
		label := featDefinitionLabel(nameShort, name)
		if label == "" {
			continue
		}
		iconURL := bungieContentURL(iconPath)
		if iconURL == "" {
			continue
		}
		out = append(out, messages.DiscordFeat{Label: label, IconURL: iconURL})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Label) < strings.ToLower(out[j].Label)
	})
	return out, nil
}

func featDefinitionLabel(nameShort, name sql.NullString) string {
	if nameShort.Valid && strings.TrimSpace(nameShort.String) != "" {
		return strings.TrimSpace(nameShort.String)
	}
	if name.Valid && strings.TrimSpace(name.String) != "" {
		return strings.TrimSpace(name.String)
	}
	return ""
}

func bungieContentURL(iconPath string) string {
	p := strings.TrimSpace(iconPath)
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return "https://www.bungie.net" + p
}
