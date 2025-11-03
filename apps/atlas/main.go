package main

import (
	"flag"

	"raidhub/lib/database/postgres"
	"raidhub/lib/messaging/publishing"
	"raidhub/lib/monitoring"
	"raidhub/lib/services/instance"
	"raidhub/lib/utils/logging"
)

var AtlasLogger = logging.NewLogger("atlas")

func main() {
	flag.Parse()

	flushSentry, recoverSentry := AtlasLogger.InitSentry()
	defer flushSentry()
	defer recoverSentry()

	config := parseConfig()
	workers = config.Workers

	monitoring.RegisterAtlasMetrics()

	AtlasLogger.Debug("WAITING_ON_CONNECTIONS", map[string]any{
		"services": []string{"postgres", "publishing"},
	})
	postgres.Wait()
	publishing.Wait()

	var instanceId int64
	var err error
	if config.TargetInstanceId == -1 {
		if instanceId, err = instance.GetLatestInstanceId(config.Buffer); err != nil {
			// In dev mode, if database is empty, try to find latest from web using binary search
			if config.DevMode {
				AtlasLogger.Info("DATABASE_EMPTY_IN_DEV_MODE", map[string]any{
					"attempting": "binary_search_from_web",
				})
				if instanceId, err = instance.GetLatestInstanceIdFromWeb(config.Buffer); err != nil {
					AtlasLogger.Fatal("FAILED_TO_GET_LATEST_INSTANCE_ID", err, map[string]any{
						"buffer": config.Buffer,
						"source": "database_and_web",
					})
				}
			} else {
				AtlasLogger.Fatal("FAILED_TO_GET_LATEST_INSTANCE_ID", err, map[string]any{
					"buffer": config.Buffer,
				})
			}
		}
	} else {
		instanceId = config.TargetInstanceId - config.Buffer
	}

	run(instanceId, config.DevSkip, config.MaxWorkers)
}
