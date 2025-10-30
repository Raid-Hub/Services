package main

import (
	"flag"

	"raidhub/lib/utils/logging"
)

var (
	numWorkers       = flag.Int("workers", 25, "number of workers to spawn at the start")
	buffer           = flag.Int64("buffer", 10_000, "number of ids to start behind last added")
	targetInstanceId = flag.Int64("target", -1, "specific instance id to start at (optional)")
	maxWorkersFlag   = flag.Int("max-workers", 0, "maximum number of workers (0 uses default constant, or 8 in dev mode)")
	devFlag          = flag.Bool("dev", false, "enable dev mode (defaults to skip=5, max-workers=8)")
	devSkip          = flag.Int("dev-skip", 0, "skip N instances between each processed instance (requires --dev flag, defaults to 5)")
)

func parseConfig() AtlasConfig {
	// Apply dev mode defaults
	effectiveDevSkip := 0
	effectiveMaxWorkers := maxWorkers

	if *devFlag {
		// Default dev-skip to 4 if not explicitly set (every 5 ids)
		if *devSkip == 0 {
			effectiveDevSkip = 4
		} else {
			effectiveDevSkip = *devSkip
		}
		// Default max-workers to 8 if not explicitly set
		if *maxWorkersFlag == 0 {
			effectiveMaxWorkers = 8
		} else {
			effectiveMaxWorkers = *maxWorkersFlag
		}
	} else {
		// Not in dev mode
		if *devSkip != 0 {
			logger.Fatal("INVALID_FLAG_COMBINATION", map[string]any{
				logging.REASON: "dev-skip flag requires --dev flag to be set",
			})
		}
		// Use max workers flag if provided
		if *maxWorkersFlag > 0 {
			effectiveMaxWorkers = *maxWorkersFlag
		}
	}

	if *devSkip < 0 {
		logger.Fatal("INVALID_FLAG_VALUE", map[string]any{
			logging.REASON: "dev-skip must be >= 0",
			"dev_skip":     *devSkip,
		})
	}

	workersValue := *numWorkers
	if *buffer < 0 || workersValue <= 0 {
		logger.Fatal("INVALID_FLAGS", map[string]any{
			logging.REASON:       "invalid flag values",
			"buffer":             *buffer,
			logging.WORKER_COUNT: workersValue,
		})
	}

	// In dev mode, cap workers to max_workers if it exceeds it (unless explicitly set)
	if *devFlag && workersValue > effectiveMaxWorkers {
		workersValue = effectiveMaxWorkers
	} else if !*devFlag && workersValue > effectiveMaxWorkers {
		logger.Fatal("INVALID_FLAGS", map[string]any{
			logging.REASON:       "workers exceeds max-workers",
			"buffer":             *buffer,
			logging.WORKER_COUNT: *numWorkers,
			"max_workers":        effectiveMaxWorkers,
		})
	}

	config := AtlasConfig{
		Workers:          workersValue,
		Buffer:           *buffer,
		TargetInstanceId: *targetInstanceId,
		DevMode:          *devFlag,
		DevSkip:          effectiveDevSkip,
		MaxWorkers:       effectiveMaxWorkers,
	}

	logger.Info("ATLAS_CONFIG_LOADED", map[string]any{
		logging.WORKER_COUNT: config.Workers,
		"buffer":             config.Buffer,
		"target_instance_id": config.TargetInstanceId,
		"dev_mode":           config.DevMode,
		"dev_skip":           config.DevSkip,
		"max_workers":        config.MaxWorkers,
	})

	return config
}
