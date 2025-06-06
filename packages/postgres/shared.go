package postgres

import (
	"database/sql"
	"raidhub/packages/pgcr_types"
)

func UpsertPlayer(tx *sql.Tx, player *pgcr_types.Player) (sql.Result, error) {
	return tx.Exec(`
			INSERT INTO player (
				"membership_id",
				"membership_type",
				"icon_path",
				"display_name",
				"bungie_global_display_name",
				"bungie_global_display_name_code",
				"last_seen"
			)
			VALUES (
				$1, $2, $3, $4, $5, $6, $7
			)
			ON CONFLICT (membership_id)
			DO UPDATE SET
				membership_type = COALESCE(player.membership_type, EXCLUDED.membership_type),
				icon_path = CASE 
					WHEN EXCLUDED.last_seen > player.last_seen THEN COALESCE(EXCLUDED.icon_path, player.icon_path)
					ELSE player.icon_path
				END,
				display_name = CASE 
					WHEN EXCLUDED.last_seen > player.last_seen THEN COALESCE(EXCLUDED.display_name, player.display_name)
					ELSE player.display_name
				END,
				bungie_global_display_name = CASE 
					WHEN EXCLUDED.last_seen > player.last_seen THEN COALESCE(EXCLUDED.bungie_global_display_name, player.bungie_global_display_name)
					ELSE player.bungie_global_display_name
				END,
				bungie_global_display_name_code = CASE 
					WHEN EXCLUDED.last_seen > player.last_seen THEN COALESCE(EXCLUDED.bungie_global_display_name_code, player.bungie_global_display_name_code)
					ELSE player.bungie_global_display_name_code
				END,
				last_seen = GREATEST(player.last_seen, EXCLUDED.last_seen),
				first_seen = LEAST(player.first_seen, EXCLUDED.first_seen)
				;
			`,
		player.MembershipId, player.MembershipType, player.IconPath, player.DisplayName,
		player.BungieGlobalDisplayName, player.BungieGlobalDisplayNameCode, player.LastSeen)
}
