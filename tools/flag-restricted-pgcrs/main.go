package main

import (
	"context"
	"raidhub/lib/services/cheat_detection"
	"raidhub/lib/services/instance"
	"raidhub/lib/services/pgcr_processing"
	"raidhub/lib/utils/logging"
)

const cheatCheckVersion = ""

var logger = logging.NewLogger("FLAG_RESTRICTED_TOOL")

func main() {
	flushSentry, recoverSentry := logger.InitSentry()
	defer flushSentry()
	defer recoverSentry()

	instanceIds, err := instance.GetInstanceIdsByHash(3896382790)
	if err != nil {
		logger.Error("ERROR_QUERYING_INSTANCE_TABLE", err, map[string]any{})
		return
	}

	total := 0
	badApples := 0

	for _, instanceId := range instanceIds {

		result, _, _ := pgcr_processing.FetchAndProcessPGCR(context.Background(), instanceId, 0)
		total++

		switch result {
		case pgcr_processing.InsufficientPrivileges:
			logger.Info("INSTANCE_RESTRICTED", map[string]any{"instance_id": instanceId})
		case pgcr_processing.Success:
			logger.Info("INSTANCE_NOT_RESTRICTED", map[string]any{"instance_id": instanceId})
		default:
			logger.Info("INSTANCE_UNEXPECTED_RESULT", map[string]any{"instance_id": instanceId, "result": result})
			result, _, _ = pgcr_processing.FetchAndProcessPGCR(context.Background(), instanceId, 0)
		}

		if result == pgcr_processing.InsufficientPrivileges {
			badApples++
			err = cheat_detection.FlagInstanceManually(instanceId, cheatCheckVersion, cheat_detection.RestrictedPGCR|cheat_detection.DesertPerpetual, 1.0)
			if err != nil {
				logger.Warn("ERROR_FLAGGING_INSTANCE", err, map[string]any{"instance_id": instanceId})
			} else {
				logger.Info("INSTANCE_FLAGGED_AS_RESTRICTED", map[string]any{"instance_id": instanceId})
			}
			err = cheat_detection.BlacklistInstanceManually(instanceId, cheatCheckVersion, "Restricted PGCR")
			if err != nil {
				logger.Warn("ERROR_BLACKLISTING_INSTANCE", err, map[string]any{"instance_id": instanceId})
			} else {
				logger.Info("INSTANCE_BLACKLISTED_AS_RESTRICTED_PGCR", map[string]any{"instance_id": instanceId})
			}
		}
	}

	logger.Info("COMPLETED", map[string]any{"total_checked": total, "restricted_flagged": badApples})
}
