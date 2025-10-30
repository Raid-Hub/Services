package flagrestricted

import (
	"raidhub/lib/database/postgres"
	"raidhub/lib/services/cheat_detection"
	"raidhub/lib/services/pgcr_processing"
	"raidhub/lib/utils"
)

const cheatCheckVersion = ""

var flagRestrictedLogger = utils.NewLogger("FLAG_RESTRICTED_TOOL")

func FlagRestrictedPGCRs() {
	rows, err := postgres.DB.Query(`SELECT instance_id FROM instance WHERE hash = $1 and completed`, 3896382790)
	if err != nil {
		flagRestrictedLogger.ErrorF("Error querying instance table:", err)
	}
	defer rows.Close()

	stmnt, err := postgres.DB.Prepare(
		`INSERT INTO flag_instance (instance_id, cheat_check_version, cheat_check_bitmask, flagged_at, cheat_probability)
		VALUES ($1, $2, $3, NOW(), $4)
		ON CONFLICT DO NOTHING`,
	)
	if err != nil {
		flagRestrictedLogger.ErrorF("Error preparing insert statement:", err)
	}
	defer stmnt.Close()

	stmnt2, err := postgres.DB.Prepare(`INSERT INTO blacklist_instance (instance_id, report_source, cheat_check_version, reason)
		VALUES ($1, 'Manual', $2, $3)
        ON CONFLICT (instance_id)
		DO UPDATE SET report_source = 'Manual', cheat_check_version = $2, reason = $3, created_at = NOW()`)
	if err != nil {
		flagRestrictedLogger.ErrorF("Error preparing blacklist insert statement:", err)
	}
	defer stmnt2.Close()

	total := 0
	badApples := 0

	for rows.Next() {
		var instanceId int64
		if err := rows.Scan(&instanceId); err != nil {
			flagRestrictedLogger.ErrorFln("Error scanning instance_id:", err)
		}

		result, _, _, _ := pgcr_processing.FetchAndProcessPGCR(instanceId)
		total++

		switch result {
		case pgcr_processing.InsufficientPrivileges:
			flagRestrictedLogger.InfoF("Instance %d is restricted", instanceId)
		case pgcr_processing.Success:
			flagRestrictedLogger.InfoF("Instance %d is not restricted", instanceId)
		default:
			flagRestrictedLogger.InfoF("Instance %d returned unexpected result: %d", instanceId, result)
			result, _, _, _ = pgcr_processing.FetchAndProcessPGCR(instanceId)
		}

		if result == pgcr_processing.InsufficientPrivileges {
			badApples++
			_, err = stmnt.Exec(instanceId, cheatCheckVersion, cheat_detection.RestrictedPGCR|cheat_detection.DesertPerpetual, 1.0)
			if err != nil {
				flagRestrictedLogger.InfoF("Error flagging instance %d: %s", instanceId, err)
			} else {
				flagRestrictedLogger.InfoF("Flagged instance %d as restricted", instanceId)
			}
			_, err = stmnt2.Exec(instanceId, cheatCheckVersion, "Restricted PGCR")
			if err != nil {
				flagRestrictedLogger.InfoF("Error blacklisting instance %d: %s", instanceId, err)
			} else {
				flagRestrictedLogger.InfoF("Blacklisted instance %d as restricted PGCR", instanceId)
			}
		}
	}

	flagRestrictedLogger.InfoF("Total instances checked: %d, Restricted instances flagged: %d", total, badApples)
}
