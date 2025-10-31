package main

import (
	"raidhub/lib/database/postgres"
	"raidhub/lib/services/cheat_detection"
	"raidhub/lib/services/pgcr_processing"
	"raidhub/lib/utils/logging"
)

const cheatCheckVersion = ""

var logger = logging.NewLogger("FLAG_RESTRICTED_TOOL")

func FlagRestrictedPGCRs() {
	rows, err := postgres.DB.Query(`SELECT instance_id FROM instance WHERE hash = $1 and completed`, 3896382790)
	if err != nil {
		logger.Error("ERROR_QUERYING_INSTANCE_TABLE", map[string]any{logging.ERROR: err.Error()})
	}
	defer rows.Close()

	stmnt, err := postgres.DB.Prepare(
		`INSERT INTO flag_instance (instance_id, cheat_check_version, cheat_check_bitmask, flagged_at, cheat_probability)
		VALUES ($1, $2, $3, NOW(), $4)
		ON CONFLICT DO NOTHING`,
	)
	if err != nil {
		logger.Error("ERROR_PREPARING_INSERT_STATEMENT", map[string]any{logging.ERROR: err.Error()})
	}
	defer stmnt.Close()

	stmnt2, err := postgres.DB.Prepare(`INSERT INTO blacklist_instance (instance_id, report_source, cheat_check_version, reason)
		VALUES ($1, 'Manual', $2, $3)
        ON CONFLICT (instance_id)
		DO UPDATE SET report_source = 'Manual', cheat_check_version = $2, reason = $3, created_at = NOW()`)
	if err != nil {
		logger.Error("ERROR_PREPARING_BLACKLIST_INSERT_STATEMENT", map[string]any{logging.ERROR: err.Error()})
	}
	defer stmnt2.Close()

	total := 0
	badApples := 0

	for rows.Next() {
		var instanceId int64
		if err := rows.Scan(&instanceId); err != nil {
			logger.Error("ERROR_SCANNING_INSTANCE_ID", map[string]any{logging.ERROR: err.Error()})
		}

		result, _, _, _ := pgcr_processing.FetchAndProcessPGCR(instanceId)
		total++

		switch result {
		case pgcr_processing.InsufficientPrivileges:
			logger.Info("INSTANCE_RESTRICTED", map[string]any{"instance_id": instanceId})
		case pgcr_processing.Success:
			logger.Info("INSTANCE_NOT_RESTRICTED", map[string]any{"instance_id": instanceId})
		default:
			logger.Info("INSTANCE_UNEXPECTED_RESULT", map[string]any{"instance_id": instanceId, "result": result})
			result, _, _, _ = pgcr_processing.FetchAndProcessPGCR(instanceId)
		}

		if result == pgcr_processing.InsufficientPrivileges {
			badApples++
			_, err = stmnt.Exec(instanceId, cheatCheckVersion, cheat_detection.RestrictedPGCR|cheat_detection.DesertPerpetual, 1.0)
			if err != nil {
				logger.Warn("ERROR_FLAGGING_INSTANCE", map[string]any{"instance_id": instanceId, logging.ERROR: err.Error()})
			} else {
				logger.Info("INSTANCE_FLAGGED_AS_RESTRICTED", map[string]any{"instance_id": instanceId})
			}
			_, err = stmnt2.Exec(instanceId, cheatCheckVersion, "Restricted PGCR")
			if err != nil {
				logger.Warn("ERROR_BLACKLISTING_INSTANCE", map[string]any{"instance_id": instanceId, logging.ERROR: err.Error()})
			} else {
				logger.Info("INSTANCE_BLACKLISTED_AS_RESTRICTED_PGCR", map[string]any{"instance_id": instanceId})
			}
		}
	}

	logger.Info("COMPLETED", map[string]any{"total_checked": total, "restricted_flagged": badApples})
}

func main() {
	FlagRestrictedPGCRs()
}
