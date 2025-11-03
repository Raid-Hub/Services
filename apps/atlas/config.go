package main

import (
	"flag"

	"raidhub/lib/utils/logging"
)

var (
	numWorkers       = flag.Int("workers", 25, "number of workers to spawn at the start")
	buffer           = flag.Int64("buffer", -1, "number of ids to start behind last added (-1 means auto: 10000 in prod, 0 in dev)")
	targetInstanceId = flag.Int64("target", -1, "specific instance id to start at (optional)")
	maxWorkersFlag   = flag.Int("max-workers", 0, "maximum number of workers (0 uses default constant, or 8 in dev mode)")
	devFlag          = flag.Bool("dev", false, "enable dev mode (defaults: skip=3, max-workers=8, buffer=0)")
	devSkip          = flag.Int("dev-skip", 0, "skip N instances between each processed instance (requires --dev flag, defaults to 3)")
)

const (
	minWorkers = 5
	maxWorkers = 250

	retryDelayTime = 5500
)

func parseConfig() AtlasConfig {
	// Apply dev mode defaults
	effectiveDevSkip := 0
	effectiveMaxWorkers := maxWorkers
	effectiveBuffer := *buffer

	// Handle buffer default based on mode
	if *buffer == -1 {
		if *devFlag {
			effectiveBuffer = 100
		} else {
			effectiveBuffer = 10_000
		}
	}

	if *devFlag {
		// Default dev-skip to 2 if not explicitly set (every 3 ids)
		if *devSkip == 0 {
			effectiveDevSkip = 2
		} else {
			effectiveDevSkip = (*devSkip - 1)
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
			AtlasLogger.Fatal("INVALID_FLAG_COMBINATION", nil, map[string]any{
				logging.REASON: "dev-skip flag requires --dev flag to be set",
			})
		}
		// Use max workers flag if provided
		if *maxWorkersFlag > 0 {
			effectiveMaxWorkers = *maxWorkersFlag
		}
	}

	if *devSkip < 0 {
		AtlasLogger.Fatal("INVALID_FLAG_VALUE", nil, map[string]any{
			logging.REASON: "dev-skip must be >= 0",
			"dev_skip":     *devSkip,
		})
	}

	workersValue := *numWorkers
	if effectiveBuffer < 0 || workersValue <= 0 {
		AtlasLogger.Fatal("INVALID_FLAGS", nil, map[string]any{
			logging.REASON:       "invalid flag values",
			"buffer":             effectiveBuffer,
			logging.WORKER_COUNT: workersValue,
		})
	}

	// In dev mode, cap workers to max_workers if it exceeds it (unless explicitly set)
	if *devFlag && workersValue > effectiveMaxWorkers {
		workersValue = effectiveMaxWorkers
	} else if !*devFlag && workersValue > effectiveMaxWorkers {
		AtlasLogger.Fatal("INVALID_FLAGS", nil, map[string]any{
			logging.REASON:       "workers exceeds max-workers",
			"buffer":             effectiveBuffer,
			logging.WORKER_COUNT: *numWorkers,
			"max_workers":        effectiveMaxWorkers,
		})
	}

	config := AtlasConfig{
		Workers:          workersValue,
		Buffer:           effectiveBuffer,
		TargetInstanceId: *targetInstanceId,
		DevMode:          *devFlag,
		DevSkip:          effectiveDevSkip,
		MaxWorkers:       effectiveMaxWorkers,
	}

	AtlasLogger.Info("ATLAS_CONFIG_LOADED", map[string]any{
		logging.WORKER_COUNT: config.Workers,
		"buffer":             config.Buffer,
		"target_instance_id": config.TargetInstanceId,
		"dev_mode":           config.DevMode,
		"dev_skip":           config.DevSkip,
		"max_workers":        config.MaxWorkers,
	})

	return config
}
