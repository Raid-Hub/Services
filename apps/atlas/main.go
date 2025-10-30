package main

import (
	"flag"

	"raidhub/lib/database/postgres"
	"raidhub/lib/messaging/publishing"
	"raidhub/lib/monitoring"
	"raidhub/lib/services/instance"
	"raidhub/lib/utils/logging"
)

func main() {
	flag.Parse()

	config := parseConfig()
	workers = config.Workers

	monitoring.RegisterAtlasMetrics()

	logger.Debug(logging.WAITING_ON_CONNECTIONS, map[string]any{
		"services": []string{"postgres", "publishing"},
	})
	postgres.Wait()
	publishing.Wait()

	// postgres.DB is initialized in init()
	var instanceId int64
	var err error
	if config.TargetInstanceId == -1 {
		if instanceId, err = instance.GetLatestInstanceId(config.Buffer); err != nil {
			logger.Fatal("FAILED_TO_GET_LATEST_INSTANCE_ID", map[string]any{
				logging.ERROR: err.Error(),
				"buffer":      config.Buffer,
			})
		}
	} else {
		instanceId = config.TargetInstanceId - config.Buffer
	}

	run(instanceId, config.DevSkip, config.MaxWorkers)
}
